package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func (s *shareServer) renderDir(w http.ResponseWriter, rel string, entries []os.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>`)
	b.WriteString(html.EscapeString(shareTitle(rel)))
	b.WriteString(` · 内网通共享</title>`)
	b.WriteString(shareCSS)
	b.WriteString(`</head><body><div class="wrap"><header><div><div class="brand">内网通共享</div><div class="sub">`)
	if s.allowUpload {
		b.WriteString(`可浏览 · 可下载 · 可上传`)
	} else {
		b.WriteString(`只读共享 · 可浏览 · 可下载`)
	}
	b.WriteString(`</div></div></header>`)

	b.WriteString(`<nav class="crumbs"><a href="`)
	b.WriteString(s.urlFor(""))
	b.WriteString(`">共享根目录</a>`)
	if rel != "" {
		cur := ""
		for _, seg := range strings.Split(rel, "/") {
			cur = path.Join(cur, seg)
			b.WriteString(`<span>/</span><a href="`)
			b.WriteString(s.urlFor(cur))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(seg))
			b.WriteString(`</a>`)
		}
	}
	b.WriteString(`</nav>`)

	if s.allowUpload {
		b.WriteString(`<form class="upload" method="post" action="`)
		b.WriteString(s.urlFor(rel))
		b.WriteString(`" enctype="multipart/form-data"><label class="pick"><input type="file" name="files" multiple required>选择文件（可多选）</label><button type="submit">上传到当前目录</button></form>`)
	}

	b.WriteString(`<div class="list">`)
	if rel != "" {
		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		b.WriteString(`<a class="row dir" href="`)
		b.WriteString(s.urlFor(parent))
		b.WriteString(`"><span class="ico">↰</span><span class="name">返回上层</span></a>`)
	}
	if len(entries) == 0 {
		b.WriteString(`<div class="empty">这个文件夹还是空的</div>`)
	}
	for _, e := range entries {
		name := e.Name()
		child := path.Join(rel, name)
		if e.IsDir() {
			b.WriteString(`<a class="row dir" href="`)
			b.WriteString(s.urlFor(child))
			b.WriteString(`"><span class="ico">📁</span><span class="name">`)
			b.WriteString(html.EscapeString(name))
			b.WriteString(`</span><span class="meta">文件夹</span></a>`)
			continue
		}
		size := int64(0)
		mod := ""
		if info, err := e.Info(); err == nil {
			size = info.Size()
			mod = info.ModTime().Format("2006-01-02 15:04")
		}
		b.WriteString(`<a class="row" href="`)
		b.WriteString(s.urlFor(child))
		b.WriteString(`"><span class="ico">📄</span><span class="name">`)
		b.WriteString(html.EscapeString(name))
		b.WriteString(`</span><span class="meta">`)
		b.WriteString(humanSize(size))
		b.WriteString(` · `)
		b.WriteString(mod)
		b.WriteString(`</span></a>`)
	}
	b.WriteString(`</div><footer>链接仅供局域网内使用，请只分享给信任的同事；拿到链接的人都能浏览和下载`)
	if s.allowUpload {
		b.WriteString(`，并可上传文件`)
	}
	b.WriteString(`。</footer></div></body></html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, b.String())
}

func (s *shareServer) handleUpload(w http.ResponseWriter, r *http.Request, rel string) {
	if rel != "" {
		f, err := s.root.Open(rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, err := f.Stat()
		_ = f.Close()
		if err != nil || !info.IsDir() {
			http.Error(w, "只能上传到文件夹", http.StatusBadRequest)
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxShareUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "上传失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "没有选择文件", http.StatusBadRequest)
		return
	}
	saved := 0
	for _, fh := range files {
		name := safeShareName(fh.Filename)
		in, err := fh.Open()
		if err != nil {
			continue
		}
		out, finalName, err := createShareFile(s.root, rel, name)
		if err != nil {
			_ = in.Close()
			http.Error(w, "创建文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_, copyErr := io.Copy(out, in)
		_ = in.Close()
		_ = out.Close()
		if copyErr != nil {
			http.Error(w, "写入失败: "+copyErr.Error(), http.StatusInternalServerError)
			return
		}
		saved++
		log.Printf("[share] 上传 %s -> %s", finalName, rel)
	}
	if saved == 0 {
		http.Error(w, "上传失败", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, s.urlFor(rel), http.StatusSeeOther)
}

func createShareFile(root *os.Root, rel, name string) (*os.File, string, error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 0; i < 1000; i++ {
		cand := name
		if i > 0 {
			cand = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		p := cand
		if rel != "" {
			p = path.Join(rel, cand)
		}
		f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, cand, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("同名文件过多")
}

func safeShareName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Base(name)
	name = strings.TrimSpace(name)
	name = strings.TrimRight(name, ". ")
	repl := strings.NewReplacer(":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")
	name = repl.Replace(name)
	if name == "" || name == "." || name == ".." {
		return "upload.bin"
	}
	switch strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name))) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	for _, u := range []string{"KB", "MB", "GB", "TB"} {
		f /= 1024
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%.1f PB", f/1024)
}

const shareCSS = `<style>
:root{color-scheme:light dark;--bg:#eef1f5;--surface:#fff;--text:#101826;--muted:#55617a;--border:#dde3ec;--accent:#1f6feb;--accent-2:#1557c0;--accent-fill:#1f6feb}
@media (prefers-color-scheme:dark){:root{--bg:#0d1320;--surface:#151d2c;--text:#e8edf7;--muted:#a3b0c6;--border:#273349;--accent:#6aa8ff;--accent-2:#8cbaff;--accent-fill:#2f6fdc}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:14px/1.6 "Segoe UI","Microsoft YaHei",-apple-system,sans-serif}
.wrap{max-width:920px;margin:0 auto;padding:32px 20px 48px}
header{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:18px}
.brand{font-size:20px;font-weight:700}
.sub{color:var(--muted);font-size:12px;margin-top:2px}
.crumbs{display:flex;flex-wrap:wrap;align-items:center;gap:4px;margin-bottom:16px;font-size:13px;color:var(--muted)}
.crumbs a{color:var(--accent-2);text-decoration:none}
.crumbs a:hover{text-decoration:underline}
.crumbs span{color:var(--muted)}
.upload{display:flex;flex-wrap:wrap;gap:10px;align-items:center;margin-bottom:18px;padding:14px;border:1px dashed var(--border);border-radius:14px;background:var(--surface)}
.upload input[type=file]{max-width:100%}
.upload button{padding:9px 16px;border:0;border-radius:7px;background:var(--accent-fill);color:#fff;font-weight:600;cursor:pointer}
.list{background:var(--surface);border:1px solid var(--border);border-radius:14px;overflow:hidden}
.row{display:flex;align-items:center;gap:12px;padding:11px 16px;border-bottom:1px solid var(--border);text-decoration:none;color:inherit}
.row:last-child{border-bottom:0}
.row:hover{background:color-mix(in srgb,var(--accent) 8%,transparent)}
.ico{width:22px;text-align:center}
.name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.meta{color:var(--muted);font-size:12px;white-space:nowrap}
.dir .name{font-weight:600}
.empty{padding:32px;text-align:center;color:var(--muted)}
footer{margin-top:18px;color:var(--muted);font-size:12px;line-height:1.7}
@media (max-width:560px){.wrap{padding:20px 12px 32px}header{flex-direction:column;gap:4px}.row{padding:10px 12px}.meta{display:none}}
</style>`
