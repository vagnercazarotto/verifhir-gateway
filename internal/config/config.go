package config

import (
	"os"
)

// Config centralizes runtime settings.
type Config struct {
	HTTPPort     string
	MLLPAddr     string
	DBPath       string // SQLite file path; ignored when DATABASE_URL is set
	ChannelsFile string // optional YAML file with channel definitions
	AuditDir     string // directory for daily .jsonl audit files
}

func Load() Config {
	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	mllp := os.Getenv("GATEWAY_MLLP_ADDR")
	if mllp == "" {
		mllp = "0.0.0.0:2575"
	}

	dbPath := os.Getenv("GATEWAY_DB_PATH")
	if dbPath == "" {
		dbPath = ".local/verifhir.db"
	}

	auditDir := os.Getenv("GATEWAY_AUDIT_DIR")
	if auditDir == "" {
		auditDir = ".local/audit"
	}

	return Config{
		HTTPPort:     port,
		MLLPAddr:     mllp,
		DBPath:       dbPath,
		ChannelsFile: os.Getenv("GATEWAY_CHANNELS_FILE"),
		AuditDir:     auditDir,
	}
}
