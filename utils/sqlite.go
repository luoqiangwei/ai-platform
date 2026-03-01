package utils

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteClient struct {
	db   *sql.DB
	path string
}

// OpenSQLite initializes a SQLite connection.
func OpenSQLite(path string, config string) (*SQLiteClient, error) {
	dsn := path
	if config != "" {
		dsn = fmt.Sprintf("%s?%s", path, config)
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	return &SQLiteClient{db: db, path: path}, db.Ping()
}

// DropSQLiteDatabase removes the physical database file.
func DropSQLiteDatabase(path string) error {
	if path == ":memory:" {
		return nil
	}
	return os.Remove(path)
}

// Close closes the database connection.
func (c *SQLiteClient) Close(sync map[string]bool) error {
	// In SQLite, the OS handles file syncing.
	// The sync map is provided for interface compatibility.
	return c.db.Close()
}

// CreateTable creates a table based on FieldDescription.
func (c *SQLiteClient) CreateTable(tableName string, fields []FieldDescription) error {
	var defs []string
	pkCount := 0

	for _, f := range fields {
		if f.IsPrimaryKey {
			pkCount++
		}
		if pkCount > 1 {
			return errors.New("multiple primary keys are not allowed")
		}

		def := fmt.Sprintf("%s %s", f.Name, f.DataType)
		if f.IsPrimaryKey {
			// SQLite requires INTEGER PRIMARY KEY for AUTOINCREMENT
			if f.IsAutoIncrement {
				def = fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", f.Name)
			} else {
				def += " PRIMARY KEY"
			}
		} else if f.HasDefault {
			def += " DEFAULT " + f.DefaultValue
		}
		defs = append(defs, def)
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(defs, ", "))
	_, err := c.db.Exec(query)
	return err
}

// DropTable removes a table.
func (c *SQLiteClient) DropTable(tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	_, err := c.db.Exec(query)
	return err
}

// UpdateTable modifies the table structure.
func (c *SQLiteClient) UpdateTable(tableName string, removeFields []string, updateFields []FieldUpdateDescription, newFields []FieldDescription) error {
	// 1. Remove Fields
	for _, field := range removeFields {
		// Note: DROP COLUMN requires SQLite 3.35.0+
		query := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, field)
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to drop column %s: %w", field, err)
		}
	}

	// 2. Rename Fields
	for _, uf := range updateFields {
		// Note: RENAME COLUMN requires SQLite 3.25.0+
		query := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", tableName, uf.OldName, uf.NewName)
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to rename column %s: %w", uf.OldName, err)
		}
	}

	// 3. Add New Fields
	for _, nf := range newFields {
		if nf.IsPrimaryKey {
			return errors.New("adding a primary key to an existing table is not supported in SQLite")
		}
		def := fmt.Sprintf("%s %s", nf.Name, nf.DataType)
		if nf.HasDefault {
			def += " DEFAULT " + nf.DefaultValue
		}
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, def)
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to add column %s: %w", nf.Name, err)
		}
	}

	return nil
}

// InsertData inserts a new record.
func (c *SQLiteClient) InsertData(tableName string, data []FieldData) error {
	if len(data) == 0 {
		return errors.New("no data provided for insertion")
	}

	cols, placeholders, vals := []string{}, []string{}, []interface{}{}
	for _, d := range data {
		cols = append(cols, d.Name)
		placeholders = append(placeholders, "?")
		vals = append(vals, d.Value)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := c.db.Exec(query, vals...)
	return err
}

// UpdateData updates a record by its ID.
func (c *SQLiteClient) UpdateData(tableName string, data []FieldData, id FieldID) error {
	if len(data) == 0 {
		return errors.New("no data provided for update")
	}

	var setClauses []string
	var vals []interface{}

	for _, d := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", d.Name))
		vals = append(vals, d.Value)
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		tableName, strings.Join(setClauses, ", "), id.ColumnName)
	vals = append(vals, id.Value)

	_, err := c.db.Exec(query, vals...)
	return err
}

// DeleteData removes a record by its ID.
func (c *SQLiteClient) DeleteData(tableName string, id FieldID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", tableName, id.ColumnName)
	_, err := c.db.Exec(query, id.Value)
	return err
}

// QueryData retrieves data with optional filtering, sorting, and limiting.
func (c *SQLiteClient) QueryData(tableName string, id *FieldID, limit int, orders []OrderDescription) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s", tableName)
	var args []interface{}

	// 1. Filter
	if id != nil {
		query += fmt.Sprintf(" WHERE %s = ?", id.ColumnName)
		args = append(args, id.Value)
	}

	// 2. Sort
	if len(orders) > 0 {
		var orderParts []string
		for _, o := range orders {
			direction := "ASC"
			if !o.IsAscending {
				direction = "DESC"
			}
			orderParts = append(orderParts, fmt.Sprintf("%s %s", o.ColumnName, direction))
		}
		query += " ORDER BY " + strings.Join(orderParts, ", ")
	}

	// 3. Limit
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Dynamic result parsing
	cols, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		results = append(results, rowMap)
	}

	return results, nil
}
