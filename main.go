package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var appIcon []byte

func main() {
	// Launched by the OS at login: start hidden in the tray.
	startHidden := false
	for _, arg := range os.Args[1:] {
		if arg == "--hidden" {
			startHidden = true
		}
	}

	svc := NewApp()

	app := application.New(application.Options{
		Name:        "FlyDog",
		Description: "局域网消息 / 文件传输",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.flydog.lan",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				svc.showWindow()
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "FlyDog",
		Width:            1060,
		Height:           760,
		MinWidth:         880,
		MinHeight:        560,
		Hidden:           startHidden,
		BackgroundColour: application.NewRGB(238, 241, 246),
		URL:              "/",
	})
	svc.setWindow(win)

	// 关闭按钮：最小化到托盘。
	// 必须用 RegisterHook（同步）监听“平台级”关闭事件：OnWindowEvent 的回调
	// 在 goroutine 中异步执行，e.Cancel() 来不及阻止默认的销毁监听器。
	win.RegisterHook(closingEvent(), func(e *application.WindowEvent) {
		if svc.quitting.Load() || !svc.trayEnabled() {
			return
		}
		e.Cancel()
		go win.Hide()
	})

	// 系统托盘
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("FlyDog 局域网传输")
	menu := app.NewMenu()
	menu.Add("显示主窗口").OnClick(func(*application.Context) { svc.showWindow() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		svc.markQuit()
		app.Quit()
	})
	tray.SetMenu(menu)

	// 双击托盘图标打开主窗口
	tray.OnDoubleClick(func() {
		svc.showWindow()
	})

	app.OnShutdown(func() { svc.Stop() })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// closingEvent returns the platform-specific "window about to close" event.
// It must be the platform event (not the mapped Common event) so a synchronous
// hook can cancel it before Wails' default close listener runs.
func closingEvent() events.WindowEventType {
	switch runtime.GOOS {
	case "windows":
		return events.Windows.WindowClosing
	case "darwin":
		return events.Mac.WindowShouldClose
	default:
		return events.Linux.WindowDeleteEvent
	}
}
