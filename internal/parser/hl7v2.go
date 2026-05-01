package parser

import "github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"

// ParsedHL7 is an alias for hl7v2.Message kept here so older call sites keep
// compiling. Prefer hl7v2.Message in new code.
type ParsedHL7 = hl7v2.Message

// Parse decomposes a raw HL7 v2 payload into a typed Message with field-level
// access. See package hl7v2 for path syntax (e.g. "PID-3.1[2].4").
func Parse(raw string) (*ParsedHL7, error) {
	return hl7v2.Parse(raw)
}
