package siu

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

// SCH field layout (HL7v2.5):
// f1=PlacerApptID f2-f6=empty f7=AppointmentReason f8=AppointmentType(empty)
// f9=Duration f10=DurationUnits f11=TimingQty(TQ) f12=Comment
const fixtureS12 = "MSH|^~\\&|SCHED|FAC|HIS|FAC|20260502160000||SIU^S12|SCH001|P|2.5\r" +
	"SCH|APPT-001||||||CONSULT^Consultation^^LN||30|MIN|^^^20260510090000|Routine follow-up\r" +
	"PID|1||MRN-007^^^HOSP^MR||BROWN^CAROL||19900601|F\r" +
	"AIP|1||DOC007^JOHNSON^MARK|MD||20260510090000|30|MIN|A"

const fixtureS15 = "MSH|^~\\&|SCHED|FAC|HIS|FAC|20260503080000||SIU^S15|SCH002|P|2.5\r" +
	"SCH|APPT-001\r" +
	"PID|1||MRN-007"

func mustParse(t *testing.T, raw string) *parser.ParsedHL7 {
	t.Helper()
	msg, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return msg
}

func TestMapS12EventType(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureS12))
	if r.EventType != "SIU^S12" {
		t.Errorf("EventType = %q, want SIU^S12", r.EventType)
	}
}

func TestMapS12StatusBooked(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureS12))
	if r.Appointment.Status != "booked" {
		t.Errorf("Status = %q, want booked", r.Appointment.Status)
	}
}

func TestMapS15StatusCancelled(t *testing.T) {
	r := Map("msg-002", mustParse(t, fixtureS15))
	if r.Appointment.Status != "cancelled" {
		t.Errorf("Status = %q, want cancelled", r.Appointment.Status)
	}
}

func TestMapServiceCode(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureS12))
	if r.Appointment.ServiceCode != "CONSULT" {
		t.Errorf("ServiceCode = %q, want CONSULT", r.Appointment.ServiceCode)
	}
	if r.Appointment.ServiceText != "Consultation" {
		t.Errorf("ServiceText = %q, want Consultation", r.Appointment.ServiceText)
	}
}

func TestMapParticipant(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureS12))
	if len(r.Appointment.Participants) == 0 {
		t.Fatal("expected at least one participant")
	}
	p := r.Appointment.Participants[0]
	if p.Role != "ATND" {
		t.Errorf("Role = %q, want ATND", p.Role)
	}
}

func TestMapStartDateTime(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureS12))
	// TQ component 4 encodes YYYYMMDDHHMMSS → 2026-05-10T09:00:00 or 2026-05-10T09:00
	if r.Appointment.Start == "" {
		t.Errorf("Start is empty, expected a formatted datetime")
	}
	if len(r.Appointment.Start) < 10 {
		t.Errorf("Start = %q, expected at least YYYY-MM-DD", r.Appointment.Start)
	}
}
