package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mattn/go-isatty"
)

var (
	// Detect if we're in a terminal
	isTerminal = isatty.IsTerminal(os.Stdout.Fd())
	colorEnabled = true
)

// DisableColors disables all color output
func DisableColors() {
	colorEnabled = false
	isTerminal = false
}

// EnableColors enables color output
func EnableColors() {
	colorEnabled = true
	isTerminal = isatty.IsTerminal(os.Stdout.Fd())
}

// IsTerminal checks if stdout is a terminal
func IsTerminal() bool {
	return isTerminal && colorEnabled
}

// Section prints a section header
func Section(title string) {
	fmt.Println()
	if IsTerminal() {
		fmt.Println("━━━ " + strings.ToUpper(title) + " ━━━")
	} else {
		fmt.Println(strings.ToUpper(title))
		fmt.Println(strings.Repeat("=", len(title)+6))
	}
}

// Subsection prints a subsection header
func Subsection(title string) {
	if IsTerminal() {
		fmt.Println("  " + title)
	} else {
		fmt.Println("  " + title)
	}
}

// FormatBytes formats bytes to human-readable format using go-humanize
func FormatBytes(bytes int64) string {
	return humanize.Bytes(uint64(bytes))
}

// FormatDuration formats duration to human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// Confirm prompts for user confirmation
func Confirm(prompt string) bool {
	if !IsTerminal() {
		// Non-interactive: default to no
		return false
	}

	fmt.Print(prompt + " (y/N): ")
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
