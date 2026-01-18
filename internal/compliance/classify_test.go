package compliance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifySuggestion_SameDirectory(t *testing.T) {
	// Same directory rename should always be SAFE
	current := "/tv/The Simpsons (1989)/Season 01/simpsons.s01e01.mkv"
	suggested := "/tv/The Simpsons (1989)/Season 01/The Simpsons (1989) S01E01.mkv"

	class, err := ClassifySuggestion(current, suggested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if class != ClassificationSafe {
		t.Errorf("same directory rename should be SAFE, got %s", class)
	}
}

func TestClassifySuggestion_CrossLibrarySameShow(t *testing.T) {
	// Cross-library move to EXISTING folder should be SAFE (consolidation)
	// This test requires creating temp directories
	tmpDir := t.TempDir()

	// Create existing show folder in tv2
	existingShow := filepath.Join(tmpDir, "tv2", "The Simpsons (1989)", "Season 01")
	if err := os.MkdirAll(existingShow, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	current := filepath.Join(tmpDir, "tv1", "The Simpsons (1989)", "Season 01", "file.mkv")
	suggested := filepath.Join(tmpDir, "tv2", "The Simpsons (1989)", "Season 01", "The Simpsons (1989) S01E01.mkv")

	// Create source structure too (for context extraction)
	sourceDir := filepath.Dir(current)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	class, err := ClassifySuggestion(current, suggested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if class != ClassificationSafe {
		t.Errorf("cross-library consolidation to existing folder should be SAFE, got %s", class)
	}
}

func TestClassifySuggestion_NewFolderIsRisky(t *testing.T) {
	// Creating new folder should be RISKY (might be duplicate)
	tmpDir := t.TempDir()

	// Don't create the target folder - it should be RISKY
	current := filepath.Join(tmpDir, "tv", "The Simpsons (1989)", "Season 01", "file.mkv")
	suggested := filepath.Join(tmpDir, "tv", "Simpsons (2020)", "Season 01", "Simpsons (2020) S01E01.mkv")

	// Create source structure
	sourceDir := filepath.Dir(current)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	class, err := ClassifySuggestion(current, suggested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if class != ClassificationRisky {
		t.Errorf("new folder creation should be RISKY, got %s", class)
	}
}

func TestClassifySuggestion_TitleMismatchIsRisky(t *testing.T) {
	// Different show names should be RISKY
	tmpDir := t.TempDir()

	// Create both folders
	simpsons := filepath.Join(tmpDir, "tv", "The Simpsons (1989)", "Season 01")
	familyGuy := filepath.Join(tmpDir, "tv", "Family Guy (1999)", "Season 01")
	os.MkdirAll(simpsons, 0755)
	os.MkdirAll(familyGuy, 0755)

	current := filepath.Join(simpsons, "file.mkv")
	suggested := filepath.Join(familyGuy, "Family Guy (1999) S01E01.mkv")

	class, err := ClassifySuggestion(current, suggested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if class != ClassificationRisky {
		t.Errorf("cross-show move should be RISKY, got %s", class)
	}
}
