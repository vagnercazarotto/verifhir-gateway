// Package oru implements HL7v2 ORU^R01 (observation result) mapping to the
// FHIR R4 DiagnosticReport + Observation resources.
//
// Field mapping reference (HL7v2.5 -> FHIR R4):
//
//	MSH-9          -> ORUResult.EventType
//	PID-3.1        -> DiagnosticReport.Subject
//	OBR-4.1/2      -> DiagnosticReport.Code/CodeText
//	OBR-7          -> DiagnosticReport.EffectiveAt
//	OBR-22         -> DiagnosticReport.IssuedAt
//	OBR-25         -> DiagnosticReport.Status (mapped)
//	OBX-1          -> Observation.ID (set ID)
//	OBX-3.1/2/3    -> Observation.Code/CodeText/System
//	OBX-5          -> Observation.Value
//	OBX-6.1        -> Observation.Unit
//	OBX-7          -> Observation.RangeText
//	OBX-8          -> Observation.Abnormal (H/L/A/AA/HH/LL)
//	OBX-11         -> Observation.Status
//	OBX-14         -> Observation.ObservedAt
package oru

import (
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Map converts an ORU^R01 HL7v2 message to an ORUResult.
func Map(msgID string, msg *hl7v2.Message) model.ORUResult {
	report := model.DiagnosticReport{
		ID:          "dr-" + msgID,
		Status:      mapReportStatus(msg.Get("OBR-25")),
		Code:        msg.Get("OBR-4.1"),
		CodeText:    msg.Get("OBR-4.2"),
		Subject:     msg.Get("PID-3.1"),
		EffectiveAt: formatDateTime(msg.Get("OBR-7")),
		IssuedAt:    formatDateTime(msg.Get("OBR-22")),
	}

	for _, obx := range msg.AllSegments("OBX") {
		obs := model.Observation{
			ID:         fieldVal(obx, 1),
			Code:       compVal(obx, 3, 0),
			CodeText:   compVal(obx, 3, 1),
			System:     compVal(obx, 3, 2),
			Value:      fieldVal(obx, 5),
			Unit:       compVal(obx, 6, 0),
			RangeText:  fieldVal(obx, 7),
			Abnormal:   isAbnormal(fieldVal(obx, 8)),
			Status:     mapObxStatus(fieldVal(obx, 11)),
			ObservedAt: formatDateTime(fieldVal(obx, 14)),
		}
		report.Observations = append(report.Observations, obs)
	}

	return model.ORUResult{
		EventType:        resolveEventType(msg),
		DiagnosticReport: report,
	}
}

func mapReportStatus(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "F":
		return "final"
	case "P":
		return "partial"
	case "C":
		return "corrected"
	case "A":
		return "amended"
	case "X":
		return "cancelled"
	default:
		return "registered"
	}
}

func mapObxStatus(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "F":
		return "final"
	case "P":
		return "preliminary"
	case "C":
		return "corrected"
	case "X":
		return "cancelled"
	default:
		return "unknown"
	}
}

func isAbnormal(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "H", "L", "A", "AA", "HH", "LL":
		return true
	}
	return false
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
