package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SchemaDefault = "public"

	TblTypeTable        = "BASE TABLE"
	TblTypeView         = "VIEW"
	TblTypeForeignTable = "FOREIGN TABLE"
	TblTypeLocal        = "LOCAL TEMPORARY"
)

// GetServerVersion fetch postgresql version
func GetServerVersion(db *pgxpool.Conn, ctx context.Context) (string, error) {
	var result string
	row := db.QueryRow(ctx, "SELECT version()")
	err := row.Scan(&result)
	return result, err
}

// TableExists returns true if specified table exists
func TableExists(db *pgxpool.Conn, ctx context.Context, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, ctx, TblTypeTable, tableName, schema)
}

// ViewExists returns true if specified view exists
func ViewExists(db *pgxpool.Conn, ctx context.Context, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, ctx, TblTypeView, tableName, schema)
}

// ForeignTableExists returns true if specified foreign table exists
func ForeignTableExists(db *pgxpool.Conn, ctx context.Context, tableName string, schema string) (bool, error) {
	return TableObjectExists(db, ctx, TblTypeForeignTable, tableName, schema)
}

// TableObjectExists checks if given table-like object exists
func TableObjectExists(db *pgxpool.Conn, ctx context.Context, tableType string, tableName string, schema string) (bool, error) {
	var record string
	qry := "SELECT table_schema FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2 AND table_type=$3"
	if err := db.QueryRow(ctx, qry, schema, tableName, tableType).Scan(&record); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return len(record) > 0, nil
}
