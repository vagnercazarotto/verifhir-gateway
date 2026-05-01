package parser

import "testing"

func TestParseSuccess(t *testing.T) {
	raw := "MSH|^~\\&|SRC|FAC|DST|FAC|202605011200||ADT^A01|1|P|2.5\rPID|1||12345"
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(parsed.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(parsed.Segments))
	}
}

func TestParseEmptyPayload(t *testing.T) {
	_, err := Parse("   ")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}
