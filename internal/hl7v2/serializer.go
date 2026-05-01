package hl7v2

import "strings"

// String reconstructs the HL7 v2 message text from the parsed structure,
// using the original delimiter set. Segments are joined with CR per the
// HL7 v2.x convention.
func (m *Message) String() string {
	var sb strings.Builder
	for i, seg := range m.Segments {
		if i > 0 {
			sb.WriteByte('\r')
		}
		sb.WriteString(seg.serialize(m.Delimiters))
	}
	return sb.String()
}

func (s Segment) serialize(d Delimiters) string {
	if s.Name == "MSH" {
		return s.serializeMSH(d)
	}
	parts := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		parts[i] = f.serialize(d)
	}
	return s.Name + string(d.Field) + strings.Join(parts, string(d.Field))
}

// serializeMSH preserves the MSH-1 / MSH-2 quirk: MSH-1 is the field
// separator itself (already implicit in the join below) and MSH-2 holds
// the literal encoding characters that must not be re-decomposed.
func (s Segment) serializeMSH(d Delimiters) string {
	parts := make([]string, 0, len(s.Fields))
	if len(s.Fields) >= 2 {
		parts = append(parts, s.Fields[1].literal())
	}
	for i := 2; i < len(s.Fields); i++ {
		parts = append(parts, s.Fields[i].serialize(d))
	}
	return "MSH" + string(d.Field) + strings.Join(parts, string(d.Field))
}

func (f Field) serialize(d Delimiters) string {
	parts := make([]string, len(f.Repetitions))
	for i, r := range f.Repetitions {
		parts[i] = r.serialize(d)
	}
	return strings.Join(parts, string(d.Repetition))
}

func (f Field) literal() string {
	if len(f.Repetitions) == 0 ||
		len(f.Repetitions[0].Components) == 0 ||
		len(f.Repetitions[0].Components[0].Subcomponents) == 0 {
		return ""
	}
	return f.Repetitions[0].Components[0].Subcomponents[0]
}

func (r Repetition) serialize(d Delimiters) string {
	parts := make([]string, len(r.Components))
	for i, c := range r.Components {
		parts[i] = c.serialize(d)
	}
	return strings.Join(parts, string(d.Component))
}

func (c Component) serialize(d Delimiters) string {
	return strings.Join(c.Subcomponents, string(d.Subcomponent))
}
