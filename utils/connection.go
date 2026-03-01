package utils

// DBClient defines the standard behavior for our database tools.
type DBClient interface {
	Close(syncConfig map[string]bool) error
	CreateTable(tableName string, fields []FieldDescription) error
	DropTable(tableName string) error
	UpdateTable(tableName string, removeFields []string, updateFields []FieldUpdateDescription, newFields []FieldDescription) error
	InsertData(tableName string, data []FieldData) error
	UpdateData(tableName string, data []FieldData, id FieldID) error
	DeleteData(tableName string, id FieldID) error
	QueryData(tableName string, id *FieldID, limit int, orders []OrderDescription) ([]map[string]interface{}, error)
}

// Data structures for schema and DML operations
type FieldDescription struct {
	Name            string
	DataType        string
	IsPrimaryKey    bool
	IsAutoIncrement bool
	HasDefault      bool
	DefaultValue    string
}

type FieldUpdateDescription struct {
	OldName string
	NewName string
}

type FieldData struct {
	Name  string
	Value interface{}
}

type FieldID struct {
	ColumnName string
	Value      interface{}
}

type OrderDescription struct {
	ColumnName  string
	IsAscending bool
}

// Common helper to convert Go types to SQL type strings
func GoTypeToSQLType(value interface{}) string {
	switch value.(type) {
	case int, int64:
		return "INTEGER"
	case float64:
		return "REAL"
	case bool:
		return "BOOLEAN"
	default:
		return "TEXT"
	}
}
