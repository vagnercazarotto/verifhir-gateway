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

// QualityReport contains scoring and warnings from mapping validation.
type QualityReport struct {
	Score    float64
	Warnings []string
}

// RoutedPayload is the final package sent to destinations.
type RoutedPayload struct {
	Resource FHIRResource
	Quality  QualityReport
}
