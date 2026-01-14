package database

import (
	"database/sql"
	"time"
)

// SkipReason defines why an item was skipped
type SkipReason string

const (
	SkipReasonParseFailure    SkipReason = "parse_failure"
	SkipReasonNoQualityWinner SkipReason = "no_quality_winner"
	SkipReasonAmbiguousTitle  SkipReason = "ambiguous_title"
	SkipReasonAIFailed        SkipReason = "ai_failed"
)

// SkippedItem represents a file that couldn't be processed automatically
type SkippedItem struct {
	ID           int64
	Path         string
	SkipReason   SkipReason
	ErrorDetails string
	AIAttempted  bool
	AIResult     string
	Attempts     int
	Status       string
	Resolution   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InsertSkippedItem adds a new item to the skip queue
func (m *MediaDB) InsertSkippedItem(path string, reason SkipReason, errorDetails string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO skipped_items (path, skip_reason, error_details)
		VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			skip_reason = excluded.skip_reason,
			error_details = excluded.error_details,
			attempts = attempts + 1,
			updated_at = CURRENT_TIMESTAMP
	`, path, reason, errorDetails)

	return err
}

// GetPendingSkippedItems returns all items awaiting review
func (m *MediaDB) GetPendingSkippedItems() ([]SkippedItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT id, path, skip_reason, error_details, ai_attempted,
		       COALESCE(ai_result, ''), attempts, status,
		       COALESCE(resolution, ''), created_at, updated_at
		FROM skipped_items
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSkippedItems(rows)
}

// GetSkippedItemsByReason returns items filtered by skip reason
func (m *MediaDB) GetSkippedItemsByReason(reason SkipReason) ([]SkippedItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT id, path, skip_reason, error_details, ai_attempted,
		       COALESCE(ai_result, ''), attempts, status,
		       COALESCE(resolution, ''), created_at, updated_at
		FROM skipped_items
		WHERE skip_reason = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, reason)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSkippedItems(rows)
}

// ResolveSkippedItem marks an item as resolved
func (m *MediaDB) ResolveSkippedItem(id int64, resolution string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE skipped_items
		SET status = 'resolved', resolution = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, resolution, id)

	return err
}

// IgnoreSkippedItem marks an item as ignored
func (m *MediaDB) IgnoreSkippedItem(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE skipped_items
		SET status = 'ignored', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)

	return err
}

// MarkAIAttempted records that AI was tried on this item
func (m *MediaDB) MarkAIAttempted(id int64, result string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE skipped_items
		SET ai_attempted = TRUE, ai_result = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, result, id)

	return err
}

// CountSkippedByStatus returns counts grouped by status
func (m *MediaDB) CountSkippedByStatus() (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT status, COUNT(*) FROM skipped_items GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}

	return counts, rows.Err()
}

// CountSkippedByReason returns counts grouped by skip reason (pending only)
func (m *MediaDB) CountSkippedByReason() (map[SkipReason]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT skip_reason, COUNT(*) FROM skipped_items
		WHERE status = 'pending'
		GROUP BY skip_reason
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[SkipReason]int)
	for rows.Next() {
		var reason string
		var count int
		if err := rows.Scan(&reason, &count); err != nil {
			return nil, err
		}
		counts[SkipReason(reason)] = count
	}

	return counts, rows.Err()
}

func scanSkippedItems(rows *sql.Rows) ([]SkippedItem, error) {
	var items []SkippedItem
	for rows.Next() {
		var item SkippedItem
		var reason string
		err := rows.Scan(
			&item.ID, &item.Path, &reason, &item.ErrorDetails,
			&item.AIAttempted, &item.AIResult, &item.Attempts,
			&item.Status, &item.Resolution, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		item.SkipReason = SkipReason(reason)
		items = append(items, item)
	}
	return items, rows.Err()
}
