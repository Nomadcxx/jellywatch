package commands

import "testing"

func TestCleanCommand_ModeSelection(t *testing.T) {
	tests := []struct {
		name       string
		duplicates bool
		naming     bool
		all        bool
		wantMode   cleanMode
	}{
		{"duplicates only", true, false, false, cleanModeDuplicates},
		{"naming only", false, true, false, cleanModeNaming},
		{"all", false, false, true, cleanModeAll},
		{"interactive (no flags)", false, false, false, cleanModeInteractive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := selectCleanMode(tt.duplicates, tt.naming, tt.all)
			if mode != tt.wantMode {
				t.Errorf("selectCleanMode() = %v, want %v", mode, tt.wantMode)
			}
		})
	}
}
