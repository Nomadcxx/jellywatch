// internal/app/init.go
package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/ai"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/logging"
)

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

// InitAI initializes the AI integrator from config.
// Returns nil if AI is disabled or config is invalid.
// This is a shared helper used by both CLI and daemon.
func InitAI(cfg *config.Config, db *database.MediaDB, logger *logging.Logger) (*ai.Integrator, error) {
	return InitAIWithOverride(cfg, db, false, logger)
}

// InitAIWithOverride initializes AI with optional force-enable.
// When forceEnable is true, overrides cfg.AI.Enabled (for --ai flag).
func InitAIWithOverride(cfg *config.Config, db *database.MediaDB, forceEnable bool, logger *logging.Logger) (*ai.Integrator, error) {
	if logger == nil {
		logger = logging.Nop()
	}

	// Check if AI should be enabled
	aiEnabled := cfg.AI.Enabled || forceEnable

	if !aiEnabled {
		logger.Debug("app", "AI disabled")
		return nil, nil
	}

	logger.Debug("app", "Initializing AI integrator",
		logging.F("force_enable", forceEnable),
		logging.F("config_enabled", cfg.AI.Enabled))

	// Apply defaults when force-enabled without full config
	aiConfig := cfg.AI
	if forceEnable && !cfg.AI.Enabled {
		logger.Info("app", "AI force-enabled via flag, applying defaults")

		defaults := struct {
			OllamaEndpoint      string
			Model               string
			ConfidenceThreshold float64
			TimeoutSeconds      int
		}{
			OllamaEndpoint:      getEnvOrDefault("JELLYWATCH_AI_ENDPOINT", "http://localhost:11434"),
			Model:               getEnvOrDefault("JELLYWATCH_AI_MODEL", "gemma:2b"),
			ConfidenceThreshold: getEnvFloatOrDefault("JELLYWATCH_AI_CONFIDENCE", 0.75),
			TimeoutSeconds:      getEnvIntOrDefault("JELLYWATCH_AI_TIMEOUT", 30),
		}

		if aiConfig.OllamaEndpoint == "" {
			aiConfig.OllamaEndpoint = defaults.OllamaEndpoint
		}
		if aiConfig.Model == "" {
			aiConfig.Model = defaults.Model
		}
		if aiConfig.ConfidenceThreshold == 0 {
			aiConfig.ConfidenceThreshold = defaults.ConfidenceThreshold
		}
		if aiConfig.TimeoutSeconds == 0 {
			aiConfig.TimeoutSeconds = defaults.TimeoutSeconds
		}
		aiConfig.CacheEnabled = true
		aiConfig.Enabled = true
	}

	// Validate AI config
	if err := aiConfig.Validate(); err != nil {
		logger.Warn("app", "AI config validation failed, AI disabled",
			logging.F("error", err),
			logging.F("force_enable", forceEnable),
		)
		return nil, nil
	}

	logger.Info("app", "AI config validated successfully",
		logging.F("model", aiConfig.Model),
		logging.F("endpoint", aiConfig.OllamaEndpoint),
		logging.F("confidence_threshold", aiConfig.ConfidenceThreshold),
	)

	// Convert config.AIConfig to ai.Config
	integConfig := ai.Config{
		Enabled:             true,
		OllamaEndpoint:      aiConfig.OllamaEndpoint,
		Model:               aiConfig.Model,
		ConfidenceThreshold: aiConfig.ConfidenceThreshold,
		Timeout:             time.Duration(aiConfig.TimeoutSeconds) * time.Second,
		CacheEnabled:        aiConfig.CacheEnabled,
		CloudModel:          aiConfig.CloudModel,
	}

	// Create AI integrator
	logger.Debug("app", "Creating AI integrator")
	aiIntegrator, err := ai.NewIntegrator(integConfig, db)
	if err != nil {
		logger.Error("app", "Failed to create AI integrator", err)
		return nil, fmt.Errorf("failed to create AI integrator: %w", err)
	}

	logger.Info("app", "AI integrator created successfully")

	return aiIntegrator, nil
}
