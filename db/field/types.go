package field

import (
	"slices"
	"sync"
)

/**
 * Reserved types
 * These string type identifiers will be passed to the driver "as-is"
 * Struct-based fields such as time.Time will not be recursively parsed as another struct, instead the type
 * is maintained all the way to the driver
 */
var reservedTypes = []string{"time.Time"}
var mu sync.RWMutex

// AddReservedType register a type as reserved
func AddReservedType(t string) {
	mu.Lock()
	defer mu.Unlock()
	for _, reservedType := range reservedTypes {
		if t == reservedType {
			return
		}
	}
	reservedTypes = append(reservedTypes, t)
}

// GetReservedTypes get a copy of the list of reserved types
func GetReservedTypes() []string {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Clone(reservedTypes)
}

/***
 * Check if a given type string is reserved
 * Reserved means the value is passed "as is"
 * This is a workaround to allow to pass struct-based types that are recognized by the driver
 */
func IsReservedType(t string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Contains(reservedTypes, t)
}
