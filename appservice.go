package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"neiwangtong/internal/ipmsg"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the service exposed to the frontend.
type App struct {
	mu      sync.Mutex
	engine  *ipmsg.Engine
	logPath string
	window  *application.WebviewWindow

	shareMu sync.Mutex
	share   *shareServer

	quitting atomic.Bool
	trayMode atomic.Bool
}

// NewApp creates a new App service.
func NewApp() *App {
	a := &App{}
	a.trayMode.Store(true)
	return a
}

// ServiceStartup is called by Wails when the service starts.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	logPath := filepath.Join(os.TempDir(), "neiwangtong.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		log.SetOutput(f)
		a.logPath = logPath
	}
	migrateLegacyData()
	migrateLegacyAutostart()
	log.Printf("==== 内网通 启动 %s ====", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// ServiceShutdown is called by Wails when the app is closing.
func (a *App) ServiceShutdown() error {
	a.Stop()
	a.StopShare()
	return nil
}

func (a *App) setWindow(w *application.WebviewWindow) { a.window = w }

func (a *App) showWindow() {
	if a.window == nil {
		return
	}
	a.window.Show()
	a.window.Focus()
}

func (a *App) trayEnabled() bool { return a.trayMode.Load() }

func (a *App) markQuit() { a.quitting.Store(true) }

// GetAutostart reports whether 内网通 is registered to launch at login.
func (a *App) GetAutostart() bool {
	app := application.Get()
	if app == nil || app.Autostart == nil {
		return false
	}
	on, err := app.Autostart.IsEnabled()
	if err != nil {
		return false
	}
	return on
}

// SetAutostart registers or removes the login-launch entry. When enabled the
// app starts hidden in the system tray.
func (a *App) SetAutostart(enabled bool) error {
	app := application.Get()
	if app == nil || app.Autostart == nil {
		return errors.New("当前平台不支持开机自启")
	}
	if enabled {
		return app.Autostart.EnableWithOptions(application.AutostartOptions{
			// The app name is Chinese, so give the registry value a stable
			// ASCII identifier instead of Wails' derived slug.
			Identifier: "neiwangtong",
			Arguments:  []string{"--hidden"},
		})
	}
	return app.Autostart.Disable()
}

func emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// LogPath returns the diagnostics log file path.
func (a *App) LogPath() string { return a.logPath }

// SetName changes the nickname and announces it to peers.
func (a *App) SetName(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine != nil {
		a.engine.SetName(name)
	}
}

// UserConfig is persisted to disk so settings survive restarts.
type UserConfig struct {
	Name           string `json:"name"`
	DownloadDir    string `json:"downloadDir"`
	AutoReceive    bool   `json:"autoReceive"`
	Avatar         string `json:"avatar"`
	MinimizeToTray bool   `json:"minimizeToTray"`

	// Night off (scheduled network adapter disable)
	NightOffEnabled bool   `json:"nightOffEnabled"`
	NightOffAdapter string `json:"nightOffAdapter"`
	NightOffStart   string `json:"nightOffStart"`
	NightOffEnd     string `json:"nightOffEnd"`

	// Shared folder (HTTP, LAN only)
	ShareDir         string `json:"shareDir"`
	SharePort        int    `json:"sharePort"`
	ShareAllowUpload bool   `json:"shareAllowUpload"`
}

func configBaseDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return dir
}

func configDir() string {
	return filepath.Join(configBaseDir(), "neiwangtong")
}

// legacyConfigDir is where builds named "FlyDog" stored their data.
func legacyConfigDir() string {
	return filepath.Join(configBaseDir(), "flydog")
}

// migrateLegacyData copies settings and history from the old data directory on
// first launch under the new name. The old directory is left untouched because
// scheduled night-off tasks installed by the old build still reference its
// script; those tasks are migrated when the schedule is re-applied.
func migrateLegacyData() {
	newDir := configDir()
	if _, err := os.Stat(filepath.Join(newDir, "config.json")); err == nil {
		return
	}
	oldDir := legacyConfigDir()
	if _, err := os.Stat(oldDir); err != nil {
		return
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return
	}
	for _, name := range []string{"config.json", "history.json", "nightoff.log"} {
		dst := filepath.Join(newDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(oldDir, name))
		if err != nil {
			continue
		}
		_ = os.WriteFile(dst, b, 0o644)
	}
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func historyPath() string {
	return filepath.Join(configDir(), "history.json")
}

// HistoryMsg is one persisted chat message.
type HistoryMsg struct {
	Peer     string   `json:"peer"` // "ip:port"
	PeerName string   `json:"peerName"`
	Dir      string   `json:"dir"` // in | out
	Kind     string   `json:"kind"`
	Text     string   `json:"text"`
	Files    []string `json:"files"`
	Time     string   `json:"time"`
}

// LoadHistory returns all persisted chat messages.
func (a *App) LoadHistory() []HistoryMsg {
	var out []HistoryMsg
	if b, err := os.ReadFile(historyPath()); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	if out == nil {
		return []HistoryMsg{}
	}
	return out
}

// SaveHistory overwrites the persisted chat history (keeps the newest entries).
func (a *App) SaveHistory(msgs []HistoryMsg) error {
	const maxKeep = 3000
	if len(msgs) > maxKeep {
		msgs = msgs[len(msgs)-maxKeep:]
	}
	p := historyPath()
	b, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return writeFileAtomic(p, b, 0o644)
}

// ClearHistory removes the persisted chat history.
func (a *App) ClearHistory() error {
	return os.Remove(historyPath())
}

func defaultUserConfig() UserConfig {
	host, _ := os.Hostname()
	return UserConfig{
		Name:             host,
		DownloadDir:      defaultDownloadDir(),
		AutoReceive:      true,
		MinimizeToTray:   true,
		NightOffStart:    "22:00",
		NightOffEnd:      "06:00",
		SharePort:        defaultSharePort,
		ShareAllowUpload: true,
	}
}

// LoadConfig reads the persisted configuration (falling back to defaults).
func (a *App) LoadConfig() UserConfig {
	cfg := defaultUserConfig()
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.Name == "" {
		cfg.Name, _ = os.Hostname()
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = defaultDownloadDir()
	}
	if cfg.NightOffStart == "" {
		cfg.NightOffStart = "22:00"
	}
	if cfg.NightOffEnd == "" {
		cfg.NightOffEnd = "06:00"
	}
	if cfg.SharePort == 0 {
		cfg.SharePort = defaultSharePort
	}
	a.trayMode.Store(cfg.MinimizeToTray)
	return cfg
}

// SaveConfig writes the configuration to disk.
func (a *App) SaveConfig(cfg UserConfig) error {
	a.trayMode.Store(cfg.MinimizeToTray)
	p := configPath()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, b, 0o644)
}

