package hl7v2

import (
	"strings"
	"testing"
)

const sampleADT = "MSH|^~\\&|SRC|FAC|DST|FAC|202605011200||ADT^A01|MSG001|P|2.5\r" +
	"EVN|A01|202605011200\r" +
	"PID|1||PATID1234^^^HOSPITAL^MR~ALT9999^^^OTHER^MR||DOE^JOHN^A||19800101|M|||123 Main St^^Boston^MA^02118\r" +
	"PV1|1|I|ICU^101^1"

func TestParseEmptyPayloadFails(t *testing.T) {
	if _, err := Parse("   "); err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestParseRequiresMSHFirst(t *testing.T) {
	if _, err := Parse("PID|1||12345"); err == nil {
		t.Fatal("expected error when first segment is not MSH")
	}
}

func TestParseExtractsDefaultDelimiters(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := DefaultDelimiters()
	if msg.Delimiters != want {
		t.Fatalf("delimiters: got %+v want %+v", msg.Delimiters, want)
	}
}

func TestParseHonorsCustomDelimiters(t *testing.T) {
	raw := "MSH#@%/$#SRC#FAC#DST#FAC#202605011200##ADT@A01#MSG001#P#2.5\r" +
		"PID#1##PATID1234@@@HOSPITAL@MR"
	msg, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.Delimiters.Field != '#' || msg.Delimiters.Component != '@' {
		t.Fatalf("custom delimiters not honored: %+v", msg.Delimiters)
	}
	if got := msg.Get("PID-3.4"); got != "HOSPITAL" {
		t.Fatalf("PID-3.4: got %q want HOSPITAL", got)
	}
}

func TestParseSegmentsCount(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(msg.Segments), 4; got != want {
		t.Fatalf("segments: got %d want %d", got, want)
	}
}

func TestParseAcceptsLFAndCRLF(t *testing.T) {
	cases := map[string]string{
		"LF":   strings.ReplaceAll(sampleADT, "\r", "\n"),
		"CRLF": strings.ReplaceAll(sampleADT, "\r", "\r\n"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			msg, err := Parse(raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got, want := len(msg.Segments), 4; got != want {
				t.Fatalf("segments: got %d want %d", got, want)
			}
		})
	}
}

func TestParseTrailingDelimiterIgnored(t *testing.T) {
	msg, err := Parse(sampleADT + "\r")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(msg.Segments), 4; got != want {
		t.Fatalf("segments: got %d want %d", got, want)
	}
}

func TestMSHFieldsPreserveLiterals(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := msg.Get("MSH-1"); got != "|" {
		t.Errorf("MSH-1: got %q want |", got)
	}
	if got := msg.Get("MSH-2"); got != "^~\\&" {
		t.Errorf("MSH-2: got %q want ^~\\&", got)
	}
	if got := msg.Get("MSH-3"); got != "SRC" {
		t.Errorf("MSH-3: got %q want SRC", got)
	}
	if got := msg.Get("MSH-9.1"); got != "ADT" {
		t.Errorf("MSH-9.1: got %q want ADT", got)
	}
	if got := msg.Get("MSH-9.2"); got != "A01" {
		t.Errorf("MSH-9.2: got %q want A01", got)
	}
}

func TestRoundTripParseSerializeParse(t *testing.T) {
	first, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	serialized := first.String()
	second, err := Parse(serialized)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("round-trip mismatch:\n  first:  %q\n  second: %q", first.String(), second.String())
	}
}

func TestParseShortSegmentRejected(t *testing.T) {
	raw := "MSH|^~\\&|SRC|FAC|DST|FAC|202605011200||ADT^A01|MSG001|P|2.5\rXY"
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected error for too-short segment")
	}
}

func TestParseShortMSHRejected(t *testing.T) {
	if _, err := Parse("MSH|^~"); err == nil {
		t.Fatal("expected error for truncated MSH header")
	}
}
