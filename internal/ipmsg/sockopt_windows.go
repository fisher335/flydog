//go:build windows

package ipmsg

import "golang.org/x/sys/windows"

// setSockOpts enables address reuse and broadcast on the socket handle.
func setSockOpts(fd uintptr) error {
	h := windows.Handle(fd)
	if err := windows.SetsockoptInt(h, windows.SOL_SOCKET, windows.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return windows.SetsockoptInt(h, windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
}
