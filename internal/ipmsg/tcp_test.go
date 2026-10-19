package ipmsg

import (
	"testing"
	"time"
)

func waitMembers(t *testing.T, n int, engines ...*Engine) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, e := range engines {
			if len(e.Members()) < n {
				all = false
				break
			}
		}
		if all {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for i, e := range engines {
		t.Logf("engine %d members: %+v", i, e.Members())
	}
	t.Fatalf("timed out waiting for %d members on all engines", n)
}

func TestTCPPeerDiscoveryAndMessage(t *testing.T) {
	a := New(Config{Name: "alice", Host: "ha", Port: 35011, TCPPort: 36011}, Callbacks{})
	bMsgs := make(chan Message, 4)
	b := New(Config{Name: "bob", Host: "hb", Port: 35012, TCPPort: 36012}, Callbacks{
		OnMessage: func(m Message) { bMsgs <- m },
	})
	if err := a.Start(); err != nil {
		t.Fatalf("a start: %v", err)
	}
	defer a.Stop()
	if err := b.Start(); err != nil {
		t.Fatalf("b start: %v", err)
	}
	defer b.Stop()

	if !a.dialTCPPeerPort("127.0.0.1", 36012) {
		t.Fatal("dial to peer failed")
	}

	waitMembers(t, 1, a, b)

	// A sends a message; it must travel over the TCP peer channel.
	if err := a.SendText("127.0.0.1", 35012, "hello-over-tcp"); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case m := <-bMsgs:
		if m.Text != "hello-over-tcp" {
			t.Fatalf("got text %q", m.Text)
		}
		if m.Peer.Name != "alice" {
			t.Fatalf("got peer %q", m.Peer.Name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bob did not receive the message over TCP")
	}
}
