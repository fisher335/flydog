package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	defaultSharePort = 2427
	maxShareUpload   = 4 << 30
)

// ShareInfo describes the current read/write share of a local folder.
type ShareInfo struct {
	Running     bool     `json:"running"`
	Dir         string   `json:"dir"`
	Port        int      `json:"port"`
	AllowUpload bool     `json:"allowUpload"`
	URLs        []string `json:"urls"`
}

// shareServer serves one folder over HTTP. Access requires the random token,
// and os.Root keeps every request inside the shared folder (including symlinks
// and ".." attempts).
type shareServer struct {
	rootDir     string
	root        *os.Root
	port        int
	token       string
	allowUpload bool
	urls        []string
	srv         *http.Server
}

func (s *shareServer) info() ShareInfo {
	return ShareInfo{
		Running:     true,
		Dir:         s.rootDir,
		Port:        s.port,
		AllowUpload: s.allowUpload,
		URLs:        append([]string(nil), s.urls...),
	}
}

func (s *shareServer) close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
	_ = s.root.Close()
}

// GetShareInfo returns the current share state.
func (a *App) GetShareInfo() ShareInfo {
	a.shareMu.Lock()
	defer a.shareMu.Unlock()
	if a.share == nil {
		return ShareInfo{}
	}
	return a.share.info()
}

// StartShare serves dir over HTTP. When allowUpload is true, visitors can also
// upload files into the folder.
func (a *App) StartShare(dir string, port int, allowUpload bool) (ShareInfo, error) {
	a.shareMu.Lock()
	defer a.shareMu.Unlock()
	if a.share != nil {
		return a.share.info(), nil
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ShareInfo{}, errors.New("请先选择要共享的文件夹")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ShareInfo{}, err
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return ShareInfo{}, errors.New("共享文件夹不存在或不是文件夹")
	}
	if port <= 0 {
		port = defaultSharePort
	}
	if port > 65535 {
		return ShareInfo{}, errors.New("端口必须在 1-65535 之间")
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return ShareInfo{}, fmt.Errorf("无法打开共享文件夹: %v", err)
	}
	token, err := newShareToken()
	if err != nil {
		_ = root.Close()
		return ShareInfo{}, err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		_ = root.Close()
		return ShareInfo{}, fmt.Errorf("端口 %d 被占用: %v", port, err)
	}
	s := &shareServer{
		rootDir:     abs,
		root:        root,
		port:        port,
		token:       token,
		allowUpload: allowUpload,
	}
	s.urls = shareURLs(port, token)
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[share] 服务停止: %v", err)
		}
	}()
	a.share = s
	log.Printf("[share] 共享已启动 %s (port %d, upload=%v)", abs, port, allowUpload)
	return s.info(), nil
}

// StopShare stops the HTTP server.
func (a *App) StopShare() ShareInfo {
	a.shareMu.Lock()
	defer a.shareMu.Unlock()
	if a.share != nil {
		dir := a.share.rootDir
		a.share.close()
		a.share = nil
		log.Printf("[share] 共享已停止 %s", dir)
	}
	return ShareInfo{}
}

// OpenURL opens a link in the system browser.
func (a *App) OpenURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return errors.New("无效的链接")
	}
	app := application.Get()
	if app == nil || app.Browser == nil {
		return errors.New("当前环境无法打开浏览器")
	}
	return app.Browser.OpenURL(raw)
}

// CopyText writes text to the system clipboard.
func (a *App) CopyText(text string) error {
	app := application.Get()
	if app == nil || app.Clipboard == nil {
		return errors.New("当前环境不支持剪贴板")
	}
	if !app.Clipboard.SetText(text) {
		return errors.New("复制失败")
	}
	return nil
}

func newShareToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func shareURLs(port int, token string) []string {
	var out []string
	seen := make(map[string]bool)
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLinkLocalUnicast() {
				continue
			}
			host := ip4.String()
			if seen[host] {
				continue
			}
			seen[host] = true
			out = append(out, fmt.Sprintf("http://%s:%d/s/%s/", host, port, token))
		}
	}
	if len(out) == 0 {
		out = append(out, fmt.Sprintf("http://127.0.0.1:%d/s/%s/", port, token))
	}
	sort.Slice(out, func(i, j int) bool { return hostOf(out[i]) < hostOf(out[j]) })
	return out
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Hostname()
}

func (s *shareServer) urlFor(rel string) string {
	base := "/s/" + s.token + "/"
	if rel == "" {
		return base
	}
	segs := strings.Split(rel, "/")
	for i := range segs {
		segs[i] = url.PathEscape(segs[i])
	}
	return base + strings.Join(segs, "/")
}

func (s *shareServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	prefix := "/s/" + s.token
	if r.URL.Path != prefix && !strings.HasPrefix(r.URL.Path, prefix+"/") {
		http.NotFound(w, r)
		return
	}
	rel := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if rel != "" {
		for _, seg := range strings.Split(rel, "/") {
			if seg == "" || seg == "." || seg == ".." {
				http.NotFound(w, r)
				return
			}
		}
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.handleGet(w, r, rel)
	case http.MethodPost:
		if !s.allowUpload {
			http.Error(w, "当前共享为只读", http.StatusForbidden)
			return
		}
		s.handleUpload(w, r, rel)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "不支持的方法", http.StatusMethodNotAllowed)
	}
}

func (s *shareServer) handleGet(w http.ResponseWriter, r *http.Request, rel string) {
	name := rel
	if name == "" {
		name = "."
	}
	f, err := s.root.Open(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "无法读取", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "无法读取", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		entries, err := f.ReadDir(-1)
		if err != nil {
			http.Error(w, "无法读取目录", http.StatusInternalServerError)
			return
		}
		s.renderDir(w, rel, entries)
		return
	}
	if !info.Mode().IsRegular() {
		http.Error(w, "不支持的文件类型", http.StatusForbidden)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func shareTitle(rel string) string {
	if rel == "" {
		return "共享根目录"
	}
	return path.Base(rel)
}
