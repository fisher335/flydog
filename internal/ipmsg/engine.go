package ipmsg

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds the engine settings.
type Config struct {
	Name        string
	Host        string
	Port        int
	TCPPort     int // TCP peer-channel port (cross-VLAN transport), default 2426
	DownloadDir string
	AutoReceive bool
	// AdvertiseUTF8 makes outgoing broadcast/discovery packets advertise UTF-8
	// capability. Messages to peers that also advertise it are sent as UTF-8.
	AdvertiseUTF8 bool
}

// MemberEvent is emitted when the member list changes.
type MemberEvent struct {
	Members []Member `json:"members"`
}

// Message is a received chat message (optionally with attachments).
type Message struct {
	Peer   Member     `json:"peer"`
	Kind   string     `json:"kind"` // text | knock | input | inputend | image | open
	Text   string     `json:"text"`
	Format string     `json:"format,omitempty"`
	Files  []FileInfo `json:"files"`
	Time   time.Time  `json:"time"`
}

// TransferState enumerates transfer progress states.
type TransferState string

const (
	StateRunning TransferState = "running"
	StateDone    TransferState = "done"
	StateError   TransferState = "error"
)

// Transfer is a progress notification for an upload or download.
type Transfer struct {
	ID        string        `json:"id"`
	Direction string        `json:"direction"` // "send" | "recv"
	Peer      string        `json:"peer"`
	Name      string        `json:"name"`
	Size      uint64        `json:"size"`
	Done      uint64        `json:"done"`
	State     TransferState `json:"state"`
	Error     string        `json:"error,omitempty"`
}

// Callbacks receives engine notifications. All are optional.
type Callbacks struct {
	OnMembers  func(MemberEvent)
	OnMessage  func(Message)
	OnTransfer func(Transfer)
	OnError    func(string)
	OnLog      func(string)
}

type outgoing struct {
	path   string
	info   FileInfo
	isDir  bool
	remote string
	utf8   bool
}

// Engine implements the IPMSG UDP discovery/messaging and TCP file transfer.
type Engine struct {
	cfg Config
	cb  Callbacks

	udp *net.UDPConn
	tcp net.Listener

	tcpLn   net.Listener
	peers   map[string]*peerConn
	peersMu sync.Mutex
	nameMu  sync.RWMutex

	members *MemberTable

	packetNo   uint32
	fileID     uint32
	versionStr string

	mu       sync.Mutex
	outgoing map[string]*outgoing

	progMu   sync.Mutex
	progLast map[string]time.Time

	localIPs map[string]bool

	quit    chan struct{}
	started atomic.Bool
	wg      sync.WaitGroup
}

// New creates an engine.
func New(cfg Config, cb Callbacks) *Engine {
	if cfg.Port == 0 {
		cfg.Port = Port
	}
	if cfg.TCPPort == 0 {
		cfg.TCPPort = 2426
	}
	if cfg.Name == "" {
		cfg.Name = "flydog"
	}
	if cfg.Host == "" {
		cfg.Host, _ = os.Hostname()
	}
	e := &Engine{
		cfg:      cfg,
		cb:       cb,
		members:  newMemberTable(),
		outgoing: make(map[string]*outgoing),
		progLast: make(map[string]time.Time),
		peers:    make(map[string]*peerConn),
		quit:     make(chan struct{}),
	}
	e.packetNo = uint32(rand.Int31())
	e.fileID = uint32(rand.Int31())
	e.versionStr = buildVersionString()
	e.localIPs = localIPv4Set()
	return e
}

// buildVersionString produces a 飞秋-compatible version field, e.g.
// "1_lbt6_0#128#AABBCCDDEEFF#0#0#0#4001#9". 飞秋 uses the MAC embedded here to
// identify peers, so mimicking it maximises interoperability.
func buildVersionString() string {
	mac := ""
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagLoopback != 0 || len(ifc.HardwareAddr) == 0 {
				continue
			}
			mac = strings.ToUpper(strings.ReplaceAll(ifc.HardwareAddr.String(), ":", ""))
			break
		}
	}
	if mac == "" {
		mac = "000000000000"
	}
	return "1_lbt6_0#128#" + mac + "#0#0#0#4001#9"
}

// Members returns the current member list.
func (e *Engine) Members() []Member { return e.members.List() }

