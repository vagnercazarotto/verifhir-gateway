// Package mllp provides an MLLP destination adapter that delivers the original
// raw HL7v2 message to a remote system over a TCP connection using MLLP framing.
//
// The adapter opens a new TCP connection per Send call so it is safe for
// concurrent use and requires no persistent connection management. For
// high-throughput scenarios a connection pool can be added later without
// changing the Sender interface.
//
// MLLP framing:
//
//	VT (0x0B) <HL7 payload bytes> FS (0x1C) CR (0x0D)
package mllp

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

const (
	vt             = byte(0x0B)
	fs             = byte(0x1C)
	cr             = byte(0x0D)
	defaultTimeout = 10 * time.Second
	ackBufSize     = 4096
)

// Config holds the runtime settings for the MLLP destination adapter.
type Config struct {
	// Addr is the host:port of the remote MLLP listener, e.g. "hl7.example.com:2575".
	Addr string

	// Timeout overrides the default 10-second dial+write+ack deadline.
	// Zero means use the default.
	Timeout time.Duration
}

// Adapter sends raw HL7v2 messages to a remote MLLP listener.
type Adapter struct {
	cfg     Config
	timeout time.Duration
	// dial is injectable for testing; defaults to net.DialTimeout.
	dial func(network, addr string, timeout time.Duration) (net.Conn, error)
}

// New creates an Adapter from cfg.
func New(cfg Config) *Adapter {
	t := cfg.Timeout
	if t <= 0 {
		t = defaultTimeout
	}
	return &Adapter{
		cfg:     cfg,
		timeout: t,
		dial:    net.DialTimeout,
	}
}

// Send wraps payload.RawHL7 in MLLP framing and delivers it to the configured
// address, then reads and validates the ACK response.
// It returns an error if RawHL7 is empty, the connection fails, or the remote
// side responds with a NACK (MSA-1 = "AE" or "AR").
func (a *Adapter) Send(ctx context.Context, payload model.RoutedPayload) error {
	raw := payload.RawHL7
	if raw == "" {
		return fmt.Errorf("mllp adapter: RawHL7 is empty for msg %s", payload.Resource.ID)
	}

	// Respect context deadline.
	deadline := time.Now().Add(a.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	conn, err := a.dial("tcp", a.cfg.Addr, time.Until(deadline))
	if err != nil {
		return fmt.Errorf("mllp adapter: dial %s: %w", a.cfg.Addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("mllp adapter: set deadline: %w", err)
	}

	// Write MLLP-framed message.
	frame := make([]byte, 0, 3+len(raw)+2)
	frame = append(frame, vt)
	frame = append(frame, []byte(raw)...)
	frame = append(frame, fs, cr)

	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("mllp adapter: write: %w", err)
	}

	// Read ACK response.
	ack, err := readACK(conn)
	if err != nil {
		return fmt.Errorf("mllp adapter: read ack: %w", err)
	}
	if isNACK(ack) {
		return fmt.Errorf("mllp adapter: received NACK from %s: %s", a.cfg.Addr, ack)
	}
	return nil
}

// readACK reads an MLLP-framed response from conn and returns the payload.
func readACK(conn net.Conn) (string, error) {
	buf := make([]byte, 1)
	// Advance to VT.
	for {
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		if buf[0] == vt {
			break
		}
	}
	var payload []byte
	for len(payload) < ackBufSize {
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		if buf[0] == fs {
			break
		}
		payload = append(payload, buf[0])
	}
	// Consume trailing CR (best-effort; ignore error).
	_, _ = io.ReadFull(conn, buf)
	return string(payload), nil
}

// isNACK returns true when the ACK message contains MSA-1 = "AE" or "AR".
// This is a minimal check on the MSA segment field 1.
func isNACK(ack string) bool {
	// Look for MSA segment and read field 1 (after first |).
	const prefix = "MSA|"
	idx := 0
	for i := 0; i+len(prefix) <= len(ack); i++ {
		if ack[i:i+len(prefix)] == prefix {
			idx = i + len(prefix)
			// Field 1 is the next 2 characters before the next |.
			end := idx
			for end < len(ack) && ack[end] != '|' && ack[end] != '\r' && ack[end] != '\n' {
				end++
			}
			code := ack[idx:end]
			return code == "AE" || code == "AR"
		}
	}
	return false
}
