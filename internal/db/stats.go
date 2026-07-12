package db

// Stats holds aggregated job statistics computed via GROUP BY queries
// instead of loading all jobs into memory.
type Stats struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"byStatus"`
	ByCategory map[string]int `json:"byCategory"`
}

// GetStats returns job counts grouped by status and category.
// Uses aggregate queries instead of loading all jobs into memory.
func (s *Store) GetStats() (Stats, error) {
	var total int
	if err := s.Get(&total, "SELECT COUNT(*) FROM jobs"); err != nil {
		return Stats{}, err
	}

	type statusCount struct {
		Status string `db:"status"`
		Count  int    `db:"count"`
	}
	var statusRows []statusCount
	if err := s.Select(&statusRows, "SELECT status, COUNT(*) as count FROM jobs GROUP BY status"); err != nil {
		return Stats{}, err
	}

	byStatus := make(map[string]int)
	for _, r := range statusRows {
		byStatus[r.Status] = r.Count
	}

	type categoryCount struct {
		Category string `db:"category"`
		Count    int    `db:"count"`
	}
	var catRows []categoryCount
	if err := s.Select(&catRows, `SELECT COALESCE(c.name, '') as category, COUNT(*) as count FROM jobs j LEFT JOIN categories c ON j.category_id = c.id GROUP BY c.name`); err != nil {
		return Stats{}, err
	}

	byCategory := make(map[string]int)
	for _, r := range catRows {
		if r.Category != "" {
			byCategory[r.Category] = r.Count
		}
	}

	return Stats{
		Total:      total,
		ByStatus:   byStatus,
		ByCategory: byCategory,
	}, nil
}
