package adt

import (
	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// mapEncounter extracts FHIR R4 Encounter fields from the PV1 segment.
//
// Field mapping reference (HL7v2.5 -> FHIR R4 Encounter):
//
//	PV1-2      -> status (derived from patient class)
//	PV1-3      -> location (point of care ^ room ^ bed)
//	PV1-44     -> period.start (admit date/time)
//	PV1-45     -> period.end   (discharge date/time)
//
// The encounter status is set by the caller based on the ADT event type:
// A01 -> in-progress, A03 -> finished.
func mapEncounter(msgID string, status string, msg *hl7v2.Message) model.Encounter {
	enc := model.Encounter{
		ID:     "enc-" + msgID,
		Status: status,
		Class:  mapEncounterClass(msg.Get("PV1-2")),
	}

	// PV1-44: admit date/time (HL7 DTM -> ISO 8601).
	if admit := msg.Get("PV1-44"); admit != "" {
		enc.Period.Start = formatDateTime(admit)
	}

	// PV1-45: discharge date/time (empty when patient is still admitted).
	if discharge := msg.Get("PV1-45"); discharge != "" {
		enc.Period.End = formatDateTime(discharge)
	}

	return enc
}

// mapEncounterClass maps PV1-2 (patient class) to FHIR v3 ActCode values.
//
//	I = Inpatient      -> IMP
//	O = Outpatient     -> AMB
//	E = Emergency      -> EMER
//	R = Recurring      -> AMB
//	B = Obstetrics     -> IMP
//	C = Commercial     -> AMB
//	N = Not Applicable -> AMB (default)
func mapEncounterClass(code string) string {
	switch code {
	case "I", "B":
		return "IMP"
	case "E":
		return "EMER"
	default:
		return "AMB"
	}
}

// formatDateTime converts an HL7v2 DTM value (YYYYMMDDHHMMSS[+-ZZZZ]) to a
// partial ISO 8601 datetime string (YYYY-MM-DDTHH:MM:SS). Only the date
// portion is required; time components are appended when present.
func formatDateTime(s string) string {
	s = trim(s)
	if len(s) < 8 {
		return s
	}
	// Date only.
	result := s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	if len(s) >= 12 {
		result += "T" + s[8:10] + ":" + s[10:12]
		if len(s) >= 14 {
			result += ":" + s[12:14]
		}
	}
	return result
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
