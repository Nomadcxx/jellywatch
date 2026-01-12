package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds AI configuration settings
type Config struct {
	Enabled             bool          `toml:"enabled"`
	OllamaEndpoint      string        `toml:"ollama_endpoint"`
	Model               string        `toml:"model"`
	ConfidenceThreshold float64       `toml:"confidence_threshold"`
	Timeout             time.Duration `toml:"timeout"`
	CacheEnabled        bool          `toml:"cache_enabled"`
	CloudModel          string        `toml:"cloud_model"` // Optional cloud model for fallback
}

// DefaultConfig returns default AI configuration
func DefaultConfig() Config {
	return Config{
		Enabled:             false,
		OllamaEndpoint:      "http://localhost:11434",
		Model:               "qwen2.5vl:7b",
		ConfidenceThreshold: 0.8,
		Timeout:             5 * time.Second,
		CacheEnabled:        true,
		CloudModel:          "nemotron-3-nano:30b-cloud", // Optional cloud fallback
	}
}

// LoadConfig loads AI configuration from a file
func LoadConfig(configPath string) (Config, error) {
	cfg := DefaultConfig()

	// If config file exists, load it
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return Config{}, fmt.Errorf("failed to read AI config: %w", err)
			}
			// Config doesn't exist, return defaults
			return cfg, nil
		}

		var fileCfg struct {
			Enabled             *bool    `json:"enabled"`
			OllamaEndpoint      *string  `json:"ollama_endpoint"`
			Model               *string  `json:"model"`
			ConfidenceThreshold *float64 `json:"confidence_threshold"`
			Timeout             *float64 `json:"timeout_sec"`
			CacheEnabled        *bool    `json:"cache_enabled"`
			CloudModel          *string  `json:"cloud_model"`
		}

		if err := json.Unmarshal(data, &fileCfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse AI config: %w", err)
		}

		// Apply non-nil values
		if fileCfg.Enabled != nil {
			cfg.Enabled = *fileCfg.Enabled
		}
		if fileCfg.OllamaEndpoint != nil {
			cfg.OllamaEndpoint = *fileCfg.OllamaEndpoint
		}
		if fileCfg.Model != nil {
			cfg.Model = *fileCfg.Model
		}
		if fileCfg.ConfidenceThreshold != nil {
			cfg.ConfidenceThreshold = *fileCfg.ConfidenceThreshold
		}
		if fileCfg.Timeout != nil {
			cfg.Timeout = time.Duration(*fileCfg.Timeout * float64(time.Second))
		}
		if fileCfg.CacheEnabled != nil {
			cfg.CacheEnabled = *fileCfg.CacheEnabled
		}
		if fileCfg.CloudModel != nil {
			cfg.CloudModel = *fileCfg.CloudModel
		}
	}

	return cfg, nil
}

// SaveConfig saves AI configuration to a file
func SaveConfig(cfg Config, configPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	fileCfg := struct {
		Enabled             *bool    `json:"enabled"`
		OllamaEndpoint      *string  `json:"ollama_endpoint"`
		Model               *string  `json:"model"`
		ConfidenceThreshold *float64 `json:"confidence_threshold"`
		Timeout             *float64 `json:"timeout_sec"`
		CacheEnabled        *bool    `json:"cache_enabled"`
		CloudModel          *string  `json:"cloud_model"`
	}{
		Enabled:             &cfg.Enabled,
		OllamaEndpoint:      &cfg.OllamaEndpoint,
		Model:               &cfg.Model,
		ConfidenceThreshold: &cfg.ConfidenceThreshold,
		CacheEnabled:        &cfg.CacheEnabled,
		CloudModel:          &cfg.CloudModel,
	}

	timeoutSec := cfg.Timeout.Seconds()
	fileCfg.Timeout = &timeoutSec

	data, err := json.MarshalIndent(fileCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal AI config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write AI config: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Enabled && c.Model == "" {
		return fmt.Errorf("AI enabled but no model specified")
	}

	if c.Enabled && c.OllamaEndpoint == "" {
		return fmt.Errorf("AI enabled but no Ollama endpoint specified")
	}

	if c.ConfidenceThreshold < 0 || c.ConfidenceThreshold > 1 {
		return fmt.Errorf("confidence threshold must be between 0 and 1")
	}

	if c.Timeout < 100*time.Millisecond {
		return fmt.Errorf("timeout must be at least 100ms")
	}

	return nil
}
