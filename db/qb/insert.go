package qb

import (
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/db/field"
	"reflect"
	"strings"
)

// Pre-compute field information for optimization
type columnInfo struct {
	meta      field.Metadata
	dbName    string
	fieldName string
	index     int // Field index for faster lookups
}

// BuildSQLInsert generates an INSERT SQL statement and parameter list
func (s *SqlBuilder) BuildSQLInsert(tableName string, data any) (string, []any, error) {
	// Input validation
	if err := validateNotNil(data, "data"); err != nil {
		return "", nil, err
	}

	if err := validateNotEmpty(tableName, "table name"); err != nil {
		return "", nil, err
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", nil, InvalidInputError("data pointer cannot be nil", nil)
		}
		v = v.Elem()
	}

	// Validate that we have a struct
	if err := validateStructType(v.Type(), "data"); err != nil {
		return "", nil, err
	}

	var columns []string
	var placeholders []string
	var values []any

	metadata, err := field.GetStructMeta(v.Type())
	if err != nil {
		return "", nil, StructParsingError(v.Type(), err)
	}

	quotedTableName, err := s.dialect.Table(tableName)
	if err != nil {
		return "", nil, DialectError(fmt.Sprintf("failed to quote table name '%s'", tableName), err)
	}
	tableName = quotedTableName

	count := 1
	for _, meta := range metadata {
		// Skip auto-generated fields for insert
		if meta.Auto {
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

		// Standard field handling
		columns = append(columns, s.dialect.Field(meta.DbName))
		placeholders = append(placeholders, s.dialect.Placeholder(count))
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

	if len(columns) == 0 {
		return "", nil, ValidationError("no insertable fields found")
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return sql, values, nil
}

// BuildSQLInsertReturning
func (s *SqlBuilder) BuildSQLInsertReturning(tableName string, data any, returningFields []string) (string, []any, error) {
	if len(returningFields) == 0 {
		return "", nil, errors.New("empty return fields")
	}
	query, args, err := s.BuildSQLInsert(tableName, data)
	if err != nil {
		return "", nil, err
	}

	returnFields := make([]string, len(returningFields))
	for i, field := range returningFields {
		returnFields[i] = s.dialect.Field(field)
	}
	query = fmt.Sprintf("%s RETURNING %s", query, strings.Join(returnFields, ", "))
	return query, args, nil
}

// BuildSQLBatchInsert generates a batch INSERT SQL statement for multiple records
//
// Important behavior notes:
// - All records must have the same struct type
// - The first record determines which columns to include based on omitnil/omitempty rules
// - All subsequent records must have the same columns (same omit behavior)
// - If any record has different columns, an error is returned
// - Fields with custom mappers are skipped (not supported)
// - Auto-generated fields are excluded from the INSERT
func (s *SqlBuilder) BuildSQLBatchInsert(tableName string, data []any) (string, []any, error) {
	// Input validation
	if err := validateNotEmpty(tableName, "table name"); err != nil {
		return "", nil, err
	}

	if data == nil {
		return "", nil, ValidationError("data cannot be nil")
	}

	if err := validateSliceNotEmpty(data, "data"); err != nil {
		return "", nil, err
	}

	// Use the first record to determine the structure AND the actual columns
	if data[0] == nil {
		return "", nil, BatchProcessingError(0, fmt.Errorf("first record cannot be nil"))
	}

	firstRecord := reflect.ValueOf(data[0])
	if firstRecord.Kind() == reflect.Ptr {
		if firstRecord.IsNil() {
			return "", nil, BatchProcessingError(0, fmt.Errorf("first record cannot be nil"))
		}
		firstRecord = firstRecord.Elem()
	}

	// Validate that we have a struct
	if err := validateStructType(firstRecord.Type(), "first record"); err != nil {
		return "", nil, BatchProcessingError(0, err)
	}

	metadata, err := field.GetStructMeta(firstRecord.Type())
	if err != nil {
		return "", nil, StructParsingError(firstRecord.Type(), err)
	}

	quotedTableName, err := s.dialect.Table(tableName)
	if err != nil {
		return "", nil, DialectError(fmt.Sprintf("failed to quote table name '%s'", tableName), err)
	}
	tableName = quotedTableName

	// Create maps for O(1) lookups
	metadataByName := make(map[string]field.Metadata, len(metadata))
	metadataByIndex := make([]field.Metadata, len(metadata))
	for i, meta := range metadata {
		metadataByName[meta.Name] = meta
		metadataByIndex[i] = meta
	}

	// Determine which fields to include based on the first record
	var columnsToInclude []columnInfo
	var columnNames []string
	includedFieldsSet := make(map[string]bool) // For O(1) lookup

	// Pre-compute omitted fields for validation (only non-auto fields)
	var omittedFields []field.Metadata

	// step 1: analyze structure from first record
	for i, meta := range metadata {
		if meta.Auto {
			continue
		}

		fieldValue := firstRecord.FieldByName(meta.Name)
		if !fieldValue.IsValid() {
			return "", nil, FieldMappingError(meta.Name, meta.DbName,
				fmt.Errorf("field not found in first record"))
		}

		// Check if this field should be included based on omitnil/omitempty
		shouldInclude := true
		if meta.OmitNil && fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			shouldInclude = false
		} else if meta.OmitEmpty && fieldValue.IsZero() {
			shouldInclude = false
		}

		if shouldInclude {
			columnsToInclude = append(columnsToInclude, columnInfo{
				meta:      meta,
				dbName:    s.dialect.Field(meta.DbName),
				fieldName: meta.Name,
				index:     i,
			})
			columnNames = append(columnNames, s.dialect.Field(meta.DbName))
			includedFieldsSet[meta.Name] = true
		} else {
			omittedFields = append(omittedFields, meta)
		}
	}

	if len(columnsToInclude) == 0 {
		return "", nil, ValidationError("no insertable fields found")
	}

	var allValues []any
	var valuePlaceholders []string
	count := 1

	// Step 2: Process each record
	for i, record := range data {
		if record == nil {
			return "", nil, BatchProcessingError(i, fmt.Errorf("record cannot be nil"))
		}

		v := reflect.ValueOf(record)
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return "", nil, BatchProcessingError(i, fmt.Errorf("record cannot be nil"))
			}
			v = v.Elem()
		}

		// Validate that all records have the same struct type
		if v.Type() != firstRecord.Type() {
			return "", nil, BatchProcessingError(i,
				fmt.Errorf("record type %s does not match first record type %s", v.Type().String(), firstRecord.Type().String()))
		}

		var recordValues []any
		var recordPlaceholders []string

		// Process only the columns that were included from the first record
		for _, col := range columnsToInclude {
			fieldValue := v.FieldByName(col.fieldName)
			if !fieldValue.IsValid() {
				return "", nil, BatchProcessingError(i,
					FieldMappingError(col.fieldName, col.meta.DbName, fmt.Errorf("field not found in record")))
			}

			// Check if this field would be omitted based on omitnil/omitempty
			wouldOmit := false
			if col.meta.OmitNil && fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
				wouldOmit = true
			} else if col.meta.OmitEmpty && fieldValue.IsZero() {
				wouldOmit = true
			}

			// If this field would be omitted but wasn't in the first record, that's an error
			if wouldOmit {
				return "", nil, BatchProcessingError(i,
					fmt.Errorf("field '%s' has inconsistent omit behavior: included in first record but would be omitted in record %d", col.fieldName, i+1))
			}

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

			recordValues = append(recordValues, actualValue)
			recordPlaceholders = append(recordPlaceholders, s.dialect.Placeholder(count))
			count++
		}

		// Check omitted fields for consistency (optimized - no nested loops)
		for _, meta := range omittedFields {
			fieldValue := v.FieldByName(meta.Name)
			if !fieldValue.IsValid() {
				continue
			}

			// Check if it would also be omitted in this record
			wouldOmit := false
			if meta.OmitNil && fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
				wouldOmit = true
			} else if meta.OmitEmpty && fieldValue.IsZero() {
				wouldOmit = true
			}

			if !wouldOmit {
				// This field was omitted in first record but not in this one - error!
				return "", nil, BatchProcessingError(i,
					fmt.Errorf("field '%s' has inconsistent omit behavior: omitted in first record but would be included in record %d", meta.Name, i+1))
			}
		}

		allValues = append(allValues, recordValues...)
		valuePlaceholders = append(valuePlaceholders, fmt.Sprintf("(%s)", strings.Join(recordPlaceholders, ", ")))
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		tableName,
		strings.Join(columnNames, ", "),
		strings.Join(valuePlaceholders, ", "))

	return sql, allValues, nil
}
