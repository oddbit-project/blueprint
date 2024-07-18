package pgsql

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
)

const (
	SchemaDefault = "public"

	TblTypeTable        = "BASE TABLE"
	TblTypeView         = "VIEW"
	TblTypeForeignTable = "FOREIGN TABLE"
	TblTypeLocal        = "LOCAL TEMPORARY"
)

// GetServerVersion fetch postgresql version
func GetServerVersion(db *sqlx.DB) (string, error) {
	var result string
	row := db.QueryRow("SELECT version()")
	err := row.Scan(&result)
	return result, err
}

// TableExists returns true if specified table exists
func TableExists(db *sqlx.DB, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, TblTypeTable, tableName, schema)
}

// ViewExists returns true if specified view exists
func ViewExists(db *sqlx.DB, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, TblTypeView, tableName, schema)
}

// ForeignTableExists returns true if specified foreign table exists
func ForeignTableExists(db *sqlx.DB, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, TblTypeForeignTable, tableName, schema)
}

// TableObjectExists checks if given table-like object exists
func TableObjectExists(db *sqlx.DB, tableType string, tableName string, schema string) (bool, error) {
	var record string
	qry := "SELECT table_schema FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2 AND table_type=$3"
	if err := db.QueryRow(qry, schema, tableName, tableType).Scan(&record); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return len(record) > 0, nil
}
