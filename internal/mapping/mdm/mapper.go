// Package mdm implements HL7v2 MDM^T02 (Medical Document Management) mapping
// to the FHIR R4 DocumentReference resource.
//
// Supported events: T02 (original document), T04 (addendum), T08 (replacement),
// T10 (deletion). All produce the same DocumentReference shape; the status
// field reflects the event.
//
// Field mapping reference (HL7v2.5 -> FHIR R4 DocumentReference):
//
//	MSH-9          -> MDMResult.EventType
//	TXA-2          -> DocumentReference.DocType
//	TXA-3          -> DocumentReference.DocTypeText
//	TXA-6          -> DocumentReference.CreatedAt
//	TXA-7          -> DocumentReference.AuthoredAt
//	TXA-11.2       -> DocumentReference.Author
//	PID-3.1        -> DocumentReference.Subject
//	OBX-5 (type TX)-> DocumentReference.Content / Title
package mdm

import (
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Map converts an MDM HL7v2 message to an MDMResult.
func Map(msgID string, msg *hl7v2.Message) model.MDMResult {
	event := resolveEventType(msg)

	doc := model.DocumentReference{
		ID:          "doc-" + msgID,
		Status:      mapDocStatus(event),
		DocType:     msg.Get("TXA-2"),
		DocTypeText: msg.Get("TXA-3"),
		Subject:     msg.Get("PID-3.1"),
		CreatedAt:   formatDateTime(msg.Get("TXA-6")),
		AuthoredAt:  formatDateTime(msg.Get("TXA-7")),
		Author:      msg.Get("TXA-11.2"), // component 2 = given name / authenticator name
	}

	// OBX segments with value type TX hold the document body.
	for _, obx := range msg.AllSegments("OBX") {
		vtype := strings.ToUpper(strings.TrimSpace(fieldVal(obx, 2)))
		if vtype != "TX" && vtype != "FT" && vtype != "ST" {
			continue
		}
		text := fieldVal(obx, 5)
		if text == "" {
			continue
		}
		// First text OBX is the title (if OBX-3 has a title code).
		if doc.Title == "" {
			doc.Title = compVal(obx, 3, 1) // OBX-3.2 text description
		}
		if doc.Content == "" {
			doc.Content = text
		} else {
			doc.Content += "\n" + text
		}
	}

	return model.MDMResult{EventType: event, DocumentReference: doc}
}

func mapDocStatus(event string) string {
	switch {
	case strings.HasSuffix(event, "T02"), strings.HasSuffix(event, "T04"):
		return "current"
	case strings.HasSuffix(event, "T08"):
		return "superseded"
	case strings.HasSuffix(event, "T10"):
		return "entered-in-error"
	default:
		return "current"
	}
}

func resolveEventType(msg *hl7v2.Message) string {
	t := strings.TrimSpace(msg.Get("MSH-9.1"))
	e := strings.TrimSpace(msg.Get("MSH-9.2"))
	if t == "" {
		return strings.TrimSpace(msg.Get("MSH-9"))
	}
	if e == "" {
		return t
	}
	return t + "^" + e
}

func fieldVal(seg hl7v2.Segment, f int) string {
	if f < 1 || f > len(seg.Fields) {
		return ""
	}
	reps := seg.Fields[f-1].Repetitions
	if len(reps) == 0 || len(reps[0].Components) == 0 {
		return ""
	}
	subs := reps[0].Components[0].Subcomponents
	if len(subs) == 0 {
		return ""
	}
	return strings.TrimSpace(subs[0])
}

func compVal(seg hl7v2.Segment, f, c int) string {
	if f < 1 || f > len(seg.Fields) {
		return ""
	}
	reps := seg.Fields[f-1].Repetitions
	if len(reps) == 0 || c >= len(reps[0].Components) {
		return ""
	}
	subs := reps[0].Components[c].Subcomponents
	if len(subs) == 0 {
		return ""
	}
	return strings.TrimSpace(subs[0])
}

func formatDateTime(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return s
	}
	result := s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	if len(s) >= 12 {
		result += "T" + s[8:10] + ":" + s[10:12]
		if len(s) >= 14 {
			result += ":" + s[12:14]
		}
	}
	return result
}
