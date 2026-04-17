package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
)

const (
	TblTypeTable = "table"
	TblTypeView  = "view"
)

// GetServerVersion fetch sqlite version
func GetServerVersion(db *sqlx.DB, ctx context.Context) (string, error) {
	var result string
	err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&result)
	return result, err
}

// TableExists returns true if specified table exists
func TableExists(ctx context.Context, client *db.SqlClient, tableName string) (bool, error) {
	return dbObjectExists(ctx, client.Db(), TblTypeTable, tableName)
}

// ColumnExists check if a column exists
func ColumnExists(ctx context.Context, client *db.SqlClient, tableName string, columnName string) (bool, error) {
	rows, err := client.Db().QueryContext(ctx, "SELECT name FROM pragma_table_info(?)", tableName)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ViewExists returns true if specified view exists
func ViewExists(ctx context.Context, client *db.SqlClient, tableName string) (bool, error) {
	return dbObjectExists(ctx, client.Db(), TblTypeView, tableName)
}

// dbObjectExists checks if given table-like object exists in sqlite_master
func dbObjectExists(ctx context.Context, db *sqlx.DB, objectType string, name string) (bool, error) {
	var record string
	qry := "SELECT name FROM sqlite_master WHERE type=? AND name=?"
	if err := db.QueryRowContext(ctx, qry, objectType, name).Scan(&record); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return len(record) > 0, nil
}
