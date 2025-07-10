package qb

import (
	"fmt"
	"reflect"
	"regexp"
)

// Simple validation helpers
func validateNotNil(value interface{}, name string) error {
	if value == nil {
		return ValidationError(fmt.Sprintf("%s cannot be nil", name))
	}
	return nil
}

func validateNotEmpty(value, name string) error {
	if value == "" {
		return ValidationError(fmt.Sprintf("%s cannot be empty", name))
	}
	return nil
}

func validateSliceNotEmpty(value interface{}, name string) error {
	if value == nil {
		return ValidationError(fmt.Sprintf("%s cannot be nil", name))
	}

	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return ValidationError(fmt.Sprintf("%s must be a slice", name))
	}

	if v.Len() == 0 {
		return ValidationError(fmt.Sprintf("%s cannot be empty", name))
	}

	return nil
}

func validateStructType(t reflect.Type, name string) error {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return ValidationError(fmt.Sprintf("%s must be a struct, got %s", name, t.Kind().String()))
	}

	return nil
}

func validateTableName(tableName string) error {
	if err := validateNotEmpty(tableName, "table name"); err != nil {
		return err
	}

	match, err := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, tableName)
	if err != nil {
		return ValidationError(fmt.Sprintf("error checking table name '%s' characters", tableName))
	}
	if !match {
		return ValidationError(fmt.Sprintf("table name '%s' contains invalid characters", tableName))
	}

	return nil
}
