package clickhouse

import (
	"context"
	"database/sql"
)

// ColumnExists check if a column exists
func ColumnExists(ctx context.Context, client *Client, database string, table string, column string) (bool, error) {
	query := `
        SELECT 1
        FROM system.columns
        WHERE database = ? AND table = ? AND name = ?
        LIMIT 1;
    `
	var col uint8
	err := client.Conn.QueryRow(ctx, query, database, table, column).Scan(&col)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func TableExists(ctx context.Context, client *Client, database, table string) (bool, error) {
	query := `
        SELECT 1
        FROM system.tables
        WHERE database = ? AND name = ?
        LIMIT 1;
    `
	var exists uint8
	err := client.Conn.QueryRow(ctx, query, database, table).Scan(&exists)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func CurrentDatabase(ctx context.Context, client *Client) (string, error) {
	var dbName string
	err := client.Conn.QueryRow(ctx, "SELECT currentDatabase()").Scan(&dbName)
	if err != nil {
		return "", err
	}
	return dbName, nil
}
