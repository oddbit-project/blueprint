# Runtime

The runtime package contains small reflection helpers for reading comma-separated struct tag values.
It is used internally by the `db/field` package to parse `db`, `ch`, `json`, `grid`, `goqu` and similar
tags.

```go
import "github.com/oddbit-project/blueprint/runtime"
```

!!! note
    The package name `runtime` shadows the standard library `runtime` package. If you need both in the
    same file, import one of them with an alias.

## Overview

- `ParseTag` - split a single tag value on commas
- `MustParseTag` - same as `ParseTag`, with a fallback value when the tag is absent
- `ParseTagList` - use the first tag, from a list of candidates, that has a value

## Usage

```go
package main

import (
	"fmt"
	"reflect"

	"github.com/oddbit-project/blueprint/runtime"
)

type User struct {
	ID    int    `db:"id,auto" json:"id"`
	Email string `ch:"email" json:"email,omitempty"`
	Name  string
}

func main() {
	t := reflect.TypeOf(User{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// first of "db" or "ch" that is present
		dbTag := runtime.ParseTagList(field, []string{"db", "ch"})

		// json tag, or an empty slice when absent
		jsonTag := runtime.ParseTag(field, "json")

		// fallback when the tag is absent
		grid := runtime.MustParseTag(field, "grid", []string{"sort"})

		fmt.Println(field.Name, dbTag, jsonTag, grid)
	}
	// prints:
	// ID [id auto] [id] [sort]
	// Email [email] [email omitempty] [sort]
	// Name [] [] [sort]
}
```

## API Reference

### Functions

#### ParseTag

```go
func ParseTag(field reflect.StructField, tag string) []string
```

Returns the value of `tag` on `field`, split on `,`, with leading and trailing whitespace removed from
each part. Returns an empty (non-nil) slice if the tag is absent or has an empty value.

#### MustParseTag

```go
func MustParseTag(field reflect.StructField, tag string, defaultValue []string) []string
```

Same as `ParseTag`, but returns `defaultValue` when the tag is absent or empty. Despite the `Must`
prefix, it never panics.

#### ParseTagList

```go
func ParseTagList(field reflect.StructField, tags []string) []string
```

Checks each tag name in `tags` in order and returns the value of the first one that is present and
non-empty, split on `,` and trimmed as in `ParseTag`. Remaining tags are ignored. Returns an empty (non-nil) slice if none match.

## Notes

- Each part is trimmed with `strings.TrimSpace`: `db:"id, auto"` yields `["id", "auto"]`, so `db/field` marks
  the field as auto-generated just like `db:"id,auto"`.
- A tag with an empty value (`db:""`) is treated the same as a missing tag.
- Special values such as `"-"` are returned as-is (`db:"-"` yields `["-"]`); interpreting them is up to
  the caller.
- `MustParseTag` returns `defaultValue` itself, not a copy; modifying the returned slice modifies the
  default.
