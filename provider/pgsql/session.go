package pgsql

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/oddbit-project/gohan"
)

const (
	// ErrInvalidSettingName is returned by WithTxSession when a setting name
	// is not a custom (dotted) PostgreSQL configuration parameter name.
	ErrInvalidSettingName = utils.Error("pgsql: invalid custom setting name")
	// ErrInvalidRoleName is returned by WithTxSession when role cannot be
	// quoted as a single identifier.
	ErrInvalidRoleName = utils.Error("pgsql: invalid role name")
	// ErrNotPostgres is returned by WithTxSession when the Querier's dialect
	// is not PostgreSQL.
	ErrNotPostgres = utils.Error("pgsql: querier dialect is not PostgreSQL")
)

// customSettingName matches a custom configuration parameter name: two or
// more dot-separated parts, each an ASCII letter or underscore followed by
// ASCII letters, digits, underscores or dollar signs.
var customSettingName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*(\.[A-Za-z_][A-Za-z0-9_$]*)+$`)

// maxIdentLen is PostgreSQL's identifier length limit (NAMEDATALEN - 1);
// a longer quoted role name would be silently truncated to another role.
const maxIdentLen = 63

// WithTxSession runs fn inside a transaction (via dbx.WithTx) after making
// settings and role transaction-local:
//   - each setting is applied with SELECT set_config(name, value, true), in
//     ascending name order, with name and value as bound parameters;
//   - if role is not empty, SET LOCAL ROLE switches to it (the name is
//     quoted with the Querier's gohan dialect).
//
// Everything is undone by PostgreSQL when the transaction ends, so nothing
// leaks to the next user of the pooled connection. Setting names must be
// custom parameter names (e.g. "app.tenant_id"; see customSettingName), no
// two of them equal ignoring case (PostgreSQL treats them as one setting),
// and role must be at most 63 bytes, without a dot, and not "*"; invalid
// input is rejected before any statement is sent. Setting and role names
// should be constants: a dotted name also matches extension settings (such
// as auto_explain.*), which set_config changes too. If q is already a transaction, WithTxSession joins it:
// the settings and role then last until that outer transaction ends, not
// just until fn returns.
func WithTxSession(ctx context.Context, q dbx.Querier, settings map[string]string, role string, fn func(tx dbx.Querier) error) error {
	d := q.Dialect()
	if d.Name() != gohan.Postgres().Name() {
		return ErrNotPostgres
	}

	names := make([]string, 0, len(settings))
	folded := make(map[string]bool, len(settings))
	for name := range settings {
		if !customSettingName.MatchString(name) {
			return fmt.Errorf("%w: %q", ErrInvalidSettingName, name)
		}
		key := strings.ToLower(name)
		if folded[key] {
			return fmt.Errorf("%w: %q is set twice (setting names ignore case)", ErrInvalidSettingName, name)
		}
		folded[key] = true
		names = append(names, name)
	}
	sort.Strings(names)

	setRole := ""
	if role != "" {
		if strings.Contains(role, ".") || role == "*" || len(role) > maxIdentLen {
			return fmt.Errorf("%w: %q", ErrInvalidRoleName, role)
		}
		quoted, err := d.QuoteIdent(role)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidRoleName, err)
		}
		setRole = "SET LOCAL ROLE " + quoted
	}

	return dbx.WithTx(ctx, q, nil, func(tx dbx.Querier) error {
		for _, name := range names {
			if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", name, settings[name]); err != nil {
				return err
			}
		}
		if setRole != "" {
			if _, err := tx.Exec(ctx, setRole); err != nil {
				return err
			}
		}
		return fn(tx)
	})
}
