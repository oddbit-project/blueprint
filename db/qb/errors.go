package qb

import (
	"fmt"
	"reflect"
)

// Simple error wrapper that adds context
type SqlError struct {
	Message string
	Cause   error
	Context map[string]string
}

func (e *SqlError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *SqlError) Unwrap() error {
	return e.Cause
}

// Error constructor
func NewError(message string, cause error) *SqlError {
	return &SqlError{
		Message: message,
		Cause:   cause,
		Context: make(map[string]string),
	}
}

// Add context to error
func (e *SqlError) WithContext(key, value string) *SqlError {
	e.Context[key] = value
	return e
}

// Common error constructors
func InvalidInputError(message string, cause error) error {
	return NewError(fmt.Sprintf("invalid input: %s", message), cause)
}

func ValidationError(message string) error {
	return NewError(fmt.Sprintf("validation failed: %s", message), nil)
}

func StructParsingError(structType reflect.Type, cause error) error {
	return NewError(fmt.Sprintf("failed to parse struct %s", structType.String()), cause)
}

func FieldMappingError(fieldName, dbName string, cause error) error {
	return NewError(fmt.Sprintf("failed to map field %s to column %s", fieldName, dbName), cause)
}

func DialectError(message string, cause error) error {
	return NewError(fmt.Sprintf("dialect error: %s", message), cause)
}

func BatchProcessingError(recordIndex int, cause error) error {
	return NewError(fmt.Sprintf("failed to process record at index %d", recordIndex), cause)
}
