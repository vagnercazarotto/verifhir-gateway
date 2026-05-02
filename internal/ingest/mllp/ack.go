package mllp

import (
	"fmt"
	"strings"
	"time"
)

// ackCode represents an HL7v2 MSA acknowledgement code.
type ackCode string

const (
	ackAccept ackCode = "AA" // Application Accept
	ackError  ackCode = "AE" // Application Error
)

// buildACK constructs an MLLP-framed HL7v2 ACK message for the given
// control ID (MSH-10 of the original message).
func buildACK(controlID string) []byte {
	return buildMSA(controlID, ackAccept, "")
}

// buildNACK constructs an MLLP-framed HL7v2 NACK (AE) message.
func buildNACK(controlID string, reason error) []byte {
	detail := ""
	if reason != nil {
		detail = reason.Error()
	}
	return buildMSA(controlID, ackError, detail)
}

func buildMSA(controlID string, code ackCode, errDetail string) []byte {
	ts := time.Now().UTC().Format("20060102150405")
	ackID := fmt.Sprintf("ACK%s", ts)

	msh := fmt.Sprintf("MSH|^~\\&|VeriFHIR|GATEWAY|||%s||ACK|%s|P|2.5", ts, ackID)
	msa := fmt.Sprintf("MSA|%s|%s", string(code), controlID)

	segments := []string{msh, msa}
	if errDetail != "" {
		// ERR segment — plain text detail in ERR-1 for HL7v2.5 compatibility.
		segments = append(segments, fmt.Sprintf("ERR|%s", sanitize(errDetail)))
	}

	// HL7v2 segment terminator is CR (0x0D).
	msg := []byte(strings.Join(segments, "\r") + "\r")

	frame := make([]byte, 0, len(msg)+3)
	frame = append(frame, vt)
	frame = append(frame, msg...)
	frame = append(frame, fs, cr)
	return frame
}

// extractControlID returns the value of MSH-10 (message control ID) from a
// raw HL7v2 payload. Returns an empty string if the segment cannot be parsed.
func extractControlID(payload []byte) string {
	// MSH is always the first segment. Fields are separated by '|'.
	// MSH-10 is the 10th field (index 9 when split by '|').
	line := string(payload)
	if len(line) < 4 {
		return ""
	}
	// Locate the first segment — handle both CR and CRLF line endings.
	end := strings.IndexAny(line, "\r\n")
	if end > 0 {
		line = line[:end]
	}
	fields := strings.Split(line, "|")
	if len(fields) < 10 {
		return ""
	}
	return fields[9]
}

// sanitize removes characters that would break HL7v2 segment structure.
func sanitize(s string) string {
	r := strings.NewReplacer("|", " ", "^", " ", "~", " ", "\\", " ", "&", " ", "\r", " ", "\n", " ")
	return r.Replace(s)
}
