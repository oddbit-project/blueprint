package log

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Database logging context keys
const (
	DBQueryKey      = "db_query"
	DBParamsKey     = "db_params"
	DBDurationKey   = "db_duration_ms"
	DBRowsKey       = "db_rows_affected"
	DBTableKey      = "db_table"
	DBOperationKey  = "db_operation"
	DBComponentKey  = "db_component"
)

// DBOperationType defines the type of database operation
type DBOperationType string

// Database operation types
const (
	DBOperationSelect DBOperationType = "SELECT"
	DBOperationInsert DBOperationType = "INSERT"
	DBOperationUpdate DBOperationType = "UPDATE"
	DBOperationDelete DBOperationType = "DELETE"
	DBOperationOther  DBOperationType = "OTHER"
)

// DetectDBOperation detects the operation type from an SQL query
func DetectDBOperation(query string) DBOperationType {
	query = strings.TrimSpace(strings.ToUpper(query))
	
	if strings.HasPrefix(query, "SELECT") {
		return DBOperationSelect
	} else if strings.HasPrefix(query, "INSERT") {
		return DBOperationInsert
	} else if strings.HasPrefix(query, "UPDATE") {
		return DBOperationUpdate
	} else if strings.HasPrefix(query, "DELETE") {
		return DBOperationDelete
	}
	
	return DBOperationOther
}

// NewDBLogger creates a new logger with database component information
func NewDBLogger(ctx context.Context, component string) *Logger {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("db")
	}
	
	return logger.
		WithField(DBComponentKey, component)
}

// LogDBQuery logs a database query with timing information
func LogDBQuery(ctx context.Context, query string, params []interface{}, duration time.Duration, err error) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("db")
	}
	
	operation := DetectDBOperation(query)
	
	fields := map[string]interface{}{
		DBQueryKey:      query,
		DBDurationKey:   duration.Milliseconds(),
		DBOperationKey:  string(operation),
	}
	
	// Add parameters if available, but sanitize sensitive values
	if params != nil && len(params) > 0 {
		safeParams := make([]interface{}, len(params))
		for i, param := range params {
			// Check if parameter might contain sensitive information
			if p, ok := param.(string); ok {
				if strings.Contains(strings.ToLower(query), "password") ||
					strings.Contains(strings.ToLower(query), "token") ||
					strings.Contains(strings.ToLower(query), "secret") ||
					strings.Contains(strings.ToLower(query), "key") {
					// Mask potential sensitive data
					safeParams[i] = "[REDACTED]"
				} else {
					safeParams[i] = p
				}
			} else {
				safeParams[i] = param
			}
		}
		fields[DBParamsKey] = safeParams
	}
	
	logMsg := fmt.Sprintf("DB %s query", operation)
	
	if err != nil {
		// For database errors, include error information
		logger.Error(err, logMsg, fields)
	} else {
		// Normal query logging
		if operation == DBOperationSelect {
			// For SELECT operations, use Debug level to avoid excessive logs
			logger.Debug(logMsg, fields)
		} else {
			// For modifying operations, use Info level
			logger.Info(logMsg, fields)
		}
	}
}

// LogDBResult logs the result of a database operation
func LogDBResult(ctx context.Context, result sql.Result, err error, operation string) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("db")
	}
	
	fields := map[string]interface{}{
		DBOperationKey: operation,
	}
	
	if result != nil {
		if rowsAffected, err := result.RowsAffected(); err == nil {
			fields[DBRowsKey] = rowsAffected
		}
		
		if lastID, err := result.LastInsertId(); err == nil && lastID > 0 {
			fields["last_insert_id"] = lastID
		}
	}
	
	if err != nil {
		logger.Error(err, fmt.Sprintf("DB %s operation failed", operation), fields)
	} else {
		logger.Info(fmt.Sprintf("DB %s operation completed", operation), fields)
	}
}

// LogDBTransaction logs database transaction events
func LogDBTransaction(ctx context.Context, event string, err error) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("db")
	}
	
	fields := map[string]interface{}{
		"db_transaction_event": event,
	}
	
	if err != nil {
		logger.Error(err, fmt.Sprintf("DB transaction %s failed", event), fields)
	} else {
		logger.Debug(fmt.Sprintf("DB transaction %s", event), fields)
	}
}