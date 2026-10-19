package ipmsg

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// The TCP peer channel carries the very same IPMSG packets as UDP, framed with a
// 4-byte big-endian length prefix. It exists because some corporate networks
// allow TCP between VLANs but drop UDP (so broadcast discovery fails), while TCP
// still works. It interoperates flydog<->flydog only; real 飞秋/飞鸽 clients only
// speak UDP 2425.

type peerConn struct {
	conn net.Conn
	mu   sync.Mutex
}

func (p *peerConn) writeFrame(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := p.conn.Write(hdr[:]); err != nil {
		return err
	}
	_, err := p.conn.Write(data)
	return err
}

func (e *Engine) peerLoop() {
	defer e.wg.Done()
	for {
		conn, err := e.tcpLn.Accept()
		if err != nil {
			select {
			case <-e.quit:
				return
			default:
				continue
			}
		}
		go e.servePeer(conn)
	}
}

// servePeer reads framed packets from an accepted peer connection.
func (e *Engine) servePeer(conn net.Conn) {
	remote, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		conn.Close()
		return
	}
	ip := remote.IP.String()
	if e.localIPs[ip] {
		conn.Close()
		return
	}
	pc := e.addPeer(ip, conn)
	if pc == nil {
		return
	}
	defer e.dropPeer(ip, conn)
	e.logf("TCP 对等连接建立(入站): %s", ip)

	// Greet the new peer over the channel.
	e.sendOnPeer(ip, CmdBrEntry|OptFileAttach, encodeText(e.Name(), e.cfg.AdvertiseUTF8), e.cfg.AdvertiseUTF8)
	e.readPeer(ip, conn)
}

func (e *Engine) readPeer(ip string, conn net.Conn) {
	_ = conn.SetReadDeadline(time.Time{})
	hdr := make([]byte, 4)
	for {
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(hdr)
		if n == 0 || n > MaxUDPPacket {
			return
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		pkt, err := ParsePacket(buf)
		if err != nil {
			e.logf("TCP 收到无法解析的帧(from %s, len=%d): %v | %q", ip, len(buf), err, string(buf))
			continue
		}
		e.logf("TCP 收到 cmd=0x%x 来自 %s", GetMode(pkt.Command), ip)
		// Members reached over TCP use the standard UDP port so that file
		// transfer (TCP 2425) keeps working across subnets.
		e.handleFrom(pkt, ip, e.cfg.Port)
	}
}
func (e *Engine) dialTCPPeer(ip string) bool {
	return e.dialTCPPeerPort(ip, e.cfg.TCPPort)
}

func (e *Engine) dialTCPPeerPort(ip string, port int) bool {
	if _, ok := e.getPeer(ip); ok {
		return true
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", port)), 600*time.Millisecond)
	if err != nil {
		return false
	}
	if e.addPeer(ip, conn) == nil {
		conn.Close()
		return false
	}
	e.logf("TCP 对等连接建立(出站): %s", ip)
	e.sendOnPeer(ip, CmdBrEntry|OptFileAttach, encodeText(e.Name(), e.cfg.AdvertiseUTF8), e.cfg.AdvertiseUTF8)
	go e.readPeer(ip, conn)
	return true
}

// sendOnPeer sends a raw packet over an existing TCP peer connection.
func (e *Engine) sendOnPeer(ip string, cmd int, extra []byte, utf8Mode bool) {
	pc := e.getPeerConn(ip)
	if pc == nil {
		return
	}
	p := &Packet{
		Version:    Version,
		VersionStr: e.versionStr,
		PacketNo:   e.nextPacketNo(),
		Name:       e.Name(),
		Host:       e.cfg.Host,
		Command:    cmd,
		Extra:      extra,
		UTF8:       utf8Mode,
	}
	_ = pc.writeFrame(p.Encode())
}

// sendViaTCP reports whether data was delivered over a TCP peer channel.
func (e *Engine) sendViaTCP(ip string, data []byte) bool {
	pc := e.getPeerConn(ip)
	if pc == nil {
		return false
	}
	if err := pc.writeFrame(data); err != nil {
		e.dropPeer(ip, pc.conn)
		return false
	}
	return true
}

func (e *Engine) getPeer(ip string) (*peerConn, bool) {
	e.peersMu.Lock()
	defer e.peersMu.Unlock()
	pc, ok := e.peers[ip]
	return pc, ok
}

func (e *Engine) getPeerConn(ip string) *peerConn {
	pc, _ := e.getPeer(ip)
	return pc
}

func (e *Engine) addPeer(ip string, conn net.Conn) *peerConn {
	e.peersMu.Lock()
	defer e.peersMu.Unlock()
	if existing, ok := e.peers[ip]; ok {
		// Keep the first connection; drop the duplicate.
		if existing.conn != conn {
			conn.Close()
		}
		return nil
	}
	pc := &peerConn{conn: conn}
	e.peers[ip] = pc
	return pc
}

func (e *Engine) dropPeer(ip string, conn net.Conn) {
	e.peersMu.Lock()
	defer e.peersMu.Unlock()
	if pc, ok := e.peers[ip]; ok && pc.conn == conn {
		delete(e.peers, ip)
	}
	conn.Close()
}

func (e *Engine) closeAllPeers() {
	e.peersMu.Lock()
	defer e.peersMu.Unlock()
	for _, pc := range e.peers {
		pc.conn.Close()
	}
	e.peers = make(map[string]*peerConn)
}