// Start binds sockets and launches the background loops.
func (e *Engine) Start() error {
	if e.started.Load() {
		return nil
	}
	e.logf("启动引擎 name=%q host=%q port=%d downloadDir=%q", e.Name(), e.cfg.Host, e.cfg.Port, e.cfg.DownloadDir)

	udp, err := listenUDP(e.cfg.Port)
	if err != nil {
		e.logf("UDP 绑定 :%d 失败: %v", e.cfg.Port, err)
		return fmt.Errorf("bind udp %d: %w", e.cfg.Port, err)
	}
	e.udp = udp
	e.logf("UDP 绑定成功 :%d", e.cfg.Port)

	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", e.cfg.Port))
	if err != nil {
		e.logf("TCP 绑定 :%d 失败: %v（常见原因: 本机已有飞秋/飞鸽等占用该端口）", e.cfg.Port, err)
		udp.Close()
		return fmt.Errorf("bind tcp %d: %w", e.cfg.Port, err)
	}
	e.tcp = tcp
	e.logf("TCP 绑定成功 :%d", e.cfg.Port)

	peers, err := net.Listen("tcp", fmt.Sprintf(":%d", e.cfg.TCPPort))
	if err != nil {
		e.logf("TCP 对等通道绑定 :%d 失败: %v", e.cfg.TCPPort, err)
	} else {
		e.tcpLn = peers
		e.logf("TCP 对等通道监听成功 :%d", e.cfg.TCPPort)
	}

	ips := make([]string, 0, len(e.localIPs))
	for ip := range e.localIPs {
		ips = append(ips, ip)
	}
	e.logf("本机 IPv4: %v", ips)

	e.started.Store(true)
	e.wg.Add(4)
	go e.udpLoop()
	go e.tcpLoop()
	go e.broadcastLoop()
	go e.expireLoop()
	if e.tcpLn != nil {
		e.wg.Add(1)
		go e.peerLoop()
	}
	return nil
}

// Stop broadcasts our exit and closes all sockets.
func (e *Engine) Stop() {
	if !e.started.Swap(false) {
		return
	}
	e.broadcast(CmdBrExit, encodeText(e.Name(), false), false)
	time.Sleep(60 * time.Millisecond)
	close(e.quit)
	if e.udp != nil {
		e.udp.Close()
	}
	if e.tcp != nil {
		e.tcp.Close()
	}
	e.closeAllPeers()
	if e.tcpLn != nil {
		e.tcpLn.Close()
	}
	e.wg.Wait()
}

func (e *Engine) nextPacketNo() uint32 { return atomic.AddUint32(&e.packetNo, 1) }

func (e *Engine) nextFileID() uint32 { return atomic.AddUint32(&e.fileID, 1) }

func (e *Engine) errf(format string, a ...any) {
	if e.cb.OnError != nil {
		e.cb.OnError(fmt.Sprintf(format, a...))
	}
}

// logf reports a diagnostic message to the callback (and stdout for dev).
func (e *Engine) logf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if e.cb.OnLog != nil {
		e.cb.OnLog(msg)
	}
	log.Println("[ipmsg]", msg)
}

// Rescan re-announces our presence so peers reply with their entry info.
func (e *Engine) Rescan() {
	if !e.started.Load() {
		return
	}
	e.announce()
}

// Name returns the current nickname (thread-safe).
func (e *Engine) Name() string {
	e.nameMu.RLock()
	defer e.nameMu.RUnlock()
	return e.cfg.Name
}

// SetName updates the nickname and, when running, announces the change to peers
// (via a BR_ABSENCE packet, which is how IPMSG clients broadcast a rename).
func (e *Engine) SetName(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	e.nameMu.Lock()
	e.cfg.Name = name
	e.nameMu.Unlock()
	if e.started.Load() {
		e.broadcast(CmdBrAbsence, encodeText(name, e.cfg.AdvertiseUTF8), e.cfg.AdvertiseUTF8)
	}
}

func (e *Engine) announce() {
	e.broadcast(CmdBrEntry|OptFileAttach,
		encodeText(e.Name(), e.cfg.AdvertiseUTF8), e.cfg.AdvertiseUTF8)
}

// AddPeer sends a direct (unicast) entry announcement to a single peer IP,
// bypassing broadcast discovery. Useful when broadcast is filtered by the
// network or by security software.
func (e *Engine) AddPeer(ip string) error {
	if !e.started.Load() {
		return errors.New("引擎未启动")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("无效的 IP 地址: %q", ip)
	}
	e.logf("手动添加用户（单播上线）: %s:%d", ip, e.cfg.Port)
	if e.dialTCPPeer(ip) {
		e.logf("已通过 TCP 通道连接 %s", ip)
	}
	return e.send(ip, e.cfg.Port, CmdBrEntry|OptFileAttach,
		encodeText(e.Name(), e.cfg.AdvertiseUTF8), e.cfg.AdvertiseUTF8, 0)
}

