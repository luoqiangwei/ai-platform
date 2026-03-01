package utils

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type PostgresClient struct {
	db *sql.DB
}

// OpenPostgres initializes a PostgreSQL connection.
func OpenPostgres(dsn string, config string) (*PostgresClient, error) {
	fullDSN := dsn
	if config != "" {
		fullDSN = dsn + " " + config
	}
	db, err := sql.Open("postgres", fullDSN)
	if err != nil {
		return nil, err
	}
	return &PostgresClient{db: db}, db.Ping()
}

// Close closes the database connection.
func (c *PostgresClient) Close(sync map[string]bool) error {
	// Note: PostgreSQL handles WAL synchronization internally;
	// sync parameters are accepted here to satisfy the interface.
	return c.db.Close()
}

// CreateTable creates a table based on FieldDescription.
func (c *PostgresClient) CreateTable(tableName string, fields []FieldDescription) error {
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
			if f.IsAutoIncrement {
				// Use SERIAL for auto-incrementing primary keys in Postgres
				def = fmt.Sprintf("%s SERIAL PRIMARY KEY", f.Name)
			} else {
				def += " PRIMARY KEY"
			}
		} else {
			if f.HasDefault {
				def += " DEFAULT " + f.DefaultValue
			}
		}
		defs = append(defs, def)
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(defs, ", "))
	_, err := c.db.Exec(query)
	return err
}

// DropTable removes the table from the database.
func (c *PostgresClient) DropTable(tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	_, err := c.db.Exec(query)
	return err
}

// UpdateTable handles column removal, renaming, and addition.
func (c *PostgresClient) UpdateTable(tableName string, removeFields []string, updateFields []FieldUpdateDescription, newFields []FieldDescription) error {
	// Postgres doesn't easily allow checking if a column is a PK without querying pg_attribute.
	// For simplicity, we execute the commands. If it violates constraints, the DB will return an error.

	// 1. Remove fields
	for _, field := range removeFields {
		query := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, field)
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to remove column %s: %w", field, err)
		}
	}

	// 2. Update (Rename) fields
	for _, uf := range updateFields {
		query := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", tableName, uf.OldName, uf.NewName)
		if _, err := c.db.Exec(query); err != nil {
			return fmt.Errorf("failed to rename column %s: %w", uf.OldName, err)
		}
	}

	// 3. Add new fields
	for _, nf := range newFields {
		if nf.IsPrimaryKey {
			return errors.New("adding a primary key to an existing table is not supported via this method")
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

// InsertData inserts a single record into the table.
func (c *PostgresClient) InsertData(tableName string, data []FieldData) error {
	cols, placeholders, vals := []string{}, []string{}, []interface{}{}
	for i, d := range data {
		cols = append(cols, d.Name)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		vals = append(vals, d.Value)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := c.db.Exec(query, vals...)
	return err
}

// UpdateData updates a record based on the provided FieldID.
func (c *PostgresClient) UpdateData(tableName string, data []FieldData, id FieldID) error {
	var setClauses []string
	var vals []interface{}

	idx := 1
	for _, d := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", d.Name, idx))
		vals = append(vals, d.Value)
		idx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
		tableName, strings.Join(setClauses, ", "), id.ColumnName, idx)
	vals = append(vals, id.Value)

	_, err := c.db.Exec(query, vals...)
	return err
}

// DeleteData removes a record based on the provided FieldID.
func (c *PostgresClient) DeleteData(tableName string, id FieldID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", tableName, id.ColumnName)
	_, err := c.db.Exec(query, id.Value)
	return err
}

// QueryData performs a SELECT query with filtering, sorting, and limit.
func (c *PostgresClient) QueryData(tableName string, id *FieldID, limit int, orders []OrderDescription) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s", tableName)
	var args []interface{}

	// 1. Where Clause
	if id != nil {
		query += fmt.Sprintf(" WHERE %s = $1", id.ColumnName)
		args = append(args, id.Value)
	}

	// 2. Order By
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

	// Parse Result Set
	cols, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		// Create a slice of interface{} to hold the row data
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Map row data to column names
		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		results = append(results, rowMap)
	}

	return results, nil
}
