package mllp

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// ---- helpers ---------------------------------------------------------------

func makeACK(code string) []byte {
	ack := "MSH|^~\\&|TEST|TEST|SENDER|SENDER|20260101120000||ACK^A01|1|P|2.5\rMSA|" + code + "|1\r"
	out := make([]byte, 0, len(ack)+3)
	out = append(out, vt)
	out = append(out, []byte(ack)...)
	out = append(out, fs, cr)
	return out
}

func serveOnce(t *testing.T, response []byte) (addr string, done <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.Read(buf) // consume inbound frame
		conn.Write(response)
	}()
	return ln.Addr().String(), ch
}

func samplePayload(raw string) model.RoutedPayload {
	return model.RoutedPayload{
		Resource: model.FHIRResource{ID: "msg-001"},
		RawHL7:   raw,
	}
}

const sampleHL7 = "MSH|^~\\&|SEND|SEND|RECV|RECV|20260101120000||ADT^A01|1|P|2.5\r" +
	"PID|1||P001^^^MRN||Doe^John\r"

// ---- tests -----------------------------------------------------------------

func TestSendACK(t *testing.T) {
	addr, done := serveOnce(t, makeACK("AA"))
	a := New(Config{Addr: addr, Timeout: 3 * time.Second})
	if err := a.Send(context.Background(), samplePayload(sampleHL7)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done
}

func TestSendNACK_AE(t *testing.T) {
	addr, done := serveOnce(t, makeACK("AE"))
	a := New(Config{Addr: addr, Timeout: 3 * time.Second})
	err := a.Send(context.Background(), samplePayload(sampleHL7))
	if err == nil {
		t.Fatal("expected error for AE NACK")
	}
	if !strings.Contains(err.Error(), "NACK") {
		t.Errorf("error should mention NACK, got: %v", err)
	}
	<-done
}

func TestSendNACK_AR(t *testing.T) {
	addr, done := serveOnce(t, makeACK("AR"))
	a := New(Config{Addr: addr, Timeout: 3 * time.Second})
	err := a.Send(context.Background(), samplePayload(sampleHL7))
	if err == nil {
		t.Fatal("expected error for AR NACK")
	}
	<-done
}

func TestSendEmptyRawHL7(t *testing.T) {
	a := New(Config{Addr: "127.0.0.1:9999", Timeout: time.Second})
	err := a.Send(context.Background(), samplePayload(""))
	if err == nil {
		t.Fatal("expected error when RawHL7 is empty")
	}
	if !strings.Contains(err.Error(), "RawHL7 is empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendDialFailure(t *testing.T) {
	// Port 1 is reserved and will always fail to connect.
	a := New(Config{Addr: "127.0.0.1:1", Timeout: 500 * time.Millisecond})
	err := a.Send(context.Background(), samplePayload(sampleHL7))
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestSendContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	// Point at a non-routable address so the dial races the deadline.
	a := New(Config{Addr: "192.0.2.1:2575", Timeout: 100 * time.Millisecond})
	err := a.Send(ctx, samplePayload(sampleHL7))
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestIsNACK(t *testing.T) {
	cases := []struct {
		ack  string
		want bool
	}{
		{"MSH|...\rMSA|AA|1\r", false},
		{"MSH|...\rMSA|AE|1\r", true},
		{"MSH|...\rMSA|AR|1\r", true},
		{"MSH|...\rMSA|CA|1\r", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isNACK(c.ack); got != c.want {
			t.Errorf("isNACK(%q) = %v, want %v", c.ack, got, c.want)
		}
	}
}
