package whisper

import (
	"bytes"
	"testing"
	"time"
)

// TestSendReceive spins up two Whispers and verifies a message sent from one
// is received by the other.
func TestSendReceive(t *testing.T) {
	// Create two whisper instances
	w1, err := NewWhisper()
	if err != nil {
		t.Fatalf("NewWhisper w1 failed: %v", err)
	}
	defer w1.Close()

	w2, err := NewWhisper()
	if err != nil {
		w1.Close()
		t.Fatalf("NewWhisper w2 failed: %v", err)
	}
	defer w2.Close()

	// Assign deterministic IDs
	w1.SetID(0xA1A1A1A1)
	w2.SetID(0xB2B2B2B2)

	payload := []byte("hello-susurrus")
	cmd := uint16(0x1234)

	// start receiver goroutine
	recvCh := make(chan Susurrus, 1)
	go func() {
		for s := range w2.RecvChan {
			// deliver first matching message
			if s.Cmd == cmd && s.Dest == w2.ID && s.Source == w1.ID {
				recvCh <- s
				return
			}
		}
	}()

	// send from w1 to w2
	if err := w1.Send(w2.ID, cmd, payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// wait for receive
	select {
	case s := <-recvCh:
		if s.Dest != w2.ID {
			t.Fatalf("unexpected dest: 0x%08X", s.Dest)
		}
		if s.Source != w1.ID {
			t.Fatalf("unexpected source: 0x%08X", s.Source)
		}
		if s.Cmd != cmd {
			t.Fatalf("unexpected cmd: 0x%04X", s.Cmd)
		}
		if !bytes.Equal(s.Data, payload) {
			t.Fatalf("payload mismatch: got %v", s.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for susurrus")
	}
}
