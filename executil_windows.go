//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents console programs (powershell/schtasks/...) spawned by the
// GUI app from flashing a console window.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	// CREATE_NO_WINDOW
	cmd.SysProcAttr.CreationFlags |= 0x08000000
}
