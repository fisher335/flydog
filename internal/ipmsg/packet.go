package ipmsg

import (
	"bytes"
	"errors"
	"strconv"
)

// Packet is a decoded IPMSG datagram.
//
// Wire format:
//
//	Ver:PacketNo:SenderName:SenderHost:CommandNo:AdditionalSection
type Packet struct {
	Version    int
	VersionStr string // if set, used verbatim as the version field
	PacketNo   uint32
	Name       string // sender name (decoded to UTF-8)
	Host       string // sender host (decoded to UTF-8)
	Command    int
	Extra      []byte // raw additional section, may contain NUL/0x07

	// UTF8 reports whether this packet used UTF-8 for text fields.
	UTF8 bool
}

// Encode renders the packet back onto the wire.
func (p *Packet) Encode() []byte {
	cmd := p.Command
	if p.UTF8 {
		cmd |= OptUTF8
	}
	ver := p.VersionStr
	if ver == "" {
		ver = strconv.Itoa(p.Version)
	}
	name := sanitizeHeaderField(p.Name)
	host := sanitizeHeaderField(p.Host)
	head := ver + ":" +
		strconv.FormatUint(uint64(p.PacketNo), 10) + ":" +
		string(encodeText(name, p.UTF8)) + ":" +
		string(encodeText(host, p.UTF8)) + ":" +
		strconv.Itoa(cmd) + ":"
	buf := make([]byte, 0, len(head)+len(p.Extra))
	buf = append(buf, head...)
	buf = append(buf, p.Extra...)
	return buf
}

// sanitizeHeaderField removes characters that would corrupt the header.
func sanitizeHeaderField(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c == HostListSeparator {
			b[i] = ';'
		}
	}
	return string(b)
}

var errBadPacket = errors.New("ipmsg: malformed packet")

// atoiPrefix mimics C's atoi: parse leading digits and ignore any trailing
// text. IPMSG version fields are often extended strings such as
// "1_lbt6_0#128#MAC#0#0#0#4001#9", so a strict strconv.Atoi would reject them.
func atoiPrefix(s string) int {
	i := 0
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	n := 0
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}

// ParsePacket decodes a datagram into a Packet.
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 6 {
		return nil, errBadPacket
	}
	fields := make([][]byte, 0, 5)
	rest := data
	for len(fields) < 5 {
		i := bytes.IndexByte(rest, HostListSeparator)
		if i < 0 {
			return nil, errBadPacket
		}
		fields = append(fields, rest[:i])
		rest = rest[i+1:]
	}
	ver := atoiPrefix(string(fields[0]))
	pno, err := strconv.ParseUint(string(fields[1]), 10, 32)
	if err != nil {
		return nil, errBadPacket
	}
	cmd, err := strconv.Atoi(string(fields[4]))
	if err != nil {
		return nil, errBadPacket
	}
	p := &Packet{
		Version:  ver,
		PacketNo: uint32(pno),
		Command:  cmd,
		UTF8:     IsSet(cmd, OptUTF8),
	}
	p.Name = decodeText(fields[2], p.UTF8)
	p.Host = decodeText(fields[3], p.UTF8)
	p.Extra = append([]byte(nil), rest...)
	return p, nil
}
