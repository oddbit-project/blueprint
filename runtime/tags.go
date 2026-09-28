package runtime

import (
	"reflect"
	"strings"
)

// splitTag splits a tag value by comma, trimming spaces from each part
func splitTag(tv string) []string {
	parts := strings.Split(tv, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// ParseTagList attempt to parse tag from a list of possible tags
// if a match from the list of possible tags is found, returns the list of options
func ParseTagList(field reflect.StructField, tags []string) []string {
	result := make([]string, 0)
	found := false
	for _, tag := range tags {
		if tv := field.Tag.Get(tag); len(tv) > 0 {
			result = append(result, splitTag(tv)...)
			found = true
		}
		if found {
			break
		}
	}
	return result
}

// ParseTag attempt to parse tag
func ParseTag(field reflect.StructField, tag string) []string {
	if tv := field.Tag.Get(tag); len(tv) > 0 {
		return splitTag(tv)
	}
	return []string{}
}

// MustParseTag attempt to parse tag; if not possible, return default value
func MustParseTag(field reflect.StructField, tag string, defaultValue []string) []string {
	if tv := field.Tag.Get(tag); len(tv) > 0 {
		return splitTag(tv)
	}
	return defaultValue
}
