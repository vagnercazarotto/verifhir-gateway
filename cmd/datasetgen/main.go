package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type profileConfig struct {
	Name             string
	SourceApp        string
	SourceFacility   string
	DestApp          string
	DestFacility     string
	PatientClass     string
	Locations        []string
	AttendingDoctors []string
	DefaultTypes     []string
}

type errorDefinition struct {
	Type        string
	Description string
	Severity    string
	Apply       func(payload, eventType string) string
}

type summary struct {
	Seed           int64          `json:"seed"`
	GeneratedAt    time.Time      `json:"generated_at"`
	OutputDir      string         `json:"output_dir"`
	Total          int            `json:"total"`
	Valid          int            `json:"valid"`
	Invalid        int            `json:"invalid"`
	ErrorRate      float64        `json:"error_rate"`
	Profile        string         `json:"profile"`
	MessageTypes   []string       `json:"message_types"`
	SeverityWeight map[string]any `json:"severity_weight"`
	SeverityCounts map[string]int `json:"severity_counts"`
}

func main() {
	count := flag.Int("count", 200, "number of HL7v2 files to generate")
	outputDir := flag.String("out", ".local/datasets/hl7v2", "output directory")
	prefix := flag.String("prefix", "sample", "file name prefix")
	errorRate := flag.Float64("error-rate", 0.20, "fraction of files with injected errors (0.0-1.0)")
	typesRaw := flag.String("types", "", "comma-separated ADT event types (empty = profile defaults)")
	profileName := flag.String("profile", "small-hospital", "scenario profile: small-hospital, large-network, emergency-dept")
	lowWeight := flag.Float64("low-weight", 0.60, "relative selection weight for low severity errors")
	mediumWeight := flag.Float64("medium-weight", 0.30, "relative selection weight for medium severity errors")
	highWeight := flag.Float64("high-weight", 0.10, "relative selection weight for high severity errors")
	seed := flag.Int64("seed", 0, "random seed (0 = auto)")
	flag.Parse()

	if *count <= 0 {
		exitf("count must be > 0")
	}
	if *errorRate < 0 || *errorRate > 1 {
		exitf("error-rate must be between 0.0 and 1.0")
	}
	if *lowWeight < 0 || *mediumWeight < 0 || *highWeight < 0 {
		exitf("severity weights must be >= 0")
	}
	totalWeight := *lowWeight + *mediumWeight + *highWeight
	if totalWeight <= 0 {
		exitf("at least one severity weight must be > 0")
	}

	profile, ok := profiles()[strings.ToLower(strings.TrimSpace(*profileName))]
	if !ok {
		exitf("unknown profile: %s", *profileName)
	}

	messageTypes := parseTypes(*typesRaw)
	if len(messageTypes) == 0 {
		messageTypes = profile.DefaultTypes
	}
	if len(messageTypes) == 0 {
		exitf("at least one message type is required")
	}

	runSeed := *seed
	if runSeed == 0 {
		runSeed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(runSeed))

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		exitf("failed to create output directory: %v", err)
	}

	manifestPath := filepath.Join(*outputDir, "manifest.csv")
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		exitf("failed to create manifest: %v", err)
	}
	defer manifestFile.Close()

	csvWriter := csv.NewWriter(manifestFile)
	defer csvWriter.Flush()
	_ = csvWriter.Write([]string{"file", "message_type", "profile", "valid", "error_type", "severity", "description"})

	validCount := 0
	invalidCount := 0
	severityCounts := map[string]int{"low": 0, "medium": 0, "high": 0}
	errorCatalog := buildErrorCatalog()

	for i := 1; i <= *count; i++ {
		eventType := messageTypes[rng.Intn(len(messageTypes))]
		payload := buildValidHL7(eventType, i, profile, rng)

		isValid := true
		errorType := ""
		severity := ""
		description := "baseline valid message"

		if rng.Float64() < *errorRate {
			isValid = false
			payload, errorType, severity, description = injectError(payload, eventType, *lowWeight, *mediumWeight, *highWeight, errorCatalog, rng)
			severityCounts[severity]++
		}

		filename := fmt.Sprintf("%s-%05d-ADT_%s.hl7", *prefix, i, eventType)
		fullPath := filepath.Join(*outputDir, filename)

		if err := os.WriteFile(fullPath, []byte(payload), 0o644); err != nil {
			exitf("failed writing %s: %v", filename, err)
		}

		_ = csvWriter.Write([]string{filename, eventType, profile.Name, strconv.FormatBool(isValid), errorType, severity, description})

		if isValid {
			validCount++
		} else {
			invalidCount++
		}
	}

	sum := summary{
		Seed:         runSeed,
		GeneratedAt:  time.Now().UTC(),
		OutputDir:    *outputDir,
		Total:        *count,
		Valid:        validCount,
		Invalid:      invalidCount,
		ErrorRate:    *errorRate,
		Profile:      profile.Name,
		MessageTypes: messageTypes,
		SeverityWeight: map[string]any{
			"low":    *lowWeight,
			"medium": *mediumWeight,
			"high":   *highWeight,
		},
		SeverityCounts: severityCounts,
	}

	summaryPath := filepath.Join(*outputDir, "summary.json")
	jsonBytes, _ := json.MarshalIndent(sum, "", "  ")
	if err := os.WriteFile(summaryPath, jsonBytes, 0o644); err != nil {
		exitf("failed writing summary.json: %v", err)
	}

	fmt.Printf("Generated %d files in %s\n", *count, *outputDir)
	fmt.Printf("Valid: %d | Invalid: %d\n", validCount, invalidCount)
	fmt.Printf("Profile: %s\n", profile.Name)
	fmt.Printf("Severity counts: low=%d medium=%d high=%d\n", severityCounts["low"], severityCounts["medium"], severityCounts["high"])
	fmt.Printf("Manifest: %s\n", manifestPath)
	fmt.Printf("Summary: %s\n", summaryPath)
	fmt.Printf("Seed: %d\n", runSeed)
}

