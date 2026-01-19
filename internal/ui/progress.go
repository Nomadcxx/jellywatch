package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ProgressBar shows a simple progress bar
type ProgressBar struct {
	total   int
	current int
	width   int
	writer  *os.File
	label   string
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, label string) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		width:   40,
		writer:  os.Stdout,
		label:   label,
	}
}

// Update updates the progress bar
func (p *ProgressBar) Update(current int) {
	p.current = current
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

// Increment increments the progress by 1
func (p *ProgressBar) Increment() {
	p.current++
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

func (p *ProgressBar) render() {
	if !IsTerminal() {
		// Non-terminal: just print percentage
		percent := float64(p.current) / float64(p.total) * 100
		fmt.Fprintf(p.writer, "\r%s: %d/%d (%.1f%%)", p.label, p.current, p.total, percent)
		if p.current >= p.total {
			fmt.Fprintln(p.writer)
		}
		return
	}

	percent := float64(p.current) / float64(p.total) * 100
	filled := int(float64(p.width) * float64(p.current) / float64(p.total))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	fmt.Fprintf(p.writer, "\r%s [%s] %d/%d (%.1f%%)", p.label, bar, p.current, p.total, percent)

	if p.current >= p.total {
		fmt.Fprintln(p.writer)
	}
}

// Spinner shows an animated spinner for indeterminate progress
type Spinner struct {
	chars  []string
	index  int
	done   chan bool
	label  string
	ticker *time.Ticker
}

// NewSpinner creates a new spinner
func NewSpinner(label string) *Spinner {
	return &Spinner{
		chars: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index: 0,
		done:  make(chan bool),
		label: label,
	}
}

// Start starts the spinner animation
func (s *Spinner) Start() {
	if !IsTerminal() {
		fmt.Printf("%s...\n", s.label)
		return
	}

	s.ticker = time.NewTicker(100 * time.Millisecond)
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				fmt.Printf("\r%s %s", s.chars[s.index], s.label)
				s.index = (s.index + 1) % len(s.chars)
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.done <- true
	if IsTerminal() {
		fmt.Print("\r" + strings.Repeat(" ", len(s.label)+10) + "\r")
	}
}
