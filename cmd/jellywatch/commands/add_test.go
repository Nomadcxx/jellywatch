package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddCommand_AutoDetectFileVsFolder(t *testing.T) {
	// Create temp directory with test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mkv")
	os.WriteFile(testFile, []byte("test"), 0644)

	// Test file detection
	isDir, err := isDirectory(testFile)
	if err != nil {
		t.Fatalf("isDirectory failed: %v", err)
	}
	if isDir {
		t.Error("Expected file to be detected as file, got directory")
	}

	// Test directory detection
	isDir, err = isDirectory(tmpDir)
	if err != nil {
		t.Fatalf("isDirectory failed: %v", err)
	}
	if !isDir {
		t.Error("Expected directory to be detected as directory, got file")
	}
}