func parseTypes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(strings.ToUpper(p))
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func buildValidHL7(eventType string, idx int, profile profileConfig, rng *rand.Rand) string {
	now := time.Now().UTC().Format("20060102150405")
	msgID := fmt.Sprintf("MSG%06d", idx)
	mrn := fmt.Sprintf("MRN%07d", rng.Intn(9_999_999))
	visit := fmt.Sprintf("V%07d", rng.Intn(9_999_999))
	location := pick(rng, profile.Locations)
	doctor := pick(rng, profile.AttendingDoctors)
	family := pick(rng, []string{"SMITH", "JOHNSON", "WILLIAMS", "BROWN", "JONES", "MILLER"})
	given := pick(rng, []string{"JAMES", "MARY", "ROBERT", "PATRICIA", "JOHN", "LINDA"})
	sex := pick(rng, []string{"M", "F"})
	dob := randomDOB(rng)

	msh := fmt.Sprintf("MSH|^~\\&|%s|%s|%s|%s|%s||ADT^%s|%s|P|2.5", profile.SourceApp, profile.SourceFacility, profile.DestApp, profile.DestFacility, now, eventType, msgID)
	evn := fmt.Sprintf("EVN|%s|%s", eventType, now)
	pid := fmt.Sprintf("PID|1||%s^^^%s^MR||%s^%s||%s|%s", mrn, profile.SourceFacility, family, given, dob, sex)
	pv1 := fmt.Sprintf("PV1|1|%s|%s|||%s|||MED|||||||%s", profile.PatientClass, location, doctor, visit)

	return strings.Join([]string{msh, evn, pid, pv1}, "\r") + "\r"
}

