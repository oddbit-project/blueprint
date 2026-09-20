package migrations

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/oddbit-project/blueprint/utils"
)

const (
	// ErrMissingVar is a placeholder the caller supplied no value for. It is an
	// error rather than an empty string: an unresolved name would otherwise
	// reach the server as literal text, usually inside a GRANT or an owner
	// clause, and fail in a way that names nothing useful.
	ErrMissingVar = utils.Error("migration placeholder has no value")
)

// Vars are the values a migration's placeholders are substituted with, keyed by
// the name inside ${...}. A value is spliced into the SQL verbatim, with no
// quoting or escaping, so it must be a trusted identifier; it is not a way to
// pass data to a statement, and it must never carry a secret, because the
// substituted text is stored in the migration table.
type Vars map[string]string

// placeholder is ${name}. The braces are required, so the form cannot collide
// with SQL's own uses of $: positional parameters ($1) and dollar-quoted bodies
// ($$ ... $$) pass through untouched. The name itself is matched permissively
// so that a misspelled or unsupported one -- ${app-role}, ${1} -- is reported
// as missing rather than shipped to the server as literal text.
var placeholder = regexp.MustCompile(`\$\{([^{}]*)\}`)

// Substitute wraps a Source, replacing ${name} in every migration it reads with
// the value given. It is how a deployment-specific identifier -- the role an
// application connects as, a schema, a tablespace -- reaches DDL that is
// otherwise fixed.
//
// The recorded SHA2 stays the TEMPLATE's, not the substituted text's, while the
// recorded contents are what actually ran. That asymmetry is deliberate: a
// migration's identity is the file as shipped, so a deployment that changes one
// of these values does not make its applied migrations look edited. Two
// consequences are worth knowing: re-hashing a stored `contents` does not
// reproduce its `sha2` for any migration that carried a placeholder, and
// changing a value does not re-run a migration that already ran -- migrations
// are skipped by name, so a new value needs a new migration.
//
// Substitution is textual and has no logic: there is no way to express a
// condition or a loop, so the SQL that runs is the SQL that was shipped with
// its identifiers filled in, and nothing else.
//
// src must not be nil.
func Substitute(src Source, vars Vars) Source {
	return &substituted{src: src, vars: vars}
}

type substituted struct {
	src  Source
	vars Vars
}

// List is the wrapped source's: substitution changes contents, never names.
func (s *substituted) List() ([]string, error) { return s.src.List() }

// Read substitutes the placeholders, keeping the template's hash. A placeholder
// with no value, or an empty one, is an error naming every such placeholder.
func (s *substituted) Read(name string) (*MigrationRecord, error) {
	record, err := s.src.Read(name)
	if err != nil {
		return nil, err
	}
	missing := make(map[string]struct{})
	contents := placeholder.ReplaceAllStringFunc(record.Contents, func(match string) string {
		// match is ${...}; the name is what the braces enclose
		key := match[2 : len(match)-1]
		value := s.vars[key]
		if value == "" {
			missing[key] = struct{}{}
			return match
		}
		return value
	})
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for key := range missing {
			names = append(names, "${"+key+"}")
		}
		sort.Strings(names)
		return nil, fmt.Errorf("%w: migration %s: %s",
			ErrMissingVar, name, strings.Join(names, ", "))
	}
	out := *record
	out.Contents = contents
	return &out, nil
}
