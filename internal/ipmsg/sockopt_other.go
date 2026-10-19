//go:build !windows

package ipmsg

import "syscall"

// setSockOpts enables address reuse and broadcast on the socket handle.
func setSockOpts(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