func injectError(payload, eventType string, lowWeight, mediumWeight, highWeight float64, catalog []errorDefinition, rng *rand.Rand) (string, string, string, string) {
	severity := pickSeverity(lowWeight, mediumWeight, highWeight, rng)
	candidates := make([]errorDefinition, 0)
	for _, e := range catalog {
		if e.Severity == severity {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		candidates = catalog
	}
	chosen := candidates[rng.Intn(len(candidates))]
	return chosen.Apply(payload, eventType), chosen.Type, chosen.Severity, chosen.Description
}

func pickSeverity(lowWeight, mediumWeight, highWeight float64, rng *rand.Rand) string {
	total := lowWeight + mediumWeight + highWeight
	v := rng.Float64() * total
	if v < lowWeight {
		return "low"
	}
	v -= lowWeight
	if v < mediumWeight {
		return "medium"
	}
	return "high"
}

func buildErrorCatalog() []errorDefinition {
	return []errorDefinition{
		{
			Type:        "missing_event",
			Description: "MSH-9 event component removed",
			Severity:    "low",
			Apply: func(payload, eventType string) string {
				return strings.Replace(payload, "ADT^"+eventType, "ADT^", 1)
			},
		},
		{
			Type:        "missing_patient_id",
			Description: "PID-3 patient identifier removed",
			Severity:    "low",
			Apply: func(payload, eventType string) string {
				return strings.Replace(payload, "PID|1||", "PID|1|||", 1)
			},
		},
		{
			Type:        "invalid_version",
			Description: "MSH version changed from 2.5 to X",
			Severity:    "medium",
			Apply: func(payload, eventType string) string {
				return strings.Replace(payload, "|2.5", "|X", 1)
			},
		},
		{
			Type:        "wrong_delimiter",
			Description: "segment delimiter changed to LF",
			Severity:    "medium",
			Apply: func(payload, eventType string) string {
				return strings.Replace(payload, "\r", "\n", -1)
			},
		},
		{
			Type:        "missing_pid",
			Description: "PID segment removed",
			Severity:    "high",
			Apply: func(payload, eventType string) string {
				lines := strings.Split(strings.TrimSuffix(payload, "\r"), "\r")
				filtered := make([]string, 0, len(lines))
				for _, ln := range lines {
					if strings.HasPrefix(ln, "PID|") {
						continue
					}
					filtered = append(filtered, ln)
				}
				return strings.Join(filtered, "\r") + "\r"
			},
		},
		{
			Type:        "truncated_message",
			Description: "message truncated after EVN segment",
			Severity:    "high",
			Apply: func(payload, eventType string) string {
				lines := strings.Split(strings.TrimSuffix(payload, "\r"), "\r")
				if len(lines) >= 2 {
					return strings.Join(lines[:2], "\r") + "\r"
				}
				return payload
			},
		},
	}
}

func profiles() map[string]profileConfig {
	return map[string]profileConfig{
		"small-hospital": {
			Name:             "small-hospital",
			SourceApp:        "VERIFHIR",
			SourceFacility:   "SMH01",
			DestApp:          "FHIR",
			DestFacility:     "REGIONALHUB",
			PatientClass:     "I",
			Locations:        []string{"ER^01^01", "MED^02^01", "SURG^03^01"},
			AttendingDoctors: []string{"1234^DOE^JANE", "2222^LEE^ALEX"},
			DefaultTypes:     []string{"A01", "A03"},
		},
		"large-network": {
			Name:             "large-network",
			SourceApp:        "VERIFHIR",
			SourceFacility:   "NET01",
			DestApp:          "FHIR",
			DestFacility:     "CENTRALPLATFORM",
			PatientClass:     "I",
			Locations:        []string{"ER^10^01", "ICU^11^01", "CARD^12^01", "NEURO^13^01", "ONC^14^01"},
			AttendingDoctors: []string{"8001^NGUYEN^SAM", "8002^PATEL^RAVI", "8003^MARTIN^EMMA"},
			DefaultTypes:     []string{"A01", "A03", "A08"},
		},
		"emergency-dept": {
			Name:             "emergency-dept",
			SourceApp:        "VERIFHIR",
			SourceFacility:   "ED01",
			DestApp:          "FHIR",
			DestFacility:     "TRAUMAHUB",
			PatientClass:     "E",
			Locations:        []string{"ER^01^01", "TRAUMA^02^01", "OBS^03^01"},
			AttendingDoctors: []string{"9901^HALL^NINA", "9902^YOUNG^RYAN"},
			DefaultTypes:     []string{"A01", "A08"},
		},
	}
}

func randomDOB(rng *rand.Rand) string {
	year := 1940 + rng.Intn(65)
	month := 1 + rng.Intn(12)
	day := 1 + rng.Intn(28)
	return fmt.Sprintf("%04d%02d%02d", year, month, day)
}

func pick(rng *rand.Rand, values []string) string {
	return values[rng.Intn(len(values))]
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
