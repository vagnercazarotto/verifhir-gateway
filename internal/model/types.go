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
	Score		float64
	Completeness	float64
	Conformity	float64
	Confidence	float64
	Findings	[]QualityFinding
	Warnings	[]string
}

// RoutedPayload is the final package sent to destinations.
type RoutedPayload struct {
	Resource FHIRResource
	Quality  QualityReport
}
