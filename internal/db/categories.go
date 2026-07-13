package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// GetCategories returns all categories.
func (s *SQLiteStore) GetCategories() ([]Category, error) {
	var cats []Category
	err := s.Select(&cats, "SELECT id, name FROM categories ORDER BY name")
	return cats, err
}

// CategoryWithCount is a category plus its job count.
type CategoryWithCount struct {
	ID      int64  `db:"id" json:"id"`
	Name    string `db:"name" json:"name"`
	JobCount int   `db:"job_count" json:"jobCount"`
}

// GetCategoriesWithCounts returns all categories with job counts in one query.
func (s *SQLiteStore) GetCategoriesWithCounts() ([]CategoryWithCount, error) {
	var cats []CategoryWithCount
	err := s.Select(&cats, `SELECT c.id, c.name, COUNT(j.id) as job_count
		FROM categories c
		LEFT JOIN jobs j ON j.category_id = c.id
		GROUP BY c.id, c.name
		ORDER BY c.name`)
	return cats, err
}

// GetCategoryByID returns a category by its ID.
func (s *SQLiteStore) GetCategoryByID(id int64) (Category, error) {
	var c Category
	err := s.Get(&c, "SELECT id, name FROM categories WHERE id = ?", id)
	if err != nil {
		return Category{}, fmt.Errorf("category %d not found", id)
	}
	return c, nil
}

// AddCategory creates a new category.
func (s *SQLiteStore) AddCategory(name string) (Category, error) {
	result, err := s.Exec("INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		return Category{}, fmt.Errorf("add category: %w", err)
	}
	id, _ := result.LastInsertId()
	return Category{ID: id, Name: name}, nil
}

// DeleteCategory removes a category by ID.
// Jobs in the deleted category are moved to uncategorized (NULL category_id).
func (s *SQLiteStore) DeleteCategory(id int64) error {
	return s.tx(func(tx *sqlx.Tx) error {
		if _, err := tx.Exec("UPDATE jobs SET category_id = NULL WHERE category_id = ?", id); err != nil {
			return fmt.Errorf("reassign jobs: %w", err)
		}
		result, err := tx.Exec("DELETE FROM categories WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("delete category: %w", err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return fmt.Errorf("category id %d not found", id)
		}
		return nil
	})
}

// RenameCategory renames a category by ID.
func (s *SQLiteStore) RenameCategory(id int64, newName string) error {
	result, err := s.Exec("UPDATE categories SET name = ? WHERE id = ?", newName, id)
	if err != nil {
		return fmt.Errorf("rename category: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("category id %d not found", id)
	}
	return nil
}

// HasCategory checks if a category exists by name.
func (s *SQLiteStore) HasCategory(name string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM categories WHERE name = ?", name)
	return count > 0, err
}

// CategoryIDByName resolves a category name to its ID. Returns 0 if not found.
func (s *SQLiteStore) CategoryIDByName(name string) (int64, error) {
	var id int64
	err := s.Get(&id, "SELECT id FROM categories WHERE name = ?", name)
	if err != nil {
		return 0, nil // not found
	}
	return id, nil
}

// CategoryJobCount returns the number of jobs in a category by ID.
func (s *SQLiteStore) CategoryJobCount(id int64) (int, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM jobs WHERE category_id = ?", id)
	return count, err
}
