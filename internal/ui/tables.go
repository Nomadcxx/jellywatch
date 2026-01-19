package ui

import (
	"fmt"
	"strings"
)

// Table creates a formatted table for output
type Table struct {
	headers  []string
	rows     [][]string
	widths   []int
	maxWidth int // Maximum total table width
}

// NewTable creates a new table
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		headers:  headers,
		rows:     [][]string{},
		widths:   widths,
		maxWidth: 120, // Default max width
	}
}

// SetMaxWidth sets the maximum table width
func (t *Table) SetMaxWidth(width int) {
	t.maxWidth = width
}

// AddRow adds a row to the table
func (t *Table) AddRow(values ...string) {
	row := make([]string, len(t.headers))
	for i := range t.headers {
		if i < len(values) {
			row[i] = values[i]
			if len(values[i]) > t.widths[i] {
				t.widths[i] = len(values[i])
			}
		} else {
			row[i] = ""
		}
	}
	t.rows = append(t.rows, row)
}

// Render renders the table to stdout
func (t *Table) Render() {
	if len(t.headers) == 0 {
		return
	}

	// Calculate column widths with padding
	widths := make([]int, len(t.headers))
	totalWidth := 0
	for i, h := range t.headers {
		widths[i] = len(h)
		for _, row := range t.rows {
			if i < len(row) && len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
		widths[i] += 2 // Padding
		totalWidth += widths[i] + 1 // +1 for separator
	}

	// Adjust if too wide
	if totalWidth > t.maxWidth {
		excess := totalWidth - t.maxWidth
		// Reduce largest columns first
		for excess > 0 {
			maxIdx := 0
			for i := 1; i < len(widths); i++ {
				if widths[i] > widths[maxIdx] {
					maxIdx = i
				}
			}
			if widths[maxIdx] > 10 {
				widths[maxIdx]--
				excess--
			} else {
				break
			}
		}
	}

	// Print header
	fmt.Print("┌")
	for i, w := range widths {
		fmt.Print(strings.Repeat("─", w))
		if i < len(widths)-1 {
			fmt.Print("┬")
		}
	}
	fmt.Println("┐")

	fmt.Print("│")
	for i, h := range t.headers {
		fmt.Printf(" %-*s│", widths[i]-2, truncate(h, widths[i]-2))
	}
	fmt.Println()

	// Print separator
	fmt.Print("├")
	for i, w := range widths {
		fmt.Print(strings.Repeat("─", w))
		if i < len(widths)-1 {
			fmt.Print("┼")
		}
	}
	fmt.Println("┤")

	// Print rows
	for _, row := range t.rows {
		fmt.Print("│")
		for i := range t.headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			fmt.Printf(" %-*s│", widths[i]-2, truncate(val, widths[i]-2))
		}
		fmt.Println()
	}

	// Print footer
	fmt.Print("└")
	for i, w := range widths {
		fmt.Print(strings.Repeat("─", w))
		if i < len(widths)-1 {
			fmt.Print("┴")
		}
	}
	fmt.Println("┘")
}

// CompactTable creates a simpler table without borders
func CompactTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
		for _, row := range rows {
			if i < len(row) && len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
		widths[i] += 2
	}

	// Print header
	for i, h := range headers {
		fmt.Printf("%-*s", widths[i], h)
		if i < len(headers)-1 {
			fmt.Print("  ")
		}
	}
	fmt.Println()

	// Print separator
	for i, w := range widths {
		fmt.Print(strings.Repeat("─", w))
		if i < len(widths)-1 {
			fmt.Print("  ")
		}
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i := range headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			fmt.Printf("%-*s", widths[i], val)
			if i < len(headers)-1 {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}
}

// truncate truncates a string to max length with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
