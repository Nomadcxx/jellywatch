package consolidate_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/consolidate"
	"github.com/Nomadcxx/jellywatch/internal/database"
)

func TestIntegrationConsolidator(t *testing.T) {
	// Create temporary directory for test database
	tempDir, err := ioutil.TempDir("", "jellywatch_consolidate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.OpenPath(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create test config
	cfg := &config.Config{
		Options: config.OptionsConfig{
			DryRun:          true, // Use dry-run to avoid actual file operations
			VerifyChecksums: false,
			DeleteSource:    false,
		},
	}

	// Create consolidator
	cons := consolidate.NewConsolidator(db, cfg)

	// Test that GetUnresolvedConflicts works
	conflicts, err := db.GetUnresolvedConflicts()
	if err != nil {
		t.Fatalf("Failed to get unresolved conflicts: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts initially, got %d", len(conflicts))
	}

	// Test that GenerateAllPlans works (it will call DetectConflicts internally)
	plans, err := cons.GenerateAllPlans()
	if err != nil {
		t.Fatalf("Failed to generate plans: %v", err)
	}

	// With an empty database, there should be no conflicts or plans
	if len(plans) != 0 {
		t.Errorf("Expected 0 plans with empty database, got %d", len(plans))
	}

	// Verify that stats are tracked correctly
	stats := cons.GetStats()
	if stats.ConflictsFound != 0 {
		t.Errorf("Expected 0 conflicts found, got %d", stats.ConflictsFound)
	}
	if stats.PlansGenerated != 0 {
		t.Errorf("Expected 0 plans generated, got %d", stats.PlansGenerated)
	}

	// Test DryRun method
	err = cons.DryRun()
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	// Test ExecuteAll method in dry-run mode
	err = cons.ExecuteAll(true)
	if err != nil {
		t.Fatalf("ExecuteAll(dryRun=true) failed: %v", err)
	}

	t.Logf("Integration test passed. Basic consolidator operations work correctly.")
	t.Logf("Note: Full conflict detection requires multiple series entries, which is prevented by UNIQUE constraint.")
}
