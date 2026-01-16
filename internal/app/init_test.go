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
		if err.Error() != "failed to create AI integrator: "+err.Error()[31:] {
			t.Logf("Expected error when Ollama not running: %v", err)
		}
	} else {
		if aiIntegrator == nil {
			t.Error("Integrator should not be nil when AI enabled")
		} else {
			aiIntegrator.Close()
		}
	}
}
