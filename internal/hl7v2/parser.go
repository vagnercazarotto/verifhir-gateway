package hl7v2

import (
	"errors"
	"fmt"
	"strings"
)

// Parse reads a raw HL7 v2 message string and returns the decomposed
// structure. Segment terminators may be CR, LF, or CRLF.
func Parse(raw string) (*Message, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("empty HL7 payload")
	}

	lines := splitSegments(raw)
	if len(lines) == 0 {
		return nil, errors.New("no segments found")
	}

	delims, err := extractDelimiters(lines[0])
	if err != nil {
		return nil, fmt.Errorf("delimiter extraction: %w", err)
	}

	msg := &Message{Delimiters: delims}
	for i, line := range lines {
		if line == "" {
			continue
		}
		seg, err := parseSegment(line, delims, i == 0)
		if err != nil {
			return nil, fmt.Errorf("segment %d (%s): %w", i, segmentName(line), err)
		}
		msg.Segments = append(msg.Segments, seg)
	}
	return msg, nil
}

func splitSegments(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
}

func segmentName(line string) string {
	if len(line) >= 3 {
		return line[:3]
	}
	return line
}

func parseSegment(line string, d Delimiters, isHeader bool) (Segment, error) {
	if len(line) < 3 {
		return Segment{}, errors.New("segment shorter than 3-character name")
	}
	if isHeader {
		return parseMSH(line, d)
	}

	seg := Segment{Name: line[:3]}
	if len(line) <= 4 {
		return seg, nil
	}
	body := line[4:]
	for _, fieldStr := range strings.Split(body, string(d.Field)) {
		seg.Fields = append(seg.Fields, parseField(fieldStr, d))
	}
	return seg, nil
}

// parseMSH special-cases MSH-1 (the field separator itself) and MSH-2 (the
// encoding characters), which are literals and must not be decomposed by the
// regular field/component/subcomponent rules.
func parseMSH(line string, d Delimiters) (Segment, error) {
	if len(line) < 8 {
		return Segment{}, errors.New("MSH segment too short")
	}
	seg := Segment{Name: "MSH"}
	seg.Fields = append(seg.Fields, literalField(string(d.Field)))
	seg.Fields = append(seg.Fields, literalField(line[4:8]))

	if len(line) <= 9 {
		return seg, nil
	}
	body := line[9:]
	for _, fieldStr := range strings.Split(body, string(d.Field)) {
		seg.Fields = append(seg.Fields, parseField(fieldStr, d))
	}
	return seg, nil
}

func parseField(s string, d Delimiters) Field {
	if s == "" {
		return emptyField()
	}
	f := Field{}
	for _, rep := range strings.Split(s, string(d.Repetition)) {
		f.Repetitions = append(f.Repetitions, parseRepetition(rep, d))
	}
	return f
}

func parseRepetition(s string, d Delimiters) Repetition {
	if s == "" {
		return Repetition{Components: []Component{{Subcomponents: []string{""}}}}
	}
	r := Repetition{}
	for _, comp := range strings.Split(s, string(d.Component)) {
		r.Components = append(r.Components, parseComponent(comp, d))
	}
	return r
}

func parseComponent(s string, d Delimiters) Component {
	if s == "" {
		return Component{Subcomponents: []string{""}}
	}
	return Component{Subcomponents: strings.Split(s, string(d.Subcomponent))}
}

func emptyField() Field {
	return Field{
		Repetitions: []Repetition{
			{Components: []Component{{Subcomponents: []string{""}}}},
		},
	}
}

func literalField(s string) Field {
	return Field{
		Repetitions: []Repetition{
			{Components: []Component{{Subcomponents: []string{s}}}},
		},
	}
}
