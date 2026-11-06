package ipmsg

import (
	"testing"
	"time"
)

// A multi-line message arrives as one SENDMSG packet per line from 飞秋/飞鸽;
// the fragments must be shown as a single chat message.
func TestTextBurstCoalesced(t *testing.T) {
	port := freePort(t)
	msgs := make(chan Message, 16)
	b := New(Config{Name: "recv", Host: "hb", Port: port, TCPPort: freePort(t)}, Callbacks{
		OnMessage: func(m Message) { msgs <- m },
	})
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()

	a := New(Config{Name: "send", Host: "ha", Port: freePort(t), TCPPort: freePort(t)}, Callbacks{})
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()

	for _, line := range []string{"第一行", "第二行", "third line"} {
		if err := a.SendText("127.0.0.1", port, line); err != nil {
			t.Fatalf("send: %v", err)
		}
	}

	select {
	case m := <-msgs:
		want := "第一行\n第二行\nthird line"
		if m.Text != want {
			t.Fatalf("text = %q, want %q", m.Text, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no message emitted")
	}

	select {
	case m := <-msgs:
		t.Fatalf("burst produced a second message: %q", m.Text)
	case <-time.After(coalesceWindow + 500*time.Millisecond):
	}
}

// Messages further apart than the window stay separate.
func TestTextMessagesNotCoalescedWhenApart(t *testing.T) {
	port := freePort(t)
	msgs := make(chan Message, 16)
	b := New(Config{Name: "recv2", Host: "hb", Port: port, TCPPort: freePort(t)}, Callbacks{
		OnMessage: func(m Message) { msgs <- m },
	})
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()

	a := New(Config{Name: "send2", Host: "ha", Port: freePort(t), TCPPort: freePort(t)}, Callbacks{})
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()

	if err := a.SendText("127.0.0.1", port, "first"); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-msgs:
		if m.Text != "first" {
			t.Fatalf("first = %q", m.Text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first message not emitted")
	}

	if err := a.SendText("127.0.0.1", port, "second"); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-msgs:
		if m.Text != "second" {
			t.Fatalf("second = %q, want %q", m.Text, "second")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second message not emitted")
	}
}

// A single multiline text sent in one packet survives unchanged.
func TestSinglePacketMultilineText(t *testing.T) {
	port := freePort(t)
	msgs := make(chan Message, 4)
	b := New(Config{Name: "recv3", Host: "hb", Port: port, TCPPort: freePort(t)}, Callbacks{
		OnMessage: func(m Message) { msgs <- m },
	})
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()

	a := New(Config{Name: "send3", Host: "ha", Port: freePort(t), TCPPort: freePort(t)}, Callbacks{})
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()

	const text = "line1\nline2\r\nline3"
	if err := a.SendText("127.0.0.1", port, text); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-msgs:
		if m.Text != text {
			t.Fatalf("text = %q, want %q", m.Text, text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("message not emitted")
	}
}
