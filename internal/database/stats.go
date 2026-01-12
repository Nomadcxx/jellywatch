package database

// Stats represents database statistics
type Stats struct {
	SeriesCount int
	MoviesCount int
}

// GetStats returns database statistics
func (m *MediaDB) GetStats() (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats Stats

	err := m.db.QueryRow(`SELECT COUNT(*) FROM series`).Scan(&stats.SeriesCount)
	if err != nil {
		return nil, err
	}

	err = m.db.QueryRow(`SELECT COUNT(*) FROM movies`).Scan(&stats.MoviesCount)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
