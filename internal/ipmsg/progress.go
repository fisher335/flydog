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
	if t.State == StateRunning {
		e.progLast[t.ID] = now
	} else {
		// The transfer is finished; drop its throttle entry so the map does
		// not grow for the lifetime of the process.
		delete(e.progLast, t.ID)
	}
	e.progMu.Unlock()

	e.cb.OnTransfer(t)
}