func (e *Engine) notifyMembers() {
	if e.cb.OnMembers != nil {
		e.cb.OnMembers(MemberEvent{Members: e.members.List()})
	}
}

// ---- UDP ----

func (e *Engine) udpLoop() {
	defer e.wg.Done()
	buf := make([]byte, MaxUDPPacket)
	for {
		n, addr, err := e.udp.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-e.quit:
				return
			default:
			}
			continue
		}
		pkt, err := ParsePacket(buf[:n])
		if err != nil {
			continue
		}
		e.handle(pkt, addr)
	}
}

func (e *Engine) handle(pkt *Packet, addr *net.UDPAddr) {
	e.handleFrom(pkt, addr.IP.String(), addr.Port)
}

// handleFrom processes a decoded packet regardless of the transport it arrived
// on (UDP or the TCP peer channel).
func (e *Engine) handleFrom(pkt *Packet, ip string, port int) {
	if e.localIPs[ip] && pkt.Name == e.Name() && pkt.Host == e.cfg.Host {
		return // our own broadcast
	}
	e.logf("收到 cmd=0x%x 来自 %s:%d name=%q", GetMode(pkt.Command), ip, port, pkt.Name)

	m := Member{
		Name:    pkt.Name,
		Host:    pkt.Host,
		IP:      ip,
		Port:    port,
		Absence: IsSet(pkt.Command, OptAbsence),
		UTF8:    pkt.UTF8,
	}

	// 飞秋/飞鸽把用户昵称放在附加区，报文头里的 SenderName 通常是登录名。
	// 对上线类报文以附加区为准；其它报文沿用已知昵称，避免被登录名覆盖。
	switch GetMode(pkt.Command) {
	case CmdBrEntry, CmdAnsEntry, CmdBrAbsence:
		if n, g := parseEntryExtra(pkt.Extra, pkt.UTF8); n != "" {
			m.Name = n
			m.Group = g
		}
	default:
		if info, ok := e.members.Lookup(ip, port); ok {
			m.Name = info.Name
			m.Group = info.Group
		}
	}

	switch GetMode(pkt.Command) {
	case CmdNoOperation:
		return
	case CmdBrExit:
		e.members.Remove(ip, port)
		e.notifyMembers()
		return
	case CmdBrEntry, CmdAnsEntry, CmdBrAbsence:
		if _, changed := e.members.Upsert(m); changed {
			e.logf("发现用户 %q (%s:%d) cmd=0x%x group=%q", m.Name, ip, port, GetMode(pkt.Command), m.Group)
			e.notifyMembers()
		}
		if GetMode(pkt.Command) == CmdBrEntry {
			_ = e.send(ip, port, CmdAnsEntry, encodeText(e.Name(), pkt.UTF8), pkt.UTF8, 0)
		}
		return
	case CmdGetInfo:
		_ = e.send(ip, port, CmdSendInfo, []byte("FlyDog ver 1.0"), pkt.UTF8, 0)
		return
	}

	// From here the peer is considered online.
	if _, changed := e.members.Upsert(m); changed {
		e.notifyMembers()
	}
	peer, _ := e.members.Lookup(ip, port)

	switch GetMode(pkt.Command) {
	case CmdSendMsg:
		if IsSet(pkt.Command, OptSendCheck) {
			_ = e.send(ip, port, CmdRecvMsg, []byte(strconv.FormatUint(uint64(pkt.PacketNo), 10)), pkt.UTF8, 0)
		}
		text, files := decodeAttachment(pkt.Extra, pkt.UTF8)
		text, format := splitTextFormat(text)
		e.emit(Message{Peer: peer, Kind: "text", Text: text, Format: format, Files: files, Time: time.Now()})
		if e.cfg.AutoReceive && len(files) > 0 {
			e.receiveFiles(peer, pkt.PacketNo, files, pkt.UTF8)
		}
	case CmdKnock:
		e.emit(Message{Peer: peer, Kind: "knock", Time: time.Now()})
	case CmdInputing:
		e.emit(Message{Peer: peer, Kind: "input", Time: time.Now()})
	case CmdInputEnd:
		e.emit(Message{Peer: peer, Kind: "inputend", Time: time.Now()})
	case CmdOpenYou:
		e.emit(Message{Peer: peer, Kind: "open", Time: time.Now()})
	case CmdSendImage:
		e.emit(Message{Peer: peer, Kind: "image", Time: time.Now()})
	case CmdReleaseFiles:
		return
	}
}

