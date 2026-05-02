// Package mllp implements the Minimal Lower Layer Protocol (MLLP) transport
// for HL7v2 messages over TCP.
//
// MLLP wraps each HL7v2 message with three framing bytes:
//
//	VT (0x0B)  — start of block
//	<HL7 message bytes>
//	FS (0x1C)  — end of block
//	CR (0x0D)  — end of message
//
// Reference: https://www.hl7.org/documentcenter/public/wg/inm/mllp_transport_specification.pdf
package mllp

import (
	"fmt"
	"io"
)

const (
	vt = byte(0x0B) // start of block
	fs = byte(0x1C) // end of block
	cr = byte(0x0D) // carriage return
)

// readMessage reads one MLLP-framed message from r, strips the framing bytes,
// and returns the raw HL7v2 payload. It blocks until a complete frame arrives
// or an error occurs.
func readMessage(r io.Reader) ([]byte, error) {
	buf := make([]byte, 1)

	// Advance to start-of-block (VT). Bytes before VT are discarded — some
	// senders prepend whitespace or keep-alive bytes.
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("mllp: waiting for VT: %w", err)
		}
		if buf[0] == vt {
			break
		}
	}

	// Collect payload bytes until end-of-block (FS).
	var payload []byte
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("mllp: reading payload: %w", err)
		}
		if buf[0] == fs {
			break
		}
		payload = append(payload, buf[0])
	}

	// Consume the mandatory trailing CR after FS.
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("mllp: reading trailing CR: %w", err)
	}
	if buf[0] != cr {
		return nil, fmt.Errorf("mllp: expected CR (0x0D) after FS, got 0x%02X", buf[0])
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("mllp: received empty payload")
	}

	return payload, nil
}

// writeFrame wraps msg in MLLP framing and writes the full frame to w.
func writeFrame(w io.Writer, msg []byte) error {
	frame := make([]byte, 0, len(msg)+3)
	frame = append(frame, vt)
	frame = append(frame, msg...)
	frame = append(frame, fs, cr)

	_, err := w.Write(frame)
	return err
}
