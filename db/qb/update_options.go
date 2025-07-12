package qb

// UpdateOptions configures how UPDATE statements are generated
type UpdateOptions struct {
	// OnlyChanged when true, only includes fields that have changed from their zero values
	OnlyChanged bool

	// IncludeZeroValues when true, includes fields with zero values in the update
	IncludeZeroValues bool

	// ExcludeFields is a list of field names to exclude from the update
	ExcludeFields []string

	// IncludeFields is a list of field names to explicitly include (if specified, only these fields are included)
	IncludeFields []string

	// UpdateAutoFields when true, includes fields marked as auto:true in updates
	UpdateAutoFields bool

	// ReturningFields is a list of field names to return after the update
	ReturningFields []string
}

// DefaultUpdateOptions returns sensible default options for UPDATE statements
func DefaultUpdateOptions() *UpdateOptions {
	return &UpdateOptions{
		OnlyChanged:       false,
		IncludeZeroValues: true,
		ExcludeFields:     nil,
		IncludeFields:     nil,
		UpdateAutoFields:  false,
		ReturningFields:   nil,
	}
}

// ShouldSkipField determines if a field should be skipped based on options
// Accepts both the struct field name and database column name for matching
func (o *UpdateOptions) ShouldSkipField(structFieldName, dbFieldName string) bool {
	// If IncludeFields is specified, only include those fields
	if len(o.IncludeFields) > 0 {
		for _, includeField := range o.IncludeFields {
			if structFieldName == includeField || dbFieldName == includeField {
				return false // Don't skip this field
			}
		}
		return true // Skip fields not in the include list
	}

	// Check exclude list
	for _, excludeField := range o.ExcludeFields {
		if structFieldName == excludeField || dbFieldName == excludeField {
			return true // Skip excluded fields
		}
	}

	return false // Don't skip by default
}
