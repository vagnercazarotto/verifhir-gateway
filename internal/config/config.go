package config

import (
	"os"
)

// Config centralizes runtime settings.
type Config struct {
	HTTPPort string
	MLLPAddr string
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

	return Config{HTTPPort: port, MLLPAddr: mllp}
}
