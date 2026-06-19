package db

import "fmt"

// GetCategories returns all categories.
func (s *Store) GetCategories() ([]Category, error) {
	var cats []Category
	err := s.Select(&cats, "SELECT id, name FROM categories ORDER BY name")
	return cats, err
}

// AddCategory creates a new category.
func (s *Store) AddCategory(name string) (Category, error) {
	result, err := s.Exec("INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		return Category{}, fmt.Errorf("add category: %w", err)
	}
	id, _ := result.LastInsertId()
	return Category{ID: id, Name: name}, nil
}

// DeleteCategory removes a category by name.
func (s *Store) DeleteCategory(name string) error {
	result, err := s.Exec("DELETE FROM categories WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("category %q not found", name)
	}
	return nil
}

// HasCategory checks if a category exists.
func (s *Store) HasCategory(name string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM categories WHERE name = ?", name)
	return count > 0, err
}
