package ipmsg

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxScanAddrs caps how many addresses a single scan will probe.
const maxScanAddrs = 8192

// Scan probes a range of addresses with unicast BR_ENTRY packets. Unlike
// broadcast discovery this works across VLANs/subnets (as long as unicast UDP
// is allowed), which is the same trick 内网通 uses.
//
// target accepts:
//
//	""                     -> scan the local /24 of every interface
//	"172.17.32.0/24"       -> CIDR
//	"172.17.32.1-172.17.32.254" / "172.17.32.1-254" -> range
//	"172.17.32."            -> 1..254 of the /24
//	"172.17.32.19"          -> single address
func (e *Engine) Scan(target string) (int, error) {
	if !e.started.Load() {
		return 0, errors.New("引擎未启动")
	}
	ips, err := parseScanTarget(target, e.localIPs)
	if err != nil {
		return 0, err
	}
	if len(ips) > maxScanAddrs {
		ips = ips[:maxScanAddrs]
	}
	e.logf("开始扫描网段: %d 个地址 (UDP 单播 + TCP:%d 探测)", len(ips), e.cfg.TCPPort)

	extra := encodeText(e.Name(), e.cfg.AdvertiseUTF8)
	go func() {
		// 1) UDP unicast BR_ENTRY (works within the same VLAN).
		for i, ip := range ips {
			p := &Packet{
				Version:    Version,
				VersionStr: e.versionStr,
				PacketNo:   e.nextPacketNo(),
				Name:       e.Name(),
				Host:       e.cfg.Host,
				Command:    CmdBrEntry | OptFileAttach,
				Extra:      extra,
				UTF8:       e.cfg.AdvertiseUTF8,
			}
			dst := &net.UDPAddr{IP: ip, Port: e.cfg.Port}
			if _, err := e.udp.WriteToUDP(p.Encode(), dst); err != nil {
				e.logf("扫描发送失败 %s: %v", ip, err)
			}
			if i%50 == 49 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		e.logf("UDP 扫描完成: %d 个地址", len(ips))

		// 2) TCP peer-channel probe (works across VLANs where UDP is dropped).
		sem := make(chan struct{}, 64)
		var wg sync.WaitGroup
		for _, ip := range ips {
			ip := ip
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				e.dialTCPPeer(ip.String())
			}()
		}
		wg.Wait()
		e.logf("TCP 扫描完成")
	}()
	return len(ips), nil
}

func parseScanTarget(target string, local map[string]bool) ([]net.IP, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		var ips []net.IP
		seen := make(map[string]bool)
		for ipstr := range local {
			ip := net.ParseIP(ipstr).To4()
			if ip == nil {
				continue
			}
			for i := 1; i < 255; i++ {
				cand := net.IPv4(ip[0], ip[1], ip[2], byte(i))
				if seen[cand.String()] {
					continue
				}
				seen[cand.String()] = true
				ips = append(ips, cand)
			}
		}
		return ips, nil
	}

	if strings.Contains(target, "/") {
		ip, ipnet, err := net.ParseCIDR(target)
		if err != nil {
			return nil, fmt.Errorf("无效的网段: %q", target)
		}
		if ip.To4() == nil {
			return nil, fmt.Errorf("仅支持 IPv4 网段: %q", target)
		}
		var ips []net.IP
		cur := ip.Mask(ipnet.Mask)
		for ipnet.Contains(cur) && len(ips) < maxScanAddrs {
			ips = append(ips, net.IPv4(cur[0], cur[1], cur[2], cur[3]))
			if !incIP(cur) {
				break
			}
		}
		return ips, nil
	}

	if strings.Contains(target, "-") {
		parts := strings.SplitN(target, "-", 2)
		start := net.ParseIP(strings.TrimSpace(parts[0])).To4()
		if start == nil {
			return nil, fmt.Errorf("无效的起始地址: %q", parts[0])
		}
		endStr := strings.TrimSpace(parts[1])
		var end net.IP
		if !strings.Contains(endStr, ".") {
			o, err := strconv.Atoi(endStr)
			if err != nil || o < 0 || o > 255 {
				return nil, fmt.Errorf("无效的结束地址: %q", endStr)
			}
			end = net.IPv4(start[0], start[1], start[2], byte(o)).To4()
		} else {
			end = net.ParseIP(endStr).To4()
			if end == nil {
				return nil, fmt.Errorf("无效的结束地址: %q", endStr)
			}
		}
		if compareIPv4(start, end) > 0 {
			return nil, fmt.Errorf("结束地址 %s 小于起始地址 %s", end, start)
		}
		// net.IPv4 returns a 16-byte IPv4-in-IPv6 address; indexing it would
		// read the prefix bytes instead of the address, so stay on 4-byte
		// slices here.
		cur := append(net.IP(nil), start...)
		var ips []net.IP
		for len(ips) < maxScanAddrs {
			ips = append(ips, append(net.IP(nil), cur...))
			if cur.Equal(end) {
				break
			}
			if !incIP(cur) {
				break
			}
		}
		return ips, nil
	}

	if strings.HasSuffix(target, ".") {
		var ips []net.IP
		for i := 1; i < 255; i++ {
			if ip := net.ParseIP(fmt.Sprintf("%s%d", target, i)); ip != nil {
				ips = append(ips, ip)
			}
		}
		return ips, nil
	}

	ip := net.ParseIP(target).To4()
	if ip == nil {
		return nil, fmt.Errorf("无效的地址: %q", target)
	}
	return []net.IP{ip}, nil
}

// compareIPv4 compares two 4-byte IPv4 addresses.
func compareIPv4(a, b net.IP) int {
	for i := 0; i < 4; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// incIP increments ip in place and reports false on overflow.
func incIP(ip net.IP) bool {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return true
		}
	}
	return false
}
