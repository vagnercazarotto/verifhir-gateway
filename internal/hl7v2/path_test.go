package hl7v2

import "testing"

func TestParsePathVariants(t *testing.T) {
	cases := []struct {
		in   string
		want Path
	}{
		{"PID-3", Path{Segment: "PID", Field: 3, Repetition: 1, Component: 1, Subcomponent: 1}},
		{"PID-5.1", Path{Segment: "PID", Field: 5, Repetition: 1, Component: 1, Subcomponent: 1}},
		{"PID-3[2]", Path{Segment: "PID", Field: 3, Repetition: 2, Component: 1, Subcomponent: 1}},
		{"PID-3[2].4", Path{Segment: "PID", Field: 3, Repetition: 2, Component: 4, Subcomponent: 1}},
		{"PID-3[2].4.1", Path{Segment: "PID", Field: 3, Repetition: 2, Component: 4, Subcomponent: 1}},
		{"PV1-19", Path{Segment: "PV1", Field: 19, Repetition: 1, Component: 1, Subcomponent: 1}},
		{"OBX-5.2.3", Path{Segment: "OBX", Field: 5, Repetition: 1, Component: 2, Subcomponent: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParsePath(tc.in)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestParsePathRejectsInvalid(t *testing.T) {
	bad := []string{"", "PID", "PID-", "PID-0", "pid-3", "PID3", "PID-3[0]", "PID-3.0", "TOOLONG-3"}
	for _, in := range bad {
		t.Run(in, func(t *testing.T) {
			if _, err := ParsePath(in); err == nil {
				t.Fatalf("expected error for %q", in)
			}
		})
	}
}

func TestMessageGetReturnsExpectedValues(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := map[string]string{
		"PID-1":      "1",
		"PID-3":      "PATID1234",       // first repetition, first component
		"PID-3.4":    "HOSPITAL",        // first repetition, fourth component
		"PID-3.5":    "MR",              // first repetition, fifth component
		"PID-3[2]":   "ALT9999",         // second repetition, first component
		"PID-3[2].4": "OTHER",           // second repetition, fourth component
		"PID-5.1":    "DOE",
		"PID-5.2":    "JOHN",
		"PID-5.3":    "A",
		"PID-7":      "19800101",
		"PID-8":      "M",
		"PID-11.3":   "Boston",
		"PID-11.4":   "MA",
		"PV1-2":      "I",
		"PV1-3.1":   "ICU",
		"PV1-3.2":   "101",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			if got := msg.Get(path); got != want {
				t.Fatalf("Get(%q): got %q want %q", path, got, want)
			}
		})
	}
}

func TestMessageGetMissingReturnsEmpty(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	missing := []string{"PID-99", "PID-3[9]", "PID-5.99", "ZZZ-1", "PID-3.1.99"}
	for _, p := range missing {
		t.Run(p, func(t *testing.T) {
			if got := msg.Get(p); got != "" {
				t.Fatalf("Get(%q): got %q want empty", p, got)
			}
			if msg.Has(p) {
				t.Fatalf("Has(%q): expected false", p)
			}
		})
	}
}

func TestMessageHasDistinguishesEmptyFromMissing(t *testing.T) {
	msg, err := Parse(sampleADT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// PID-2 is empty in the sample.
	if msg.Has("PID-2") {
		t.Errorf("Has(PID-2): expected false for empty field")
	}
	if v, ok := msg.Lookup("PID-2"); !ok || v != "" {
		t.Errorf("Lookup(PID-2): got (%q,%v) want ('',true)", v, ok)
	}
	if v, ok := msg.Lookup("PID-99"); ok || v != "" {
		t.Errorf("Lookup(PID-99): got (%q,%v) want ('',false)", v, ok)
	}
}

func TestAllSegments(t *testing.T) {
	raw := "MSH|^~\\&|SRC|FAC|DST|FAC|202605011200||ORU^R01|MSG|P|2.5\r" +
		"PID|1||12345\r" +
		"OBX|1|NM|1234-5^Glucose|1|110|mg/dL\r" +
		"OBX|2|NM|789-8^Sodium|1|140|mmol/L\r" +
		"OBX|3|NM|2345-7^Hemoglobin|1|14.5|g/dL"
	msg, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(msg.AllSegments("OBX")), 3; got != want {
		t.Fatalf("AllSegments(OBX): got %d want %d", got, want)
	}
}
