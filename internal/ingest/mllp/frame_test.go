package mllp

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestReadMessageHappyPath(t *testing.T) {
	payload := []byte("MSH|^~\\&|SRC|FAC|DST|FAC|20260501||ADT^A01|1|P|2.5")
	r := bytes.NewReader(frameMessage(string(payload)))

	got, err := readMessage(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestReadMessageSkipsGarbageBeforeVT(t *testing.T) {
	// Some senders prepend keep-alive bytes or whitespace before the VT.
	payload := []byte("MSH|^~\\&|SRC|FAC|DST|FAC|20260501||ADT^A01|2|P|2.5")
	frame := frameMessage(string(payload))

	// Prefix with garbage bytes that are not VT.
	garbage := []byte{0x00, 0x20, 0x0A, 0x0D}
	data := append(garbage, frame...)

	got, err := readMessage(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestReadMessageEmptyPayload(t *testing.T) {
	// VT immediately followed by FS CR — empty payload must be rejected.
	data := []byte{vt, fs, cr}
	_, err := readMessage(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

func TestReadMessageWrongByteAfterFS(t *testing.T) {
	// A frame that has a byte other than CR (0x0D) after FS must be rejected.
	payload := []byte("MSH|^~\\&|SRC|FAC|DST|FAC|20260501||ADT^A01|3|P|2.5")
	data := make([]byte, 0, len(payload)+3)
	data = append(data, vt)
	data = append(data, payload...)
	data = append(data, fs, 0x0A) // 0x0A = LF, not CR

	_, err := readMessage(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for wrong byte after FS, got nil")
	}
}

func TestReadMessageEOFWaitingForVT(t *testing.T) {
	_, err := readMessage(bytes.NewReader([]byte{}))
	if err == nil {
		t.Fatal("expected error on empty reader, got nil")
	}
}

func TestReadMessageEOFInsidePayload(t *testing.T) {
	// VT with payload but no FS — connection drops mid-message.
	data := []byte{vt, 'M', 'S', 'H'}
	_, err := readMessage(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error on truncated frame, got nil")
	}
}

func TestReadMessageEOFAfterFS(t *testing.T) {
	// Payload + FS but no trailing CR.
	payload := []byte("MSH|^~\\&|SRC|FAC|DST|FAC|20260501||ADT^A01|4|P|2.5")
	data := make([]byte, 0, len(payload)+2)
	data = append(data, vt)
	data = append(data, payload...)
	data = append(data, fs) // no CR follows

	_, err := readMessage(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error when CR is missing after FS, got nil")
	}
}

func TestWriteFrameProducesValidFrame(t *testing.T) {
	msg := []byte("MSH|^~\\&|SRC|FAC|DST|FAC|20260501||ADT^A01|5|P|2.5")
	var buf strings.Builder

	if err := writeFrame(&buf, msg); err != nil {
		t.Fatalf("writeFrame returned error: %v", err)
	}

	b := []byte(buf.String())

	if b[0] != vt {
		t.Errorf("first byte: got 0x%02X, want VT (0x0B)", b[0])
	}
	if b[len(b)-2] != fs {
		t.Errorf("second-to-last byte: got 0x%02X, want FS (0x1C)", b[len(b)-2])
	}
	if b[len(b)-1] != cr {
		t.Errorf("last byte: got 0x%02X, want CR (0x0D)", b[len(b)-1])
	}

	inner := b[1 : len(b)-2]
	if !bytes.Equal(inner, msg) {
		t.Errorf("inner payload mismatch:\n got  %q\n want %q", inner, msg)
	}
}

func TestWriteFrameErrorPropagated(t *testing.T) {
	err := writeFrame(errWriter{}, []byte("MSH|test"))
	if err == nil {
		t.Fatal("expected error from failing writer, got nil")
	}
}

// errWriter is an io.Writer that always returns an error.
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}
