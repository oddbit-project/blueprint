package qb

import (
	"fmt"
	"strconv"
	"strings"
)

type SqlDialect struct {
	PlaceHolderFragment   string
	IncludePlaceholderNum bool
	QuoteTable            string
	QuoteField            string
	QuoteSchema           string
	QuoteDatabase         string
	QuoteSeparator        string
}

func DefaultSqlDialect() SqlDialect {
	return SqlDialect{
		PlaceHolderFragment:   "?",
		IncludePlaceholderNum: false,
		QuoteTable:            `"%s"`,
		QuoteField:            `"%s"`,
		QuoteSchema:           `"%s"`,
		QuoteDatabase:         `"%s"`,
		QuoteSeparator:        `.`,
	}
}

func (d SqlDialect) Placeholder(count int) string {
	if count >= 0 {
		if d.IncludePlaceholderNum {
			return d.PlaceHolderFragment + strconv.Itoa(count)
		}
	}
	return d.PlaceHolderFragment
}

func (d SqlDialect) Table(name string) (string, error) {
	if name == "" {
		return "", ValidationError("table name cannot be empty")
	}

	if strings.ContainsRune(name, '.') {
		parts := strings.Split(name, ".")
		if len(parts) != 2 {
			return "", ValidationError(
				fmt.Sprintf("invalid table name format '%s': expected 'schema.table'", name))
		}

		if parts[0] == "" {
			return "", ValidationError("schema name cannot be empty")
		}

		if parts[1] == "" {
			return "", ValidationError("table name cannot be empty")
		}

		return d.TableSchema(parts[0], parts[1]), nil
	}
	return fmt.Sprintf(d.QuoteTable, name), nil
}

func (d SqlDialect) TableSchema(schema, name string) string {
	return fmt.Sprintf(d.QuoteSchema+d.QuoteSeparator+d.QuoteTable, schema, name)
}

func (d SqlDialect) Field(name string) string {
	return fmt.Sprintf(d.QuoteField, name)
}
