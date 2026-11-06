package ipmsg

import (
	"strconv"
	"strings"
	"time"
)

// coalesceWindow is how long a text message waits for more fragments from the
// same peer before it is emitted.
//
// 飞秋 / 飞鸽 send a pasted multi-line message as one SENDMSG packet per line,
// so the newline never reaches us: without coalescing the chat shows a separate
// message for every line. Buffering a short burst and joining the fragments
// with '\n' restores the original text.
//
// The window resets with every fragment, so a long paste still merges as long
// as consecutive packets are less than this far apart. 300 ms keeps the added
// latency imperceptible while staying well below human typing speed.
const coalesceWindow = 300 * time.Millisecond

// pendingText buffers one peer's text burst.
type pendingText struct {
	peer     Member
	text     string
	format   string
	deadline time.Time
	timer    *time.Timer
}

func msgKey(ip string, port int) string {
	return ip + ":" + strconv.Itoa(port)
}

// queueText appends a text fragment to the peer's pending burst, starting or
// extending its debounce timer.
func (e *Engine) queueText(peer Member, text, format string) {
	key := msgKey(peer.IP, peer.Port)

	e.pendMu.Lock()
	defer e.pendMu.Unlock()

	p := e.pending[key]
	if p == nil {
		p = &pendingText{peer: peer, text: text, format: format}
		e.pending[key] = p
		p.timer = time.AfterFunc(coalesceWindow, func() { e.flushPending(key) })
	} else {
		// Keep line breaks sane when the peer also sends them at fragment ends.
		sep := "\n"
		if strings.HasSuffix(p.text, "\n") || strings.HasPrefix(text, "\n") {
			sep = ""
		}
		p.text += sep + text
		if format != "" {
			p.format = format
		}
		p.timer.Reset(coalesceWindow)
	}
	p.deadline = time.Now().Add(coalesceWindow)
}

// flushPending emits the buffered burst for key once it has been quiet for the
// whole window. It re-arms itself when a fragment arrived while it was waiting
// for the lock.
func (e *Engine) flushPending(key string) {
	e.pendMu.Lock()
	p := e.pending[key]
	if p == nil {
		e.pendMu.Unlock()
		return
	}
	if remaining := time.Until(p.deadline); remaining > 0 {
		p.timer.Reset(remaining)
		e.pendMu.Unlock()
		return
	}
	delete(e.pending, key)
	msg := Message{
		Peer:   p.peer,
		Kind:   "text",
		Text:   p.text,
		Format: p.format,
		Time:   time.Now(),
	}
	e.pendMu.Unlock()

	e.emit(msg)
}

// flushPeer emits a pending burst immediately, keeping it ahead of a message
// that cannot be merged (e.g. one carrying attachments).
func (e *Engine) flushPeer(ip string, port int) {
	key := msgKey(ip, port)
	e.pendMu.Lock()
	p := e.pending[key]
	delete(e.pending, key)
	if p != nil {
		p.timer.Stop()
	}
	e.pendMu.Unlock()
	if p == nil {
		return
	}
	e.emit(Message{Peer: p.peer, Kind: "text", Text: p.text, Format: p.format, Time: time.Now()})
}

// flushAllPending emits every buffered burst (called on shutdown).
func (e *Engine) flushAllPending() {
	e.pendMu.Lock()
	pending := e.pending
	e.pending = make(map[string]*pendingText)
	e.pendMu.Unlock()

	for _, p := range pending {
		p.timer.Stop()
		e.emit(Message{Peer: p.peer, Kind: "text", Text: p.text, Format: p.format, Time: time.Now()})
	}
}
