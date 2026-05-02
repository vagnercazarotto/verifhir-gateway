package oru

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

const fixtureR01 = "MSH|^~\\&|LIS|LAB|HIS|FAC|20260502150000||ORU^R01|RES001|P|2.5\r" +
	"PID|1||MRN-099^^^HOSP^MR||JONES^BOB||19651120|M\r" +
	"OBR|1|ORD-099||1988-5^Hemoglobin^LN|R||20260502140000||||||||||||||||||F\r" +
	"OBX|1|NM|1988-5^Hemoglobin^LN||14.2|g/dL|13.5-17.5|N||F|||20260502150000\r" +
	"OBX|2|NM|718-7^Hematocrit^LN||41.5|%|38.0-52.0|H||F|||20260502150000"

func mustParse(t *testing.T, raw string) *parser.ParsedHL7 {
	t.Helper()
	msg, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return msg
}

func TestMapEventType(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	if r.EventType != "ORU^R01" {
		t.Errorf("EventType = %q, want ORU^R01", r.EventType)
	}
}

func TestMapSubject(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	if r.DiagnosticReport.Subject != "MRN-099" {
		t.Errorf("Subject = %q, want MRN-099", r.DiagnosticReport.Subject)
	}
}

func TestMapReportCode(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	dr := r.DiagnosticReport
	if dr.Code != "1988-5" {
		t.Errorf("Code = %q, want 1988-5", dr.Code)
	}
	if dr.CodeText != "Hemoglobin" {
		t.Errorf("CodeText = %q, want Hemoglobin", dr.CodeText)
	}
}

func TestMapReportStatusFinal(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	if r.DiagnosticReport.Status != "final" {
		t.Errorf("Status = %q, want final", r.DiagnosticReport.Status)
	}
}

func TestMapObservationCount(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	if len(r.DiagnosticReport.Observations) != 2 {
		t.Errorf("Observations count = %d, want 2", len(r.DiagnosticReport.Observations))
	}
}

func TestMapObservationValues(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	obs := r.DiagnosticReport.Observations[0]
	if obs.Value != "14.2" {
		t.Errorf("Value = %q, want 14.2", obs.Value)
	}
	if obs.Unit != "g/dL" {
		t.Errorf("Unit = %q, want g/dL", obs.Unit)
	}
	if obs.Abnormal {
		t.Errorf("expected Abnormal=false for normal result")
	}
}

func TestMapObservationAbnormal(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureR01))
	obs := r.DiagnosticReport.Observations[1]
	if !obs.Abnormal {
		t.Errorf("expected Abnormal=true for OBX-8=H")
	}
}
