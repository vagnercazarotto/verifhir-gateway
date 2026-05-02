// Package siu implements HL7v2 SIU (Scheduling Information Unsolicited) message
// mapping to the FHIR R4 Appointment resource.
//
// Supported events: S12 (new), S13 (reschedule), S14 (modify), S15 (cancel),
// S17 (delete), S26 (no-show). All produce the same Appointment shape; the
// status field reflects the event.
//
// Field mapping reference (HL7v2.5 -> FHIR R4 Appointment):
//
//	MSH-9          -> SIUResult.EventType
//	SCH-1          -> Appointment.ID (placer appointment ID)
//	SCH-6.1/2      -> ServiceCode/ServiceText
//	SCH-9          -> duration (minutes, used to compute End)
//	SCH-11.4       -> Start datetime
//	SCH-12         -> Comment
//	AIP-3          -> Participants[].Name (practitioner)
package siu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Map converts a SIU HL7v2 message to a SIUResult.
func Map(msgID string, msg *hl7v2.Message) model.SIUResult {
	event := resolveEventType(msg)
	startRaw := msg.Get("SCH-11.4")
	start := formatDateTime(startRaw)
	end := computeEnd(startRaw, msg.Get("SCH-9"))

	appt := model.Appointment{
		ID:          "appt-" + msgID,
		Status:      mapAppointmentStatus(event),
		ServiceCode: msg.Get("SCH-7.1"),
		ServiceText: msg.Get("SCH-7.2"),
		Start:       start,
		End:         end,
		Comment:     msg.Get("SCH-12"),
	}

	// AIP segments list practitioner participants.
	for _, aip := range msg.AllSegments("AIP") {
		name := compVal(aip, 3, 1) // AIP-3.2 (given) or full name
		if name == "" {
			name = fieldVal(aip, 3)
		}
		if name != "" {
			appt.Participants = append(appt.Participants, model.AppointmentParticipant{
				Role:   "ATND",
				Name:   name,
				Status: "accepted",
			})
		}
	}

	// AIL segments (locations) as participants.
	for _, ail := range msg.AllSegments("AIL") {
		loc := fieldVal(ail, 3)
		if loc != "" {
			appt.Participants = append(appt.Participants, model.AppointmentParticipant{
				Role:   "LOC",
				Name:   loc,
				Status: "accepted",
			})
		}
	}

	return model.SIUResult{EventType: event, Appointment: appt}
}

func mapAppointmentStatus(event string) string {
	switch {
	case strings.HasSuffix(event, "S12"):
		return "booked"
	case strings.HasSuffix(event, "S13"), strings.HasSuffix(event, "S14"):
		return "pending"
	case strings.HasSuffix(event, "S15"), strings.HasSuffix(event, "S17"):
		return "cancelled"
	case strings.HasSuffix(event, "S26"):
		return "noshow"
	default:
		return "proposed"
	}
}

// computeEnd adds duration minutes to the start DTM string.
// Returns empty string when either input is missing.
func computeEnd(startDTM, durationMin string) string {
	if startDTM == "" || durationMin == "" {
		return ""
	}
	mins, err := strconv.Atoi(strings.TrimSpace(durationMin))
	if err != nil || mins <= 0 {
		return ""
	}
	// encode as offset in the ISO string: start + Xm  (simple text arithmetic)
	// For a gateway purpose we store as "<start>+<mins>m" annotation; a full
	// FHIR server would compute a real timestamp.
	return fmt.Sprintf("%s+%dm", formatDateTime(startDTM), mins)
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
