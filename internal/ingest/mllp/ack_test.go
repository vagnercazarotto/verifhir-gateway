package mllp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestBuildACKContainsMSAAA(t *testing.T) {
	frame := buildACK("CTRL-001")
	// Strip MLLP framing to inspect the HL7 payload.
	payload := frame[1 : len(frame)-2]
	if !bytes.Contains(payload, []byte("MSA|AA|CTRL-001")) {
		t.Errorf("ACK payload missing MSA|AA|CTRL-001: %q", payload)
	}
}

func TestBuildACKFramingBytes(t *testing.T) {
	frame := buildACK("CTRL-002")
	if frame[0] != vt {
		t.Errorf("first byte: got 0x%02X, want VT (0x0B)", frame[0])
	}
	if frame[len(frame)-2] != fs {
		t.Errorf("second-to-last byte: got 0x%02X, want FS (0x1C)", frame[len(frame)-2])
	}
	if frame[len(frame)-1] != cr {
		t.Errorf("last byte: got 0x%02X, want CR (0x0D)", frame[len(frame)-1])
	}
}

func TestBuildNACKContainsMSAAE(t *testing.T) {
	frame := buildNACK("CTRL-003", fmt.Errorf("parse failed"))
	payload := frame[1 : len(frame)-2]
	if !bytes.Contains(payload, []byte("MSA|AE|CTRL-003")) {
		t.Errorf("NACK payload missing MSA|AE|CTRL-003: %q", payload)
	}
}

func TestBuildNACKContainsERRSegment(t *testing.T) {
	frame := buildNACK("CTRL-004", fmt.Errorf("missing PID segment"))
	payload := frame[1 : len(frame)-2]
	if !bytes.Contains(payload, []byte("ERR|")) {
		t.Errorf("NACK payload missing ERR segment: %q", payload)
	}
	if !bytes.Contains(payload, []byte("missing PID segment")) {
		t.Errorf("NACK ERR segment missing error detail: %q", payload)
	}
}

func TestBuildNACKWithNilReason(t *testing.T) {
	// nil error must not panic and must still produce a valid NACK frame.
	frame := buildNACK("CTRL-005", nil)
	if len(frame) == 0 {
		t.Fatal("expected non-empty NACK frame for nil reason")
	}
	payload := frame[1 : len(frame)-2]
	if !bytes.Contains(payload, []byte("MSA|AE|CTRL-005")) {
		t.Errorf("NACK payload missing MSA|AE|CTRL-005: %q", payload)
	}
	// No ERR segment should be added when reason is nil.
	if bytes.Contains(payload, []byte("ERR|")) {
		t.Errorf("unexpected ERR segment for nil reason: %q", payload)
	}
}

func TestBuildACKIsReadableAsMLLP(t *testing.T) {
	// The ACK frame must be parseable by readMessage — ensures ACK framing
	// is byte-compatible with what a real MLLP client would receive.
	frame := buildACK("CTRL-006")
	payload, err := readMessage(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("ACK frame is not a valid MLLP message: %v", err)
	}
	if !bytes.Contains(payload, []byte("MSA|AA")) {
		t.Errorf("parsed ACK payload missing MSA|AA: %q", payload)
	}
}

func TestSanitizeRemovesHL7SpecialChars(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"pipe|char", "pipe char"},
		{"caret^char", "caret char"},
		{"tilde~char", "tilde char"},
		{"backslash\\char", "backslash char"},
		{"ampersand&char", "ampersand char"},
		{"line\rbreak", "line break"},
		{"newline\nchar", "newline char"},
		{"mix|pipe^caret~tilde\\back&amp", "mix pipe caret tilde back amp"},
	}

	for _, tc := range cases {
		got := sanitize(tc.input)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractControlIDWithCRLF(t *testing.T) {
	// Some senders use CRLF as segment delimiter — extractControlID must handle it.
	msg := "MSH|^~\\&|SRC|FAC|DST|FAC|20260501120000||ADT^A01|CTRL-CRLF|P|2.5\r\nPID|1||12345"
	got := extractControlID([]byte(msg))
	if got != "CTRL-CRLF" {
		t.Errorf("got %q, want %q", got, "CTRL-CRLF")
	}
}

func TestExtractControlIDMultiSegment(t *testing.T) {
	msg := "MSH|^~\\&|SRC|FAC|DST|FAC|20260501120000||ADT^A01|CTRL-MULTI|P|2.5\rPID|1||12345\rPV1|1|I"
	got := extractControlID([]byte(msg))
	if got != "CTRL-MULTI" {
		t.Errorf("got %q, want %q", got, "CTRL-MULTI")
	}
}
