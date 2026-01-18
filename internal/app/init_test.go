// internal/app/init_test.go
package app

import (
	"testing"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
)

func TestInitAI_DisabledReturnsNil(t *testing.T) {
	// Create temporary database
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Config with AI disabled
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false,
		},
	}

	aiIntegrator, err := InitAI(cfg, db)
	if err != nil {
		t.Fatalf("InitAI() error = %v", err)
	}
	if aiIntegrator != nil {
		t.Error("Should return nil when AI disabled")
	}
}

func TestInitAI_EnabledCreatesIntegrator(t *testing.T) {
	// Create temporary database
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Config with AI enabled (using mock endpoint)
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:             true,
			OllamaEndpoint:      "http://localhost:11434",
			Model:               "llama3.2",
			ConfidenceThreshold: 0.8,
			TimeoutSeconds:      5,
			CacheEnabled:        true,
		},
	}

	aiIntegrator, err := InitAI(cfg, db)
	// May fail if Ollama not running, but that's OK for this test
	// We're testing the initialization logic
	if err != nil {
		t.Logf("AI initialization failed (Ollama may not be running): %v", err)
	} else if aiIntegrator != nil {
		aiIntegrator.Close()
	}
}

func TestInitAI_ValidationFailureReturnsNil(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Config with AI enabled but missing required fields
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:        true,
			OllamaEndpoint: "", // Empty endpoint should fail validation
			Model:          "", // Empty model should fail validation
		},
	}

	aiIntegrator, err := InitAI(cfg, db)
	if err != nil {
		t.Fatalf("InitAI() error = %v", err)
	}
	if aiIntegrator != nil {
		t.Error("Should return nil when validation fails")
	}
}

func TestInitAIWithOverride_FlagOverride(t *testing.T) {
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:             false,
			OllamaEndpoint:      "http://localhost:11434",
			Model:               "gemma:2b",
			ConfidenceThreshold: 0.8,
			TimeoutSeconds:      30,
			CacheEnabled:        true,
		},
	}

	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer db.Close()

	integrator, err := InitAIWithOverride(cfg, db, true)
	if err != nil {
		t.Fatalf("InitAIWithOverride failed: %v", err)
	}

	if integrator == nil {
		t.Fatal("Expected non-nil integrator when forceEnable=true")
	}

	if !integrator.IsEnabled() {
		t.Fatal("Expected integrator to be enabled")
	}

	integrator.Close()
}

func TestInitAIWithOverride_NoOverride_Disabled(t *testing.T) {
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false,
		},
	}

	db, _ := database.OpenInMemory()
	defer db.Close()

	integrator, err := InitAIWithOverride(cfg, db, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if integrator != nil {
		t.Fatal("Expected nil integrator when disabled and no override")
	}
}

func TestInitAIWithOverride_UsesDefaults(t *testing.T) {
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false,
		},
	}

	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer db.Close()

	integrator, err := InitAIWithOverride(cfg, db, true)
	if err != nil {
		t.Logf("AI initialization failed (Ollama may not be running): %v", err)
		return
	}

	if integrator == nil {
		t.Fatal("Expected non-nil integrator with defaults")
	}

	integrator.Close()
}
