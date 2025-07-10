package qb

import (
	"fmt"
	"github.com/oddbit-project/blueprint/db/field"
	"reflect"
	"slices"
	"strings"
)

// UpdateBuilder provides a fluent interface for building UPDATE statements with complex WHERE clauses
type UpdateBuilder struct {
	sqlBuilder *SqlBuilder
	tableName  string
	record     any
	where      WhereClause
	options    *UpdateOptions
}

// Update creates a new UpdateBuilder instance
func (s *SqlBuilder) Update(tableName string, record any) *UpdateBuilder {
	return &UpdateBuilder{
		sqlBuilder: s,
		tableName:  tableName,
		record:     record,
		options:    nil, // Will use defaults
	}
}

// HasReturnFields returns true if clause has return fields
func (b *UpdateBuilder) HasReturnFields() bool {
	if b.options == nil {
		return false
	}
	return len(b.options.ReturningFields) > 0
}

// Where sets the WHERE clause for the update
func (b *UpdateBuilder) Where(clause WhereClause) *UpdateBuilder {
	b.where = clause
	return b
}

// WithOptions sets the update options
func (b *UpdateBuilder) WithOptions(options *UpdateOptions) *UpdateBuilder {
	b.options = options
	return b
}

// Build generates the final SQL statement and arguments
func (b *UpdateBuilder) Build() (string, []any, error) {
	if b.where == nil {
		return "", nil, ValidationError("WHERE clause is required for UPDATE")
	}
	return b.buildUpdateSQL()
}

// buildUpdateSQL contains the core UPDATE SQL generation logic
func (b *UpdateBuilder) buildUpdateSQL() (string, []any, error) {
	// Use default options if none provided
	opts := DefaultUpdateOptions()
	if b.options != nil {
		opts = b.options
	}

	// Input validation
	if err := validateNotNil(b.record, "record"); err != nil {
		return "", nil, err
	}

	if err := validateNotEmpty(b.tableName, "table name"); err != nil {
		return "", nil, err
	}

	v := reflect.ValueOf(b.record)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", nil, InvalidInputError("record pointer cannot be nil", nil)
		}
		v = v.Elem()
	}

	// Validate that we have a struct
	if err := validateStructType(v.Type(), "record"); err != nil {
		return "", nil, err
	}

	metadata, err := field.GetStructMeta(v.Type())
	if err != nil {
		return "", nil, StructParsingError(v.Type(), err)
	}

	quotedTableName, err := b.sqlBuilder.dialect.Table(b.tableName)
	if err != nil {
		return "", nil, DialectError(fmt.Sprintf("failed to quote table name '%s'", b.tableName), err)
	}
	tableName := quotedTableName

	var validDbFields = make([]string, len(metadata))
	var setClauses []string
	var values []any
	count := 1

	// Build SET clause
	for i, meta := range metadata {

		// store name to validate possible RETURNING clause
		validDbFields[i] = meta.DbName

		// Skip auto fields unless explicitly requested
		if meta.Auto && !opts.UpdateAutoFields {
			continue
		}

		// Handle field exclusion/inclusion
		if opts.ShouldSkipField(meta.Name, meta.DbName) {
			continue
		}

		// Get field value by name
		fieldValue := v.FieldByName(meta.Name)
		if !fieldValue.IsValid() {
			return "", nil, FieldMappingError(meta.Name, meta.DbName,
				fmt.Errorf("field not found in struct"))
		}

		// Handle omitnil - skip if field is nil pointer
		if meta.OmitNil && fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			continue
		}

		// Handle omitempty - skip if field is empty
		if meta.OmitEmpty && fieldValue.IsZero() {
			continue
		}

		// Handle zero value exclusion
		if !opts.IncludeZeroValues && fieldValue.IsZero() {
			continue
		}

		// Standard field handling
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", b.sqlBuilder.dialect.Field(meta.DbName), b.sqlBuilder.dialect.Placeholder(count)))
		count++

		// Extract the actual value
		var actualValue any
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				actualValue = nil
			} else {
				actualValue = fieldValue.Elem().Interface()
			}
		} else {
			actualValue = fieldValue.Interface()
		}

		values = append(values, actualValue)
	}

	if len(setClauses) == 0 {
		return "", nil, ValidationError("no updatable fields found")
	}

	// Build WHERE clause using the new system
	whereSql, whereValues, _ := b.where.Build(b.sqlBuilder.dialect, count)

	// Construct final SQL
	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		tableName,
		strings.Join(setClauses, ", "),
		whereSql)

	// Add RETURNING clause if specified
	if len(opts.ReturningFields) > 0 {
		var returningFields []string
		for _, field := range opts.ReturningFields {
			if field == "*" {
				returningFields = append(returningFields, "*")
			} else {
				if !slices.Contains(returningFields, field) {
					return "", nil, ValidationError(fmt.Sprintf("invalid return field '%s'", field))
				}

				returningFields = append(returningFields, b.sqlBuilder.dialect.Field(field))
			}
		}
		sql += fmt.Sprintf(" RETURNING %s", strings.Join(returningFields, ", "))
	}

	// Combine all values
	allValues := append(values, whereValues...)

	return sql, allValues, nil
}

