package model

// HL7Message is the raw payload received from source systems.
type HL7Message struct {
	ID      string
	Source  string
	Payload string
}

// FHIRResource is a simplified representation of mapped output.
type FHIRResource struct {
	ResourceType string
	ID           string
	Body         map[string]any
}

// HumanName represents a FHIR HumanName element.
type HumanName struct {
	Family string
	Given  []string
}

// Identifier represents a FHIR Identifier element.
type Identifier struct {
	System string
	Value  string
}

// Address represents a FHIR Address element.
type Address struct {
	Line       []string
	City       string
	State      string
	PostalCode string
	Country    string
}

// Patient holds mapped FHIR R4 Patient fields extracted from PID.
type Patient struct {
	ID          string
	Identifiers []Identifier
	Name        []HumanName
	BirthDate   string // YYYY-MM-DD
	Gender      string // male | female | other | unknown
	Address     []Address
}

// Period represents a FHIR Period element (start/end datetimes).
type Period struct {
	Start string // ISO 8601
	End   string // ISO 8601, empty when not yet discharged
}

// Encounter holds mapped FHIR R4 Encounter fields extracted from PV1.
type Encounter struct {
	ID     string
	Status string // in-progress | finished | cancelled
	Class  string // IMP (inpatient) | AMB (ambulatory) | EMER (emergency)
	Period Period
}

// ADTResult is the output of mapping an ADT message. It always contains a
// Patient and optionally an Encounter (absent for update-only events like A08).
type ADTResult struct {
	EventType string // e.g. "ADT^A01"
	Patient   Patient
	Encounter *Encounter // nil for A08 (update)
}

// QualityFinding is a single scored observation about one mapped field.
type QualityFinding struct {
	Field  string  // HL7 path that was evaluated, e.g. "PID.5"
	Rule   string  // rule name: "required", "enum", "format", "plausibility"
	Value  string  // the actual value found (empty string if absent)
	Impact float64 // negative score deduction applied
}

// QualityReport contains scoring and warnings from mapping validation.
type QualityReport struct {
	Score        float64
	Completeness float64
	Conformity   float64
	Confidence   float64
	Findings     []QualityFinding
	Warnings     []string
}

// ---- ORM^O01 ---------------------------------------------------------------

// OrderItem holds a single item from an ORM^O01 order message.
type OrderItem struct {
	ID       string // ORC-2 (placer order number)
	Code     string // OBR-4.1 universal service identifier
	CodeText string // OBR-4.2 text description
	System   string // OBR-4.3 coding system (e.g. "LN" for LOINC)
	Priority string // OBR-5 priority (R=Routine, S=Stat, U=Urgent)
}

// ServiceRequest is the FHIR R4 ServiceRequest resource mapped from ORM^O01.
type ServiceRequest struct {
	ID        string
	Status    string      // draft | active | on-hold | revoked | completed
	Intent    string      // order | plan | proposal
	Subject   string      // patient MRN (PID-3.1)
	OrderedAt string      // ORC-9 datetime ISO 8601
	Items     []OrderItem // one per OBR segment
}

// ORMResult is the output of mapping an ORM^O01 message.
type ORMResult struct {
	EventType      string // "ORM^O01"
	ServiceRequest ServiceRequest
}

// ---- ORU^R01 ---------------------------------------------------------------

// Observation holds a single OBX observation.
type Observation struct {
	ID         string // OBX-1 set ID
	Code       string // OBX-3.1
	CodeText   string // OBX-3.2
	System     string // OBX-3.3
	Value      string // OBX-5 (stringified)
	Unit       string // OBX-6.1
	RangeText  string // OBX-7 reference range
	Status     string // OBX-11: F=final, P=preliminary, C=corrected, X=cancelled
	ObservedAt string // OBX-14 datetime ISO 8601
	Abnormal   bool   // true when OBX-8 is H, L, A, AA, LL, HH
}

// DiagnosticReport is the FHIR R4 DiagnosticReport mapped from ORU^R01.
type DiagnosticReport struct {
	ID           string
	Status       string        // registered | partial | final | amended | corrected | cancelled
	Code         string        // OBR-4.1 universal service ID
	CodeText     string        // OBR-4.2
	Subject      string        // patient MRN
	EffectiveAt  string        // OBR-7 observation date/time ISO 8601
	IssuedAt     string        // OBR-22 result status change date/time
	Observations []Observation // one per OBX segment
}

// ORUResult is the output of mapping an ORU^R01 message.
type ORUResult struct {
	EventType        string // "ORU^R01"
	DiagnosticReport DiagnosticReport
}

// ---- SIU^S12 ---------------------------------------------------------------

// AppointmentParticipant holds a single participant in an appointment.
type AppointmentParticipant struct {
	Role   string // FHIR ParticipantType (PART, ATND, CON, ...)
	Name   string // SCH-16 / AIP-3
	Status string // accepted | declined | tentative | needs-action
}

// Appointment is the FHIR R4 Appointment resource mapped from SIU^S12.
type Appointment struct {
	ID           string
	Status       string // proposed | pending | booked | arrived | fulfilled | cancelled | noshow
	ServiceCode  string // SCH-6.1
	ServiceText  string // SCH-6.2
	Start        string // SCH-11.4 ISO 8601
	End          string // SCH-11.4 + SCH-9 duration
	Comment      string // SCH-12
	Participants []AppointmentParticipant
}

// SIUResult is the output of mapping an SIU message.
type SIUResult struct {
	EventType   string // "SIU^S12", "SIU^S13", etc.
	Appointment Appointment
}

// ---- MDM^T02 ---------------------------------------------------------------

// DocumentReference is the FHIR R4 DocumentReference mapped from MDM^T02.
type DocumentReference struct {
	ID          string
	Status      string // current | superseded | entered-in-error
	DocType     string // TXA-2 document type code
	DocTypeText string // TXA-3 document content presentation
	Subject     string // patient MRN
	CreatedAt   string // TXA-6 origination datetime ISO 8601
	AuthoredAt  string // TXA-7 transcription datetime ISO 8601
	Author      string // TXA-11 authenticated by
	Title       string // OBX-5 where OBX-3 = document title
	Content     string // OBX-5 document text (first OBX with type TX)
}

// MDMResult is the output of mapping an MDM^T02 message.
type MDMResult struct {
	EventType         string // "MDM^T02"
	DocumentReference DocumentReference
}

// RoutedPayload is the final package sent to destinations.
type RoutedPayload struct {
	Resource FHIRResource
	Quality  QualityReport
	// RawHL7 holds the original HL7v2 message text. It is populated by the
	// ingest stage and used by hl7_passthrough channels.
	RawHL7 string
}
