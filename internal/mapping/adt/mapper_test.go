package adt

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

// Fixtures — real HL7v2 ADT messages with CR segment terminators.

const fixtureA01 = "MSH|^~\\&|SENDING|FAC|RECEIVING|FAC|20260502120000||ADT^A01|MSG001|P|2.5\r" +
	"EVN|A01|20260502120000\r" +
	"PID|1||MRN-001^^^HOSP^MR||DOE^JOHN^A||19800115|M|||123 MAIN ST^^BOSTON^MA^02101^USA\r" +
	"PV1|1|I|ICU^101^A|E|||DOC001^SMITH^JANE|||INT|||||ADM123|A||||||||||||||||||||||||||||20260502120000"

const fixtureA03 = "MSH|^~\\&|SENDING|FAC|RECEIVING|FAC|20260503090000||ADT^A03|MSG002|P|2.5\r" +
	"EVN|A03|20260503090000\r" +
	"PID|1||MRN-001^^^HOSP^MR||DOE^JOHN^A||19800115|M\r" +
	"PV1|1|I|ICU^101^A||||||||||||ADM123|A||||||||||||||||||||||||||||20260502120000|20260503090000"

const fixtureA08 = "MSH|^~\\&|SENDING|FAC|RECEIVING|FAC|20260504080000||ADT^A08|MSG003|P|2.5\r" +
	"EVN|A08|20260504080000\r" +
	"PID|1||MRN-001^^^HOSP^MR||DOE^JANE^B||19820320|F|||456 ELM ST^^CAMBRIDGE^MA^02139^USA"

const fixtureUnknown = "MSH|^~\\&|SENDING|FAC|RECEIVING|FAC|20260504080000||ADT^A99|MSG004|P|2.5\r" +
	"PID|1||MRN-001^^^HOSP^MR"

// helpers

func mustParse(t *testing.T, raw string) *parser.ParsedHL7 {
	t.Helper()
	msg, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return msg
}

// A01 — admit

func TestMapA01EventType(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventType != "ADT^A01" {
		t.Errorf("EventType = %q, want %q", result.EventType, "ADT^A01")
	}
}

func TestMapA01PatientID(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Patient.Identifiers) == 0 {
		t.Fatal("expected at least one patient identifier")
	}
	if result.Patient.Identifiers[0].Value != "MRN-001" {
		t.Errorf("identifier = %q, want %q", result.Patient.Identifiers[0].Value, "MRN-001")
	}
}

func TestMapA01PatientName(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Patient.Name) == 0 {
		t.Fatal("expected patient name")
	}
	if result.Patient.Name[0].Family != "DOE" {
		t.Errorf("family = %q, want %q", result.Patient.Name[0].Family, "DOE")
	}
	if len(result.Patient.Name[0].Given) == 0 || result.Patient.Name[0].Given[0] != "JOHN" {
		t.Errorf("given = %v, want [JOHN ...]", result.Patient.Name[0].Given)
	}
}

func TestMapA01PatientBirthDate(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Patient.BirthDate != "1980-01-15" {
		t.Errorf("BirthDate = %q, want %q", result.Patient.BirthDate, "1980-01-15")
	}
}

func TestMapA01PatientGender(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Patient.Gender != "male" {
		t.Errorf("Gender = %q, want %q", result.Patient.Gender, "male")
	}
}

func TestMapA01PatientAddress(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Patient.Address) == 0 {
		t.Fatal("expected patient address")
	}
	addr := result.Patient.Address[0]
	if addr.City != "BOSTON" {
		t.Errorf("City = %q, want %q", addr.City, "BOSTON")
	}
	if addr.State != "MA" {
		t.Errorf("State = %q, want %q", addr.State, "MA")
	}
	if addr.PostalCode != "02101" {
		t.Errorf("PostalCode = %q, want %q", addr.PostalCode, "02101")
	}
}

