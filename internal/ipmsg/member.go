package ipmsg

import (
	"sort"
	"sync"
	"time"
)

// Member is a discovered peer on the LAN.
type Member struct {
	Name     string    `json:"name"`
	Host     string    `json:"host"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	Absence  bool      `json:"absence"`
	UTF8     bool      `json:"utf8"`
	LastSeen time.Time `json:"lastSeen"`
}

// MemberTable is a concurrency-safe set of peers keyed by ip:port.
type MemberTable struct {
	mu sync.RWMutex
	m  map[string]*Member
}

func newMemberTable() *MemberTable {
	return &MemberTable{m: make(map[string]*Member)}
}

func (t *MemberTable) key(ip string, port int) string {
	return ip + ":" + itoa(port)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

// Upsert adds or refreshes a member and reports whether it changed.
func (t *MemberTable) Upsert(m Member) (Member, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := t.key(m.IP, m.Port)
	cur, ok := t.m[k]
	if !ok {
		cp := m
		cp.LastSeen = time.Now()
		t.m[k] = &cp
		return cp, true
	}
	changed := cur.Name != m.Name || cur.Host != m.Host || cur.Absence != m.Absence || cur.UTF8 != m.UTF8
	cur.Name = m.Name
	cur.Host = m.Host
	cur.Absence = m.Absence
	cur.UTF8 = m.UTF8
	cur.LastSeen = time.Now()
	return *cur, changed
}

// Remove drops a member.
func (t *MemberTable) Remove(ip string, port int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, t.key(ip, port))
}

// List returns all members sorted by name.
func (t *MemberTable) List() []Member {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Member, 0, len(t.m))
	for _, v := range t.m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].IP < out[j].IP
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Expire removes members not seen for longer than ttl and returns the count.
func (t *MemberTable) Expire(ttl time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for k, v := range t.m {
		if time.Since(v.LastSeen) > ttl {
			delete(t.m, k)
			n++
		}
	}
	return n
}

// Lookup finds a member by ip:port.
func (t *MemberTable) Lookup(ip string, port int) (Member, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.m[t.key(ip, port)]
	if !ok {
		return Member{}, false
	}
	return *v, true
}
