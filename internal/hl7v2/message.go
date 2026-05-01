package hl7v2

// Message is a parsed HL7 v2 message decomposed into segments, fields,
// repetitions, components, and subcomponents.
//
// Use Get / Has / Lookup with HL7 dotted paths to read values:
//
//	msg.Get("PID-3")        // first PID, field 3, first repetition, first component, first subcomponent
//	msg.Get("PID-5.1")      // first component of PID-5
//	msg.Get("PID-3[2]")     // second repetition of PID-3
//	msg.Get("PID-3[2].4.1") // first subcomponent of fourth component of second repetition of PID-3
type Message struct {
	Delimiters Delimiters
	Segments   []Segment
}

// Segment is one line of an HL7 message (e.g. MSH, PID, PV1).
type Segment struct {
	Name   string  // three-character segment identifier
	Fields []Field // Fields[i] is HL7 field number i+1
}

// Field is a single HL7 field, possibly with multiple repetitions.
type Field struct {
	Repetitions []Repetition
}

// Repetition is one occurrence of a repeating field.
type Repetition struct {
	Components []Component
}

// Component is a structured piece of a repetition that may itself be
// subdivided into subcomponents.
type Component struct {
	Subcomponents []string
}

// FirstSegment returns the first segment matching name and whether it was found.
func (m *Message) FirstSegment(name string) (Segment, bool) {
	for _, s := range m.Segments {
		if s.Name == name {
			return s, true
		}
	}
	return Segment{}, false
}

// AllSegments returns every segment matching name (e.g. multiple OBX rows).
func (m *Message) AllSegments(name string) []Segment {
	var out []Segment
	for _, s := range m.Segments {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}
