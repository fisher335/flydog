package ipmsg

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// listenUDP binds a reusable, broadcast-capable UDP socket.
func listenUDP(port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) {
				serr = setSockOpts(fd)
			}); err != nil {
				return err
			}
			return serr
		},
	}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, fmt.Errorf("unexpected packet conn type %T", pc)
	}
	return uc, nil
}

// localIPv4Set returns the set of local IPv4 addresses.
func localIPv4Set() map[string]bool {
	set := make(map[string]bool)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return set
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				set[ip4.String()] = true
			}
		}
	}
	return set
}

// broadcastAddrs returns the global and per-interface IPv4 broadcast addresses.
func broadcastAddrs() []net.IP {
	seen := make(map[string]bool)
	var out []net.IP
	add := func(ip net.IP) {
		if ip == nil {
			return
		}
		if s := ip.String(); !seen[s] {
			seen[s] = true
			out = append(out, ip)
		}
	}
	add(net.IPv4bcast)

	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			mask := ipnet.Mask
			if len(mask) != 4 {
				continue
			}
			bcast := make(net.IP, 4)
			for i := 0; i < 4; i++ {
				bcast[i] = ip4[i] | ^mask[i]
			}
			add(bcast)
		}
	}
	return out
}
