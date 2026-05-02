// Package adt implements HL7v2 ADT event mapping to FHIR R4 resources.
package adt

import (
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// mapPatient extracts FHIR R4 Patient fields from the PID segment.
//
// Field mapping reference (HL7v2.5 -> FHIR R4 Patient):
//
//	PID-3      -> identifier (patient ID / MRN)
//	PID-5      -> name (family=PID-5.1, given=PID-5.2)
//	PID-7      -> birthDate (YYYYMMDD -> YYYY-MM-DD)
//	PID-8      -> gender (M/F/O/U -> male/female/other/unknown)
//	PID-11     -> address
func mapPatient(msgID string, msg *hl7v2.Message) model.Patient {
	p := model.Patient{ID: msgID}

	// PID-3: patient identifier list (repeating field, first repetition used).
	if id := msg.Get("PID-3.1"); id != "" {
		system := msg.Get("PID-3.4") // assigning authority
		p.Identifiers = append(p.Identifiers, model.Identifier{
			System: system,
			Value:  id,
		})
	}

	// PID-5: patient name (XPN — family^given^middle^suffix^prefix).
	if family := msg.Get("PID-5.1"); family != "" {
		name := model.HumanName{Family: family}
		if given := msg.Get("PID-5.2"); given != "" {
			name.Given = append(name.Given, given)
		}
		if middle := msg.Get("PID-5.3"); middle != "" {
			name.Given = append(name.Given, middle)
		}
		p.Name = append(p.Name, name)
	}

	// PID-7: date of birth — convert YYYYMMDD to YYYY-MM-DD.
	if dob := msg.Get("PID-7"); dob != "" {
		p.BirthDate = formatDate(dob)
	}

	// PID-8: administrative sex.
	p.Gender = mapGender(msg.Get("PID-8"))

	// PID-11: patient address (XAD).
	street := msg.Get("PID-11.1")
	city := msg.Get("PID-11.3")
	state := msg.Get("PID-11.4")
	zip := msg.Get("PID-11.5")
	country := msg.Get("PID-11.6")
	if street != "" || city != "" || state != "" || zip != "" {
		addr := model.Address{
			City:       city,
			State:      state,
			PostalCode: zip,
			Country:    country,
		}
		if street != "" {
			addr.Line = []string{street}
		}
		p.Address = append(p.Address, addr)
	}

	return p
}

// mapGender converts HL7v2 PID-8 administrative sex codes to FHIR gender values.
func mapGender(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "M":
		return "male"
	case "F":
		return "female"
	case "O":
		return "other"
	default:
		return "unknown"
	}
}

// formatDate converts an HL7v2 date (YYYYMMDD or YYYYMMDDHHMMSS) to YYYY-MM-DD.
// Returns the original string unchanged if it does not match.
func formatDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 8 {
		return s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	return s
}
