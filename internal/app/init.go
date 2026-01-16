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
	// Check if AI is enabled in config
	if !cfg.AI.Enabled {
		return nil, nil // AI disabled, not an error
	}

	// Validate AI config
	if err := cfg.AI.Validate(); err != nil {
		log.Printf("AI config validation failed: %v (AI disabled)", err)
		return nil, nil
	}

	// Convert config.AIConfig to ai.Config
	aiConfig := ai.Config{
		Enabled:             cfg.AI.Enabled,
		OllamaEndpoint:      cfg.AI.OllamaEndpoint,
		Model:               cfg.AI.Model,
		ConfidenceThreshold: cfg.AI.ConfidenceThreshold,
		Timeout:             time.Duration(cfg.AI.TimeoutSeconds) * time.Second,
		CacheEnabled:        cfg.AI.CacheEnabled,
		CloudModel:          cfg.AI.CloudModel,
	}

	// Create AI integrator
	aiIntegrator, err := ai.NewIntegrator(aiConfig, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI integrator: %w", err)
	}

	return aiIntegrator, nil
}
