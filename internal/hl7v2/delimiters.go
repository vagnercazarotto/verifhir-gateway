// Package hl7v2 parses HL7 v2.x messages into a typed structure with
// field-level access via HL7-style dotted paths (e.g. "PID-3.1[2].4").
//
// The parser honors the delimiter characters declared in MSH-1 and MSH-2
// rather than assuming the conventional pipe-and-hat defaults.
package hl7v2

import "errors"

// Delimiters captures the field and encoding characters declared in MSH-1
// and MSH-2. Every HL7 v2 message advertises its own delimiters and parsers
// must read them from the header rather than hardcoding the defaults.
type Delimiters struct {
	Field        rune // MSH-1
	Component    rune // MSH-2 char 1
	Repetition   rune // MSH-2 char 2
	Escape       rune // MSH-2 char 3
	Subcomponent rune // MSH-2 char 4
}

// DefaultDelimiters returns the conventional HL7 v2 delimiter set: |^~\&
func DefaultDelimiters() Delimiters {
	return Delimiters{
		Field:        '|',
		Component:    '^',
		Repetition:   '~',
		Escape:       '\\',
		Subcomponent: '&',
	}
}

// extractDelimiters reads the leading MSH segment line and returns the
// delimiter set declared by the sender.
func extractDelimiters(mshLine string) (Delimiters, error) {
	if len(mshLine) < 3 || mshLine[:3] != "MSH" {
		return Delimiters{}, errors.New("first segment must be MSH")
	}
	if len(mshLine) < 8 {
		return Delimiters{}, errors.New("MSH header too short to declare delimiters")
	}
	return Delimiters{
		Field:        rune(mshLine[3]),
		Component:    rune(mshLine[4]),
		Repetition:   rune(mshLine[5]),
		Escape:       rune(mshLine[6]),
		Subcomponent: rune(mshLine[7]),
	}, nil
}
