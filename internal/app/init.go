// internal/app/init.go
package app

import (
	"fmt"
	"log"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/ai"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
)

// InitAI initializes the AI integrator from config.
// Returns nil if AI is disabled or config is invalid.
// This is a shared helper used by both CLI and daemon.
func InitAI(cfg *config.Config, db *database.MediaDB) (*ai.Integrator, error) {
	return InitAIWithOverride(cfg, db, false)
}

// InitAIWithOverride initializes AI with optional force-enable.
// When forceEnable is true, overrides cfg.AI.Enabled (for --ai flag).
func InitAIWithOverride(cfg *config.Config, db *database.MediaDB, forceEnable bool) (*ai.Integrator, error) {
	// Check if AI should be enabled
	aiEnabled := cfg.AI.Enabled || forceEnable

	if !aiEnabled {
		return nil, nil
	}

	// Apply defaults when force-enabled without full config
	aiConfig := cfg.AI
	if forceEnable && !cfg.AI.Enabled {
		if aiConfig.OllamaEndpoint == "" {
			aiConfig.OllamaEndpoint = "http://localhost:11434"
		}
		if aiConfig.Model == "" {
			aiConfig.Model = "gemma:2b"
		}
		if aiConfig.ConfidenceThreshold == 0 {
			aiConfig.ConfidenceThreshold = 0.75
		}
		if aiConfig.TimeoutSeconds == 0 {
			aiConfig.TimeoutSeconds = 30
		}
		aiConfig.CacheEnabled = true
		aiConfig.Enabled = true
	}

	// Validate AI config
	if err := aiConfig.Validate(); err != nil {
		log.Printf("AI config validation failed: %v (AI disabled)", err)
		return nil, nil
	}

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
	aiIntegrator, err := ai.NewIntegrator(integConfig, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI integrator: %w", err)
	}

	return aiIntegrator, nil
}
