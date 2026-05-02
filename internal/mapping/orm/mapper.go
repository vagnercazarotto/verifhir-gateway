// Package orm implements HL7v2 ORM^O01 (order message) mapping to the FHIR R4
// ServiceRequest resource.
//
// Field mapping reference (HL7v2.5 -> FHIR R4 ServiceRequest):
//
//	MSH-9      -> ORMResult.EventType
//	ORC-2      -> ServiceRequest.Items[].ID  (placer order number)
//	ORC-9      -> ServiceRequest.OrderedAt
//	ORC-25     -> ServiceRequest.Status  (order status -> FHIR status)
//	PID-3.1    -> ServiceRequest.Subject (patient MRN)
//	OBR-4.1/2  -> ServiceRequest.Items[].Code/CodeText
//	OBR-5      -> ServiceRequest.Items[].Priority
package orm

import (
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Map converts an ORM^O01 HL7v2 message to an ORMResult.
func Map(msgID string, msg *hl7v2.Message) model.ORMResult {
	result := model.ORMResult{
		EventType: resolveEventType(msg),
		ServiceRequest: model.ServiceRequest{
			ID:        "sr-" + msgID,
			Status:    mapOrderStatus(msg.Get("ORC-5")),
			Intent:    "order",
			Subject:   msg.Get("PID-3.1"),
			OrderedAt: formatDateTime(msg.Get("ORC-9")),
		},
	}

	// Each OBR segment is one order item.
	for _, obr := range msg.AllSegments("OBR") {
		item := model.OrderItem{
			ID:       fieldVal(obr, 2),  // OBR-2 (filler order number, fallback)
			Code:     compVal(obr, 4, 0), // OBR-4.1
			CodeText: compVal(obr, 4, 1), // OBR-4.2
			System:   compVal(obr, 4, 2), // OBR-4.3
			Priority: fieldVal(obr, 5),  // OBR-5
		}
		// Prefer ORC-2 placer number when available.
		if placerID := msg.Get("ORC-2"); placerID != "" {
			item.ID = placerID
		}
		result.ServiceRequest.Items = append(result.ServiceRequest.Items, item)
	}

	return result
}

// mapOrderStatus converts ORC-25 order status codes to FHIR ServiceRequest status.
func mapOrderStatus(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "IP", "A":
		return "active"
	case "CM":
		return "completed"
	case "CA", "DC":
		return "revoked"
	case "HD":
		return "on-hold"
	default:
		return "draft"
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

// fieldVal returns the raw string of field index f (1-based) of a segment.
func fieldVal(seg hl7v2.Segment, f int) string {
	if f < 1 || f > len(seg.Fields) {
		return ""
	}
	reps := seg.Fields[f-1].Repetitions
	if len(reps) == 0 || len(reps[0].Components) == 0 {
		return ""
	}
	comps := reps[0].Components[0].Subcomponents
	if len(comps) == 0 {
		return ""
	}
	return strings.TrimSpace(comps[0])
}

// compVal returns the string of component c (0-based) of field f (1-based).
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

// formatDateTime converts an HL7v2 DTM value to a partial ISO 8601 string.
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
