package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"strings"
	"testing"
	"time"
)

// Create a mock for sql.Result
type MockSQLResult struct {
	mock.Mock
}

func (m *MockSQLResult) LastInsertId() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSQLResult) RowsAffected() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func TestDetectDBOperation(t *testing.T) {
	testCases := []struct {
		query    string
		expected DBOperationType
	}{
		{"SELECT * FROM users", DBOperationSelect},
		{"select * from users", DBOperationSelect},
		{"  SELECT * FROM users", DBOperationSelect},
		{"INSERT INTO users VALUES (?)", DBOperationInsert},
		{"insert into users values (?)", DBOperationInsert},
		{"UPDATE users SET name = ?", DBOperationUpdate},
		{"update users set name = ?", DBOperationUpdate},
		{"DELETE FROM users", DBOperationDelete},
		{"delete from users", DBOperationDelete},
		{"CREATE TABLE users", DBOperationOther},
		{"DROP TABLE users", DBOperationOther},
	}
	
	for _, tc := range testCases {
		result := DetectDBOperation(tc.query)
		assert.Equal(t, tc.expected, result)
	}
}

func TestNewDBLogger(t *testing.T) {
	// Simpler test that only tests the module info
	ctx := context.Background()
	logger := NewDBLogger(ctx, "postgres")
	
	assert.NotNil(t, logger)
	assert.Equal(t, "default", logger.moduleInfo) // Default when no logger in context
	
	// Test with another logger in context
	origLogger := New("testmodule")
	ctx = origLogger.WithContext(ctx)
	
	logger = NewDBLogger(ctx, "postgres")
	assert.Equal(t, "testmodule", logger.moduleInfo)
}

func TestLogDBQuery(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		params   []interface{}
		duration time.Duration
		err      error
		level    string
	}{
		{
			name:     "Select query success",
			query:    "SELECT * FROM users",
			params:   []interface{}{"param1", 123},
			duration: time.Millisecond * 100,
			err:      nil,
			level:    "debug", // SELECT uses debug level
		},
		{
			name:     "Insert query success",
			query:    "INSERT INTO users VALUES (?)",
			params:   []interface{}{"param1"},
			duration: time.Millisecond * 50,
			err:      nil,
			level:    "info", // Other operations use info level
		},
		{
			name:     "Query with error",
			query:    "SELECT * FROM invalid_table",
			params:   nil,
			duration: time.Millisecond * 10,
			err:      errors.New("table not found"),
			level:    "error", // Errors use error level
		},
		{
			name:     "Query with sensitive data",
			query:    "SELECT * FROM users WHERE password = ?",
			params:   []interface{}{"secret_password"},
			duration: time.Millisecond * 15,
			err:      nil,
			level:    "debug",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up a test logger
			buf := &bytes.Buffer{}
			logger := &Logger{
				logger:     zerolog.New(buf),
				moduleInfo: "db-test",
			}
			ctx := logger.WithContext(context.Background())
			
			// Call the function
			LogDBQuery(ctx, tc.query, tc.params, tc.duration, tc.err)
			
			// Parse the log
			logMap := map[string]interface{}{}
			err := json.Unmarshal(buf.Bytes(), &logMap)
			assert.NoError(t, err)
			
			// Check log properties
			assert.Equal(t, tc.level, logMap["level"])
			assert.Equal(t, tc.query, logMap[DBQueryKey])
			assert.Equal(t, float64(tc.duration.Milliseconds()), logMap[DBDurationKey])
			
			// Check params sanitization for sensitive queries
			if tc.params != nil && strings.Contains(tc.query, "password") {
				params, ok := logMap[DBParamsKey].([]interface{})
				assert.True(t, ok)
				assert.Equal(t, "[REDACTED]", params[0])
			}
			
			// Check error handling
			if tc.err != nil {
				assert.Equal(t, tc.err.Error(), logMap["error"])
			}
		})
	}
}

func TestLogDBResult(t *testing.T) {
	testCases := []struct {
		name      string
		operation string
		rowsAff   int64
		lastID    int64
		err       error
		level     string
	}{
		{
			name:      "Insert success",
			operation: "INSERT",
			rowsAff:   1,
			lastID:    123,
			err:       nil,
			level:     "info",
		},
		{
			name:      "Update success",
			operation: "UPDATE",
			rowsAff:   5,
			lastID:    0,
			err:       nil,
			level:     "info",
		},
		{
			name:      "Operation error",
			operation: "DELETE",
			rowsAff:   0,
			lastID:    0,
			err:       errors.New("constraint violation"),
			level:     "error",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock sql.Result
			mockResult := new(MockSQLResult)
			mockResult.On("RowsAffected").Return(tc.rowsAff, nil)
			mockResult.On("LastInsertId").Return(tc.lastID, nil)
			
			// Set up test logger
			buf := &bytes.Buffer{}
			logger := &Logger{
				logger:     zerolog.New(buf),
				moduleInfo: "db-test",
			}
			ctx := logger.WithContext(context.Background())
			
			// Call the function
			LogDBResult(ctx, mockResult, tc.err, tc.operation)
			
			// Parse the log
			logMap := map[string]interface{}{}
			err := json.Unmarshal(buf.Bytes(), &logMap)
			assert.NoError(t, err)
			
			// Check log properties
			assert.Equal(t, tc.level, logMap["level"])
			assert.Equal(t, tc.operation, logMap[DBOperationKey])
			
			if tc.err != nil {
				assert.Equal(t, tc.err.Error(), logMap["error"])
			} else {
				assert.Equal(t, float64(tc.rowsAff), logMap[DBRowsKey])
				if tc.lastID > 0 {
					assert.Equal(t, float64(tc.lastID), logMap["last_insert_id"])
				}
			}
		})
	}
}

func TestLogDBTransaction(t *testing.T) {
	testCases := []struct {
		name  string
		event string
		err   error
		level string
	}{
		{
			name:  "Begin transaction",
			event: "begin",
			err:   nil,
			level: "debug",
		},
		{
			name:  "Commit transaction",
			event: "commit",
			err:   nil,
			level: "debug",
		},
		{
			name:  "Rollback transaction",
			event: "rollback",
			err:   nil,
			level: "debug",
		},
		{
			name:  "Transaction error",
			event: "commit",
			err:   errors.New("deadlock detected"),
			level: "error",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test logger
			buf := &bytes.Buffer{}
			logger := &Logger{
				logger:     zerolog.New(buf),
				moduleInfo: "db-test",
			}
			ctx := logger.WithContext(context.Background())
			
			// Call the function
			LogDBTransaction(ctx, tc.event, tc.err)
			
			// Parse the log
			logMap := map[string]interface{}{}
			err := json.Unmarshal(buf.Bytes(), &logMap)
			assert.NoError(t, err)
			
			// Check log properties
			assert.Equal(t, tc.level, logMap["level"])
			assert.Equal(t, tc.event, logMap["db_transaction_event"])
			
			if tc.err != nil {
				assert.Equal(t, tc.err.Error(), logMap["error"])
			}
		})
	}
}