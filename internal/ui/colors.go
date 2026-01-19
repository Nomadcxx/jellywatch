package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Base styles - will be initialized based on terminal support
	successStyle lipgloss.Style
	errorStyle   lipgloss.Style
	warningStyle lipgloss.Style
	infoStyle    lipgloss.Style
	dimStyle     lipgloss.Style
	safeStyle    lipgloss.Style
	riskyStyle   lipgloss.Style
	movieStyle   lipgloss.Style
	tvShowStyle  lipgloss.Style
	actionStyle  lipgloss.Style
	pathStyle    lipgloss.Style
)

func init() {
	initStyles()
}

func initStyles() {
	if !IsTerminal() {
		// Plain styles for non-terminal
		successStyle = lipgloss.NewStyle()
		errorStyle = lipgloss.NewStyle()
		warningStyle = lipgloss.NewStyle()
		infoStyle = lipgloss.NewStyle()
		dimStyle = lipgloss.NewStyle()
		safeStyle = lipgloss.NewStyle()
		riskyStyle = lipgloss.NewStyle()
		movieStyle = lipgloss.NewStyle()
		tvShowStyle = lipgloss.NewStyle()
		actionStyle = lipgloss.NewStyle()
		pathStyle = lipgloss.NewStyle()
		return
	}

	// Colored styles for terminal
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	safeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	riskyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	movieStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	tvShowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	actionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	pathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
}

// Success prints success text
func Success(text string) string {
	return successStyle.Render(text)
}

// Error prints error text
func Error(text string) string {
	return errorStyle.Render(text)
}

// Warning prints warning text
func Warning(text string) string {
	return warningStyle.Render(text)
}

// Info prints info text
func Info(text string) string {
	return infoStyle.Render(text)
}

// Dim prints dim text
func Dim(text string) string {
	return dimStyle.Render(text)
}

// Safe prints safe classification
func Safe(text string) string {
	return safeStyle.Render(text)
}

// Risky prints risky classification
func Risky(text string) string {
	return riskyStyle.Render(text)
}

// Movie prints movie text
func Movie(text string) string {
	return movieStyle.Render(text)
}

// TVShow prints TV show text
func TVShow(text string) string {
	return tvShowStyle.Render(text)
}

// Action prints action text
func Action(text string) string {
	return actionStyle.Render(text)
}

// Path prints path text
func Path(text string) string {
	return pathStyle.Render(text)
}

// SuccessMsg prints a success message
func SuccessMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Success("✓") + " " + msg)
}

// ErrorMsg prints an error message
func ErrorMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Error("✗") + " " + msg)
}

// WarningMsg prints a warning message
func WarningMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Warning("⚠") + " " + msg)
}

// InfoMsg prints an info message
func InfoMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Info("ℹ") + " " + msg)
}
