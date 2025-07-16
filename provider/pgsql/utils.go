package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
)

const (
	SchemaDefault = "public"

	TblTypeTable        = "BASE TABLE"
	TblTypeView         = "VIEW"
	TblTypeForeignTable = "FOREIGN TABLE"
	TblTypeLocal        = "LOCAL TEMPORARY"
)

// GetServerVersion fetch postgresql version
func GetServerVersion(db *sqlx.DB, ctx context.Context) (string, error) {
	var result string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&result)
	return result, err
}

// TableExists returns true if specified table exists
func TableExists(ctx context.Context, client *db.SqlClient, tableName string, schema string) (bool, error) {
	return dbObjectExists(ctx, client.Db(), TblTypeTable, tableName, schema)
}

// ColumnExists check if a column exists
func ColumnExists(ctx context.Context, client *db.SqlClient, tableName string, columnName string, schema string) (bool, error) {
	query := `
        SELECT column_name 
        FROM information_schema.columns 
        WHERE table_schema = $1 AND table_name = $2 AND column_name = $3;
    `
	var col string
	err := client.Db().QueryRowContext(ctx, query, schema, tableName, columnName).Scan(&col)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ViewExists returns true if specified view exists
func ViewExists(ctx context.Context, client *db.SqlClient, tableName string, schema string) (bool, error) {
	return dbObjectExists(ctx, client.Db(), TblTypeView, tableName, schema)
}

// ForeignTableExists returns true if specified foreign table exists
func ForeignTableExists(ctx context.Context, client *db.SqlClient, tableName string, schema string) (bool, error) {
	return dbObjectExists(ctx, client.Db(), TblTypeForeignTable, tableName, schema)
}

// dbObjectExists checks if given table-like object exists
func dbObjectExists(ctx context.Context, db *sqlx.DB, tableType string, tableName string, schema string) (bool, error) {
	var record string
	qry := "SELECT table_schema FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2 AND table_type=$3"
	if err := db.QueryRowContext(ctx, qry, schema, tableName, tableType).Scan(&record); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return len(record) > 0, nil
}
