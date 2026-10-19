package ipmsg

import "time"

// emitTransfer reports transfer progress to the callback, throttling frequent
// running updates so the frontend isn't flooded with events.
func (e *Engine) emitTransfer(t Transfer) {
	if e.cb.OnTransfer == nil {
		return
	}
	now := time.Now()
	e.progMu.Lock()
	last, ok := e.progLast[t.ID]
	if t.State == StateRunning && ok && now.Sub(last) < 200*time.Millisecond && t.Done != t.Size {
		e.progMu.Unlock()
		return
	}
	e.progLast[t.ID] = now
	e.progMu.Unlock()

	e.cb.OnTransfer(t)
}