func TestMapA01HasEncounterInProgress(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, err := Map("msg-001", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Encounter == nil {
		t.Fatal("expected Encounter for A01, got nil")
	}
	if result.Encounter.Status != "in-progress" {
		t.Errorf("Status = %q, want %q", result.Encounter.Status, "in-progress")
	}
}

func TestMapA01EncounterClassInpatient(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, _ := Map("msg-001", msg)
	if result.Encounter.Class != "IMP" {
		t.Errorf("Class = %q, want %q", result.Encounter.Class, "IMP")
	}
}

func TestMapA01EncounterPeriodStart(t *testing.T) {
	msg := mustParse(t, fixtureA01)
	result, _ := Map("msg-001", msg)
	if result.Encounter.Period.Start != "2026-05-02T12:00:00" {
		t.Errorf("Period.Start = %q, want %q", result.Encounter.Period.Start, "2026-05-02T12:00:00")
	}
}

// A03 — discharge

func TestMapA03EncounterStatusFinished(t *testing.T) {
	msg := mustParse(t, fixtureA03)
	result, err := Map("msg-002", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Encounter == nil {
		t.Fatal("expected Encounter for A03, got nil")
	}
	if result.Encounter.Status != "finished" {
		t.Errorf("Status = %q, want %q", result.Encounter.Status, "finished")
	}
}

func TestMapA03EncounterPeriodEnd(t *testing.T) {
	msg := mustParse(t, fixtureA03)
	result, _ := Map("msg-002", msg)
	if result.Encounter.Period.End != "2026-05-03T09:00:00" {
		t.Errorf("Period.End = %q, want %q", result.Encounter.Period.End, "2026-05-03T09:00:00")
	}
}

// A08 — update patient

func TestMapA08NoEncounter(t *testing.T) {
	msg := mustParse(t, fixtureA08)
	result, err := Map("msg-003", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Encounter != nil {
		t.Errorf("expected nil Encounter for A08, got %+v", result.Encounter)
	}
}

func TestMapA08PatientGenderFemale(t *testing.T) {
	msg := mustParse(t, fixtureA08)
	result, _ := Map("msg-003", msg)
	if result.Patient.Gender != "female" {
		t.Errorf("Gender = %q, want %q", result.Patient.Gender, "female")
	}
}

// Unsupported event type

func TestMapUnsupportedEventReturnsError(t *testing.T) {
	msg := mustParse(t, fixtureUnknown)
	_, err := Map("msg-004", msg)
	if err == nil {
		t.Fatal("expected error for unsupported ADT event, got nil")
	}
}

// Unit tests for helpers

func TestFormatDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"19800115", "1980-01-15"},
		{"20260502120000", "2026-05-02"},
		{"", ""},
		{"202", "202"},
	}
	for _, tc := range cases {
		got := formatDate(tc.in)
		if got != tc.want {
			t.Errorf("formatDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	cases := []struct{ in, want string }{
		{"20260502120000", "2026-05-02T12:00:00"},
		{"20260502", "2026-05-02"},
		{"202605021200", "2026-05-02T12:00"},
		{"", ""},
	}
	for _, tc := range cases {
		got := formatDateTime(tc.in)
		if got != tc.want {
			t.Errorf("formatDateTime(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapGender(t *testing.T) {
	cases := []struct{ in, want string }{
		{"M", "male"},
		{"F", "female"},
		{"O", "other"},
		{"U", "unknown"},
		{"", "unknown"},
		{"X", "unknown"},
		{"m", "male"},
		{"f", "female"},
	}
	for _, tc := range cases {
		got := mapGender(tc.in)
		if got != tc.want {
			t.Errorf("mapGender(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapEncounterClass(t *testing.T) {
	cases := []struct{ in, want string }{
		{"I", "IMP"},
		{"B", "IMP"},
		{"E", "EMER"},
		{"O", "AMB"},
		{"R", "AMB"},
		{"", "AMB"},
	}
	for _, tc := range cases {
		got := mapEncounterClass(tc.in)
		if got != tc.want {
			t.Errorf("mapEncounterClass(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
