package migrations

import (
	"fmt"
	"regexp"
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
// the name inside ${...}.
type Vars map[string]string

// placeholder is ${name}. The braces are required, so the form cannot collide
// with SQL's own uses of $: positional parameters ($1) and dollar-quoted bodies
// ($$ ... $$) pass through untouched.
var placeholder = regexp.MustCompile(`\$\{([A-Za-z][A-Za-z0-9_]*)\}`)

// Substitute wraps a Source, replacing ${name} in every migration it reads with
// the value given. It is how a deployment-specific identifier -- the role an
// application connects as, a schema, a tablespace -- reaches DDL that is
// otherwise fixed.
//
// The recorded SHA2 stays the TEMPLATE's, not the substituted text's, while the
// recorded contents are what actually ran. That asymmetry is deliberate: a
// migration's identity is the file as shipped, so a deployment that changes one
// of these values does not make its applied migrations look edited. The
// consequence, worth knowing when auditing an installation, is that re-hashing
// a stored `contents` does not reproduce its `sha2` for any migration that
// carried a placeholder.
//
// Substitution is textual and has no logic: there is no way to express a
// condition or a loop, so the SQL that runs is the SQL that was shipped with
// its identifiers filled in, and nothing else.
//
// A nil source, or no vars, is returned unchanged.
func Substitute(src Source, vars Vars) Source {
	if src == nil || len(vars) == 0 {
		return src
	}
	return &substituted{src: src, vars: vars}
}

type substituted struct {
	src  Source
	vars Vars
}

// List is the wrapped source's: substitution changes contents, never names.
func (s *substituted) List() ([]string, error) { return s.src.List() }

// Read substitutes the placeholders, keeping the template's hash.
func (s *substituted) Read(name string) (*MigrationRecord, error) {
	record, err := s.src.Read(name)
	if err != nil {
		return nil, err
	}
	var missing []string
	contents := placeholder.ReplaceAllStringFunc(record.Contents, func(match string) string {
		key := placeholder.FindStringSubmatch(match)[1]
		value, ok := s.vars[key]
		if !ok {
			missing = append(missing, key)
			return match
		}
		return value
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: migration %s: ${%s}",
			ErrMissingVar, name, strings.Join(missing, "}, ${"))
	}
	out := *record
	out.Contents = contents
	return &out, nil
}