// Convenience methods for common WHERE patterns

// WhereEq adds a simple equality condition
func (b *UpdateBuilder) WhereEq(field string, value any) *UpdateBuilder {
	return b.Where(Eq(field, value))
}

// WhereAnd adds an AND condition
func (b *UpdateBuilder) WhereAnd(conditions ...WhereClause) *UpdateBuilder {
	return b.Where(And(conditions...))
}

// WhereOr adds an OR condition
func (b *UpdateBuilder) WhereOr(conditions ...WhereClause) *UpdateBuilder {
	return b.Where(Or(conditions...))
}

// WhereIn adds an IN condition
func (b *UpdateBuilder) WhereIn(field string, values ...any) *UpdateBuilder {
	return b.Where(In(field, values...))
}

// WhereBetween adds a BETWEEN condition
func (b *UpdateBuilder) WhereBetween(field string, start, end any) *UpdateBuilder {
	return b.Where(Between(field, start, end))
}

// WhereNull adds an IS NULL condition
func (b *UpdateBuilder) WhereNull(field string) *UpdateBuilder {
	return b.Where(IsNull(field))
}

// WhereNotNull adds an IS NOT NULL condition
func (b *UpdateBuilder) WhereNotNull(field string) *UpdateBuilder {
	return b.Where(IsNotNull(field))
}

// WhereLiteral adds a literal SQL condition with placeholders
func (b *UpdateBuilder) WhereLiteral(sql string, values ...any) *UpdateBuilder {
	return b.Where(Literal(sql, values...))
}

// WhereRaw adds raw SQL without any processing
func (b *UpdateBuilder) WhereRaw(sql string) *UpdateBuilder {
	return b.Where(Raw(sql))
}

// Options configuration methods

// ExcludeFields excludes specific fields from the update
func (b *UpdateBuilder) ExcludeFields(fields ...string) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.ExcludeFields = append(b.options.ExcludeFields, fields...)
	return b
}

// IncludeFields includes only specific fields in the update
func (b *UpdateBuilder) IncludeFields(fields ...string) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.IncludeFields = append(b.options.IncludeFields, fields...)
	return b
}

// IncludeZeroValues includes fields with zero values
func (b *UpdateBuilder) IncludeZeroValues(include bool) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.IncludeZeroValues = include
	return b
}

// UpdateAutoFields includes auto fields in the update
func (b *UpdateBuilder) UpdateAutoFields(update bool) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.UpdateAutoFields = update
	return b
}

// Convenience methods that combine common operations

// ByID sets WHERE id = value (common pattern)
func (b *UpdateBuilder) ByID(id any) *UpdateBuilder {
	return b.WhereEq("id", id)
}

// Where sets a complex WHERE clause and immediately builds
func (b *UpdateBuilder) WhereAndBuild(conditions ...WhereClause) (string, []any, error) {
	return b.WhereAnd(conditions...).Build()
}

// Set allows setting WHERE and OPTIONS in one call for simple cases
func (b *UpdateBuilder) Set(where WhereClause, options *UpdateOptions) *UpdateBuilder {
	b.where = where
	b.options = options
	return b
}

// RETURNING configuration methods

// Returning sets the fields to return after the update
func (b *UpdateBuilder) Returning(fields ...string) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.ReturningFields = fields
	return b
}

// ReturningAll returns all fields after the update (using *)
func (b *UpdateBuilder) ReturningAll() *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.ReturningFields = []string{"*"}
	return b
}

// AddReturning adds additional fields to the RETURNING clause
func (b *UpdateBuilder) AddReturning(fields ...string) *UpdateBuilder {
	if b.options == nil {
		b.options = DefaultUpdateOptions()
	}
	b.options.ReturningFields = append(b.options.ReturningFields, fields...)
	return b
}
