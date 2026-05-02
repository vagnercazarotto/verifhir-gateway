package mllp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

const (
	// readTimeout is the maximum time allowed to read a complete MLLP frame
	// from a single connection. Protects against slow or stalled senders.
	readTimeout = 30 * time.Second
)

// Handler is the function called for each successfully received HL7v2 message.
// Returning a non-nil error causes the server to send a NACK back to the sender.
type Handler func(msg model.HL7Message) error

// Server is a concurrent MLLP TCP listener. Each accepted connection is
// handled in its own goroutine. The server sends an ACK on success or a NACK
// on handler error, then waits for the next message on the same connection.
type Server struct {
	addr    string
	handler Handler
}

// New creates an MLLP Server that will listen on addr and call handler for
// every received message.
func New(addr string, handler Handler) *Server {
	return &Server{addr: addr, handler: handler}
}

// ListenAndServe starts the TCP listener and blocks until ctx is cancelled.
// Active connections are closed after the context is done.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("mllp: listen %s: %w", s.addr, err)
	}

	// Close the listener when the context is cancelled so Accept() unblocks.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	log.Printf("[mllp] listening on %s", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // graceful shutdown
			default:
				return fmt.Errorf("mllp: accept: %w", err)
			}
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	remote := conn.RemoteAddr().String()
	log.Printf("[mllp] connection accepted from %s", remote)

	for {
		// Check context before blocking on the next read.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("[mllp] %s: set deadline: %v", remote, err)
			return
		}

		payload, err := readMessage(conn)
		if err != nil {
			// EOF means the sender closed the connection — not an error.
			if isClosedErr(err) {
				log.Printf("[mllp] %s: connection closed by sender", remote)
			} else {
				log.Printf("[mllp] %s: read error: %v", remote, err)
			}
			return
		}

		controlID := extractControlID(payload)
		msgID := newMessageID()

		msg := model.HL7Message{
			ID:      msgID,
			Source:  remote,
			Payload: string(payload),
		}

		handlerErr := s.handler(msg)

		// Reset deadline for writing the acknowledgement.
		_ = conn.SetWriteDeadline(time.Now().Add(readTimeout))

		if handlerErr != nil {
			log.Printf("[mllp] %s msg=%s NACK: %v", remote, msgID, handlerErr)
			_, _ = conn.Write(buildNACK(controlID, handlerErr))
		} else {
			log.Printf("[mllp] %s msg=%s ACK", remote, msgID)
			_, _ = conn.Write(buildACK(controlID))
		}
	}
}

// newMessageID generates a cryptographically random hex message ID.
func newMessageID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use current time in nanoseconds (should never happen).
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// isClosedErr returns true for errors that indicate a cleanly closed connection.
func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return contains(s, "EOF") ||
		contains(s, "use of closed network connection") ||
		contains(s, "connection reset by peer")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && searchString(s, sub))
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