// writeFileAtomic writes data to a temp file in the same directory and renames
// it over path, so a crash mid-write cannot leave a truncated config/history
// file behind.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	_ = os.Chmod(tmp, perm)
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "neiwangtong-downloads"
	}
	return filepath.Join(home, "Downloads", "内网通")
}

// Start launches the LAN engine.
func (a *App) Start(name, downloadDir string, autoReceive bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine != nil {
		return nil
	}
	if downloadDir == "" {
		downloadDir = defaultDownloadDir()
	}
	log.Printf("[app] Start name=%q dir=%q auto=%v", name, downloadDir, autoReceive)
	_ = os.MkdirAll(downloadDir, 0o755)

	e := ipmsg.New(ipmsg.Config{
		Name:        name,
		DownloadDir: downloadDir,
		AutoReceive: autoReceive,
		Port:        ipmsg.Port,
	}, ipmsg.Callbacks{
		OnMembers:  func(ev ipmsg.MemberEvent) { emit("members", ev.Members) },
		OnMessage:  func(m ipmsg.Message) { emit("message", m) },
		OnTransfer: func(t ipmsg.Transfer) { emit("transfer", t) },
		OnError:    func(s string) { emit("engine:error", s) },
		OnLog:      func(s string) { emit("engine:log", s) },
	})
	if err := e.Start(); err != nil {
		log.Printf("[app] 启动失败: %v", err)
		return err
	}
	a.engine = e
	log.Printf("[app] 启动成功")
	return nil
}

// Stop shuts the engine down.
func (a *App) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine != nil {
		a.engine.Stop()
		a.engine = nil
	}
}

// IsRunning reports whether the engine is active.
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engine != nil
}

// GetMembers returns the current peer list.
func (a *App) GetMembers() []ipmsg.Member {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return []ipmsg.Member{}
	}
	return a.engine.Members()
}

// Rescan re-broadcasts our presence to refresh the peer list.
func (a *App) Rescan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine != nil {
		a.engine.Rescan()
	}
}

// AddPeer connects directly to a peer by IP, bypassing broadcast discovery.
func (a *App) AddPeer(ip string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return os.ErrClosed
	}
	return a.engine.AddPeer(ip)
}

// Scan probes an address range with unicast packets to discover peers across VLANs.
func (a *App) Scan(target string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return 0, os.ErrClosed
	}
	return a.engine.Scan(target)
}

// SendText sends a plain message to a peer.
func (a *App) SendText(ip string, port int, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return os.ErrClosed
	}
	return a.engine.SendText(ip, port, text)
}

// SendFiles sends files and/or folders to a peer.
func (a *App) SendFiles(ip string, port int, text string, paths []string) ([]ipmsg.FileInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return nil, os.ErrClosed
	}
	return a.engine.SendFiles(ip, port, text, paths)
}

// SendKnock sends a window shake to a peer.
func (a *App) SendKnock(ip string, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return os.ErrClosed
	}
	return a.engine.SendKnock(ip, port)
}

// SendTyping notifies a peer whether we are typing.
func (a *App) SendTyping(ip string, port int, typing bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return nil
	}
	return a.engine.SendTyping(ip, port, typing)
}

// OpenYou tells a peer we opened a chat with them.
func (a *App) OpenYou(ip string, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return nil
	}
	return a.engine.OpenYou(ip, port)
}

// PickFiles opens a native multi-file picker.
func (a *App) PickFiles() ([]string, error) {
	return application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                   "选择要发送的文件",
		CanChooseFiles:          true,
		AllowsMultipleSelection: true,
	}).PromptForMultipleSelection()
}

// PickFolder opens a native folder picker.
func (a *App) PickFolder() (string, error) {
	return application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "选择要发送的文件夹",
		CanChooseDirectories: true,
		CanChooseFiles:       false,
	}).PromptForSingleSelection()
}

// OpenDownloadDir opens the download folder in the system file manager.
func (a *App) OpenDownloadDir(dir string) error {
	if dir == "" {
		dir = defaultDownloadDir()
	}
	return application.Get().Env.OpenFileManager(dir, false)
}
