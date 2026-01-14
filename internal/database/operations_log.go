package database

import (
	"time"
)

// OperationType defines the type of operation performed
type OperationType string

const (
	OpKeep       OperationType = "keep"
	OpDelete     OperationType = "delete"
	OpMove       OperationType = "move"
	OpRename     OperationType = "rename"
	OpReorganize OperationType = "reorganize"
	OpSkip       OperationType = "skip"
)

// ExecutedBy defines who/what triggered the operation
type ExecutedBy string

const (
	ExecDaemon ExecutedBy = "daemon"
	ExecCLI    ExecutedBy = "cli"
	ExecUser   ExecutedBy = "user"
)

// OperationLog represents a logged operation
type OperationLog struct {
	ID                 int64
	OperationType      OperationType
	SourcePath         string
	TargetPath         string
	Reason             string
	QualityScoreSource int
	QualityScoreWinner int
	BytesFreed         int64
	ExecutedBy         ExecutedBy
	ExecutedAt         time.Time
}

// LogOperation records an operation in the audit log
func (m *MediaDB) LogOperation(op OperationType, sourcePath, targetPath, reason string, scoreSource, scoreWinner int, bytesFreed int64, executedBy ExecutedBy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO operations_log (
			operation_type, source_path, target_path, reason,
			quality_score_source, quality_score_winner, bytes_freed, executed_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, op, sourcePath, targetPath, reason, scoreSource, scoreWinner, bytesFreed, executedBy)

	return err
}

// GetRecentOperations returns the most recent operations
func (m *MediaDB) GetRecentOperations(limit int) ([]OperationLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT id, operation_type, source_path, COALESCE(target_path, ''),
		       COALESCE(reason, ''), COALESCE(quality_score_source, 0),
		       COALESCE(quality_score_winner, 0), COALESCE(bytes_freed, 0),
		       executed_by, executed_at
		FROM operations_log
		ORDER BY executed_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []OperationLog
	for rows.Next() {
		var op OperationLog
		var opType, execBy string
		err := rows.Scan(
			&op.ID, &opType, &op.SourcePath, &op.TargetPath,
			&op.Reason, &op.QualityScoreSource, &op.QualityScoreWinner,
			&op.BytesFreed, &execBy, &op.ExecutedAt,
		)
		if err != nil {
			return nil, err
		}
		op.OperationType = OperationType(opType)
		op.ExecutedBy = ExecutedBy(execBy)
		ops = append(ops, op)
	}

	return ops, rows.Err()
}

// GetOperationStats returns summary statistics
func (m *MediaDB) GetOperationStats() (map[OperationType]int, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count by type
	rows, err := m.db.Query(`
		SELECT operation_type, COUNT(*) FROM operations_log GROUP BY operation_type
	`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	counts := make(map[OperationType]int)
	for rows.Next() {
		var opType string
		var count int
		if err := rows.Scan(&opType, &count); err != nil {
			return nil, 0, err
		}
		counts[OperationType(opType)] = count
	}

	// Total bytes freed
	var totalFreed int64
	err = m.db.QueryRow(`
		SELECT COALESCE(SUM(bytes_freed), 0) FROM operations_log WHERE operation_type = 'delete'
	`).Scan(&totalFreed)
	if err != nil {
		return nil, 0, err
	}

	return counts, totalFreed, nil
}
