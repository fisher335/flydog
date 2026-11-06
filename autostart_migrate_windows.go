//go:build windows

package main

import (
	"errors"
	"log"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows/registry"
)

const (
	legacyAutostartValue = "flydog"
	autostartValue       = "neiwangtong"
)

// migrateLegacyAutostart carries an existing "launch at login" preference
// across the rename. Wails names the registry value after the application
// name, so the old "flydog" value would otherwise keep launching the old
// binary (or silently vanish from the new app's settings).
func migrateLegacyAutostart() {
	app := application.Get()
	if app == nil || app.Autostart == nil {
		return
	}
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	cmd, _, err := key.GetStringValue(legacyAutostartValue)
	if err != nil {
		return // no legacy entry
	}
	if !strings.Contains(strings.ToLower(cmd), "flydog") {
		return
	}
	if err := app.Autostart.EnableWithOptions(application.AutostartOptions{
		Identifier: autostartValue,
		Arguments:  []string{"--hidden"},
	}); err != nil {
		log.Printf("[autostart] 迁移旧自启动项失败: %v", err)
		return
	}
	if err := key.DeleteValue(legacyAutostartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		log.Printf("[autostart] 删除旧自启动项失败: %v", err)
	}
}
