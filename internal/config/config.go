package config

import (
	"os"
)

// Config centralizes runtime settings.
type Config struct {
	HTTPPort string
}

func Load() Config {
	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	return Config{HTTPPort: port}
}