func (e *Engine) emit(m Message) {
	if e.cb.OnMessage != nil {
		e.cb.OnMessage(m)
	}
}

// splitTextFormat splits a 飞秋 text body of the form "text{font-format}".
func splitTextFormat(s string) (string, string) {
	if len(s) < 3 || s[len(s)-1] != '}' {
		return s, ""
	}
	if i := strings.IndexByte(s, '{'); i >= 0 && i < len(s)-1 {
		return s[:i], s[i+1 : len(s)-1]
	}
	return s, ""
}

func (e *Engine) reply(addr *net.UDPAddr, cmd int, extra []byte, utf8Mode bool, packetNo uint32) {
	e.send(addr.IP.String(), addr.Port, cmd, extra, utf8Mode, packetNo)
}

func (e *Engine) send(ip string, port int, cmd int, extra []byte, utf8Mode bool, packetNo uint32) error {
	if packetNo == 0 {
		packetNo = e.nextPacketNo()
	}
	p := &Packet{
		Version:    Version,
		VersionStr: e.versionStr,
		PacketNo:   packetNo,
		Name:       e.Name(),
		Host:       e.cfg.Host,
		Command:    cmd,
		Extra:      extra,
		UTF8:       utf8Mode,
	}
	data := p.Encode()
	if e.sendViaTCP(ip, data) {
		return nil
	}
	dst := &net.UDPAddr{IP: net.ParseIP(ip), Port: port}
	_, err := e.udp.WriteToUDP(data, dst)
	return err
}

func (e *Engine) broadcast(cmd int, extra []byte, utf8Mode bool) {
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
	data := p.Encode()
	addrs := broadcastAddrs()
	for _, b := range addrs {
		dst := &net.UDPAddr{IP: b, Port: e.cfg.Port}
		if _, err := e.udp.WriteToUDP(data, dst); err != nil {
			e.logf("广播 cmd=0x%x -> %s 失败: %v", cmd, dst, err)
		}
	}
	e.logf("广播 cmd=0x%x -> %v", cmd, addrs)
}

func (e *Engine) broadcastLoop() {
	defer e.wg.Done()
	// Announce a few times shortly after startup: UDP broadcasts can be lost,
	// and peers that started earlier only answer when they hear an entry.
	for _, d := range []time.Duration{150 * time.Millisecond, 1 * time.Second, 3 * time.Second} {
		select {
		case <-time.After(d):
		case <-e.quit:
			return
		}
		e.announce()
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.quit:
			return
		case <-ticker.C:
			e.announce()
		}
	}
}

func (e *Engine) expireLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.quit:
			return
		case <-ticker.C:
			if n := e.members.Expire(90 * time.Second); n > 0 {
				e.notifyMembers()
			}
		}
	}
}

// ---- Messaging API ----

// SendText sends a plain text message to a peer.
func (e *Engine) SendText(ip string, port int, text string) error {
	if !e.started.Load() {
		return errors.New("engine not started")
	}
	member, ok := e.members.Lookup(ip, port)
	utf8Mode := ok && member.UTF8
	return e.send(ip, port, CmdSendMsg|OptSendCheck, encodeText(text, utf8Mode), utf8Mode, 0)
}

// SendKnock sends a window knock (shake) to a peer (飞秋 extension).
func (e *Engine) SendKnock(ip string, port int) error {
	if !e.started.Load() {
		return errors.New("engine not started")
	}
	return e.send(ip, port, CmdKnock, nil, false, 0)
}

// SendTyping notifies a peer that we started/stopped typing.
func (e *Engine) SendTyping(ip string, port int, typing bool) error {
	if !e.started.Load() {
		return nil
	}
	cmd := CmdInputEnd
	if typing {
		cmd = CmdInputing
	}
	return e.send(ip, port, cmd, nil, false, 0)
}

// OpenYou tells a peer that we opened a chat window with them (飞秋 extension).
func (e *Engine) OpenYou(ip string, port int) error {
	if !e.started.Load() {
		return nil
	}
	return e.send(ip, port, CmdOpenYou, nil, false, 0)
}
