package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	// ListenAddress is the gRPC server listen address (default "0.0.0.0:4050").
	ListenAddress string
	// GCPCredentialsFile is the path to the GCP service account JSON key file.
	GCPCredentialsFile string
	// KMSKeyName is the full resource name of the GCP KMS crypto key.
	// Format: projects/*/locations/*/keyRings/*/cryptoKeys/*
	KMSKeyName string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddress:      envOrDefault("KMS_LISTEN_ADDRESS", "0.0.0.0:4050"),
		GCPCredentialsFile: os.Getenv("KMS_GCP_CREDENTIALS_FILE"),
		KMSKeyName:         os.Getenv("KMS_KEY_NAME"),
	}

	if cfg.GCPCredentialsFile == "" {
		return nil, fmt.Errorf("KMS_GCP_CREDENTIALS_FILE is required")
	}
	if cfg.KMSKeyName == "" {
		return nil, fmt.Errorf("KMS_KEY_NAME is required")
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
