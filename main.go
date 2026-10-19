package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var appIcon []byte

func main() {
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
		BackgroundColour: application.NewRGB(238, 241, 246),
		URL:              "/",
	})
	svc.setWindow(win)

	// 关闭按钮：最小化到托盘（除非用户选择直接退出或点托盘"退出"）
	win.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if svc.quitting.Load() || !svc.trayEnabled() {
			return
		}
		e.Cancel()
		win.Hide()
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

	app.OnShutdown(func() { svc.Stop() })

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
