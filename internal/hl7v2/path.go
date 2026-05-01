package hl7v2

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// pathRegex matches HL7-style dotted path notation:
//
//	SEGMENT-FIELD                              -> e.g. PID-3
//	SEGMENT-FIELD[REP]                         -> e.g. PID-3[2]
//	SEGMENT-FIELD.COMPONENT                    -> e.g. PID-5.1
//	SEGMENT-FIELD[REP].COMPONENT               -> e.g. PID-3[2].4
//	SEGMENT-FIELD[REP].COMPONENT.SUBCOMPONENT  -> e.g. PID-3[2].4.1
//
// Segment names are three uppercase characters where the second and third
// may be alphanumeric (e.g. PV1, AL1, OBX). All numeric indices are 1-based,
// matching HL7 spec convention.
var pathRegex = regexp.MustCompile(`^([A-Z][A-Z0-9]{2})-(\d+)(?:\[(\d+)\])?(?:\.(\d+))?(?:\.(\d+))?$`)

// Path is a parsed HL7 location reference. All indices are 1-based.
type Path struct {
	Segment      string
	Field        int
	Repetition   int // defaults to 1 when not specified
	Component    int // defaults to 1 when not specified
	Subcomponent int // defaults to 1 when not specified
}

// ParsePath parses HL7-style dotted path notation. Returns an error if the
// path does not match the supported syntax.
func ParsePath(s string) (Path, error) {
	m := pathRegex.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Path{}, fmt.Errorf("invalid HL7 path: %q", s)
	}
	p := Path{
		Segment:      m[1],
		Repetition:   1,
		Component:    1,
		Subcomponent: 1,
	}
	p.Field, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		p.Repetition, _ = strconv.Atoi(m[3])
	}
	if m[4] != "" {
		p.Component, _ = strconv.Atoi(m[4])
	}
	if m[5] != "" {
		p.Subcomponent, _ = strconv.Atoi(m[5])
	}
	if p.Field < 1 || p.Repetition < 1 || p.Component < 1 || p.Subcomponent < 1 {
		return Path{}, fmt.Errorf("HL7 path indices must be >= 1: %q", s)
	}
	return p, nil
}

// Get returns the value at the given HL7 path, or an empty string if any
// part of the path is missing.
func (m *Message) Get(path string) string {
	v, _ := m.Lookup(path)
	return v
}

// Has reports whether the given HL7 path resolves to a non-empty value.
func (m *Message) Has(path string) bool {
	v, ok := m.Lookup(path)
	return ok && v != ""
}

// Lookup returns the string value at path along with a boolean reporting
// whether the path was reachable. A reachable but empty value returns ("", true).
func (m *Message) Lookup(path string) (string, bool) {
	p, err := ParsePath(path)
	if err != nil {
		return "", false
	}
	return m.lookup(p)
}

func (m *Message) lookup(p Path) (string, bool) {
	seg, ok := m.FirstSegment(p.Segment)
	if !ok || p.Field < 1 || p.Field > len(seg.Fields) {
		return "", false
	}
	field := seg.Fields[p.Field-1]
	if p.Repetition > len(field.Repetitions) {
		return "", false
	}
	rep := field.Repetitions[p.Repetition-1]
	if p.Component > len(rep.Components) {
		return "", false
	}
	comp := rep.Components[p.Component-1]
	if p.Subcomponent > len(comp.Subcomponents) {
		return "", false
	}
	return comp.Subcomponents[p.Subcomponent-1], true
}
