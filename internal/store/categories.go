package store

import (
	"database/sql"
	"strings"
)

type Category struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // income | expense
	Color    string `json:"color"`
}

func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query("SELECT id, parent_id, name, kind, color FROM categories ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Kind, &c.Color); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) getCategory(id int64) (*Category, error) {
	var c Category
	err := s.db.QueryRow("SELECT id, parent_id, name, kind, color FROM categories WHERE id=?", id).
		Scan(&c.ID, &c.ParentID, &c.Name, &c.Kind, &c.Color)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) CreateCategory(c *Category) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return invalid("category name is required")
	}
	if c.ParentID != nil {
		p, err := s.getCategory(*c.ParentID)
		if err != nil {
			return invalid("parent category does not exist")
		}
		if p.ParentID != nil {
			return invalid("only two levels are allowed (category and subcategory)")
		}
		c.Kind = p.Kind // la sottocategoria eredita il tipo
	}
	if c.Kind != "income" && c.Kind != "expense" {
		return invalid("invalid category type")
	}
	res, err := s.db.Exec("INSERT INTO categories(parent_id,name,kind,color) VALUES(?,?,?,?)", c.ParentID, c.Name, c.Kind, c.Color)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateCategory(c *Category) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return invalid("category name is required")
	}
	res, err := s.db.Exec("UPDATE categories SET name=?, color=? WHERE id=?", c.Name, c.Color, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCategory elimina la categoria e le sue sottocategorie. Se ci sono movimenti o
// ricorrenze collegati, moveTo (categoria dello stesso tipo, fuori dall'albero eliminato) è obbligatorio.
func (s *Store) DeleteCategory(id int64, moveTo *int64) error {
	c, err := s.getCategory(id)
	if err != nil {
		return err
	}
	ids := []int64{id}
	rows, err := s.db.Query("SELECT id FROM categories WHERE parent_id=?", id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var sub int64
		rows.Scan(&sub)
		ids = append(ids, sub)
	}
	rows.Close()

	inList := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, v := range ids {
		args[i] = v
	}
	var used int
	err = s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM transactions WHERE category_id IN (`+inList+`))
	  + (SELECT COUNT(*) FROM recurring WHERE category_id IN (`+inList+`))`, append(args, args...)...).Scan(&used)
	if err != nil {
		return err
	}

	if used > 0 {
		if moveTo == nil {
			return invalid("the category has transactions: choose where to move them")
		}
		for _, v := range ids {
			if v == *moveTo {
				return invalid("the destination category cannot be the one being deleted")
			}
		}
		dest, err := s.getCategory(*moveTo)
		if err != nil {
			return invalid("destination category does not exist")
		}
		if dest.Kind != c.Kind {
			return invalid("the destination category must be of the same type")
		}
	}

	// da qui in poi si usa solo tx: con una sola connessione aperta, s.db andrebbe in deadlock
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if used > 0 {
		for _, tbl := range []string{"transactions", "recurring"} {
			if _, err := tx.Exec("UPDATE "+tbl+" SET category_id=? WHERE category_id IN ("+inList+")", append([]any{*moveTo}, args...)...); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec("DELETE FROM categories WHERE id IN ("+inList+")", args...); err != nil {
		return err
	}
	return tx.Commit()
}
