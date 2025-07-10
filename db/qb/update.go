package qb

import (
	"fmt"
)

// BuildSQLUpdateWhere generates UPDATE with complex WHERE support
// Deprecated: Use Update() for new code
func (s *SqlBuilder) BuildSQLUpdateWhere(tableName string, record any, where WhereClause, options *UpdateOptions) (string, []any, error) {
	return s.Update(tableName, record).
		Where(where).
		WithOptions(options).
		Build()
}

// BuildSQLUpdate generates an UPDATE SQL statement with complex WHERE clause support
// Deprecated: Use Update() for new code
func (s *SqlBuilder) BuildSQLUpdate(tableName string, record any, where WhereClause, options *UpdateOptions) (string, []any, error) {
	return s.Update(tableName, record).
		Where(where).
		WithOptions(options).
		Build()
}

// BuildSQLUpdateByID generates an UPDATE SQL statement using ID as the WHERE condition
// Deprecated: Use Update() for new code
func (s *SqlBuilder) BuildSQLUpdateByID(tableName string, data any, id any, options *UpdateOptions) (string, []any, error) {
	sql, args, err := s.Update(tableName, data).
		WhereEq("id", id).
		WithOptions(options).
		Build()
	if err != nil {
		// Add more context to the error
		if sqlErr, ok := err.(*SqlError); ok {
			return "", nil, sqlErr.WithContext("id_value", fmt.Sprintf("%v", id))
		}
		return "", nil, err
	}

	return sql, args, nil
}
