package mllp

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// sampleHL7 is a minimal valid ADT A01 message.
const sampleHL7 = "MSH|^~\\&|SRC|FAC|DST|FAC|20260501120000||ADT^A01|CTRL001|P|2.5\rPID|1||12345^^^HOSP^MR||DOE^JOHN||19800101|M"

// frameMessage wraps a raw HL7 string in MLLP framing bytes.
func frameMessage(payload string) []byte {
	b := []byte(payload)
	out := make([]byte, 0, len(b)+3)
	out = append(out, vt)
	out = append(out, b...)
	out = append(out, fs, cr)
	return out
}

// startServer starts the MLLP server on a random port and returns the address.
func startServer(t *testing.T, handler Handler) (addr string, cancel context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	srv := New("127.0.0.1:0", handler)

	// We need to bind to port 0 to get a random free port, but Server.ListenAndServe
	// does the Listen internally. Use a temporary listener to find the port first.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind to random port: %v", err)
	}
	addr = ln.Addr().String()
	ln.Close()

	srv.addr = addr
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			// Errors during test teardown are expected.
			_ = err
		}
	}()

	// Wait for the server to start accepting connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return addr, cancel
}

// sendAndReceive sends a framed HL7 message over TCP and returns the raw response.
func sendAndReceive(t *testing.T, addr string, payload []byte) []byte {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp
}

func TestServerReceivesMessageAndSendsACK(t *testing.T) {
	received := make(chan model.HL7Message, 1)

	handler := func(msg model.HL7Message) error {
		received <- msg
		return nil
	}

	addr, cancel := startServer(t, handler)
	defer cancel()

	resp := sendAndReceive(t, addr, frameMessage(sampleHL7))

	// ACK must contain MSA|AA.
	if !bytes.Contains(resp, []byte("MSA|AA")) {
		t.Errorf("expected ACK (MSA|AA) in response, got: %q", resp)
	}

	select {
	case msg := <-received:
		if msg.Payload != sampleHL7 {
			t.Errorf("payload mismatch:\n got  %q\n want %q", msg.Payload, sampleHL7)
		}
		if msg.ID == "" {
			t.Error("expected non-empty message ID")
		}
		if msg.Source == "" {
			t.Error("expected non-empty source address")
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not called within timeout")
	}
}

func TestServerSendsNACKOnHandlerError(t *testing.T) {
	handler := func(msg model.HL7Message) error {
		return fmt.Errorf("pipeline failure")
	}

	addr, cancel := startServer(t, handler)
	defer cancel()

	resp := sendAndReceive(t, addr, frameMessage(sampleHL7))

	if !bytes.Contains(resp, []byte("MSA|AE")) {
		t.Errorf("expected NACK (MSA|AE) in response, got: %q", resp)
	}
}

func TestACKContainsOriginalControlID(t *testing.T) {
	handler := func(msg model.HL7Message) error { return nil }

	addr, cancel := startServer(t, handler)
	defer cancel()

	resp := sendAndReceive(t, addr, frameMessage(sampleHL7))

	// Control ID from sampleHL7 is CTRL001.
	if !bytes.Contains(resp, []byte("CTRL001")) {
		t.Errorf("expected control ID CTRL001 in ACK, got: %q", resp)
	}
}

func TestExtractControlID(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "standard message",
			payload: sampleHL7,
			want:    "CTRL001",
		},
		{
			name:    "empty payload",
			payload: "",
			want:    "",
		},
		{
			name:    "too few fields",
			payload: "MSH|^~\\&|SRC",
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractControlID([]byte(tc.payload))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadWriteFrame(t *testing.T) {
	original := []byte(sampleHL7)

	var buf strings.Builder
	if err := writeFrame(&buf, original); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}

	r := strings.NewReader(buf.String())
	got, err := readMessage(r)
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}

	if !bytes.Equal(got, original) {
		t.Errorf("round-trip mismatch:\n got  %q\n want %q", got, original)
	}
}

func TestServerHandlesMultipleMessagesOnSameConnection(t *testing.T) {
	// A single TCP connection must be able to carry N messages sequentially.
	const n = 5
	count := make(chan struct{}, n)

	handler := func(msg model.HL7Message) error {
		count <- struct{}{}
		return nil
	}

	addr, cancel := startServer(t, handler)
	defer cancel()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	for i := 0; i < n; i++ {
		if _, err := conn.Write(frameMessage(sampleHL7)); err != nil {
			t.Fatalf("write message %d: %v", i, err)
		}
		if _, err := readMessage(conn); err != nil {
			t.Fatalf("read ACK %d: %v", i, err)
		}
	}

	if len(count) != n {
		t.Errorf("handler called %d times, want %d", len(count), n)
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	handler := func(msg model.HL7Message) error { return nil }

	addr, cancel := startServer(t, handler)

	// Cancel immediately — server should stop accepting new connections.
	cancel()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("expected connection to be refused after shutdown, but it succeeded")
	}
}
