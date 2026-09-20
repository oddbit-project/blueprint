package migrations

import (
	"testing"

	"github.com/oddbit-project/blueprint/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMigration = "001_test.sql"

// newTestSource builds a memory source holding one migration.
func newTestSource(t *testing.T, name string, contents string) Source {
	t.Helper()
	src := NewMemorySource()
	src.Add(name, contents)
	return src
}

// failingSource fails every operation; it stands in for a source whose
// underlying storage is unavailable.
type failingSource struct {
	err error
}

func (f *failingSource) List() ([]string, error)               { return nil, f.err }
func (f *failingSource) Read(string) (*MigrationRecord, error) { return nil, f.err }

func TestSubstitute(t *testing.T) {
	// the $body$ tag is what makes the braces load-bearing: without them,
	// $body would be a placeholder name
	const dollars = `DO $body$ BEGIN
	IF to_regclass('old') IS NOT NULL THEN
		EXECUTE 'DELETE FROM thing WHERE id = $1';
	END IF;
END $body$;`

	tests := []struct {
		name     string
		contents string
		vars     Vars
		want     string
		wantErr  string
	}{
		{
			name:     "every placeholder is replaced",
			contents: "GRANT SELECT ON ${table} TO ${role};",
			vars:     Vars{"table": "thing", "role": "app"},
			want:     "GRANT SELECT ON thing TO app;",
		},
		{
			name:     "a repeated placeholder is replaced everywhere",
			contents: "GRANT SELECT ON thing TO ${role}; REVOKE DELETE ON thing FROM ${role};",
			vars:     Vars{"role": "app"},
			want:     "GRANT SELECT ON thing TO app; REVOKE DELETE ON thing FROM app;",
		},
		{
			name:     "SQL's own dollars are left alone",
			contents: dollars + " GRANT SELECT ON thing TO ${role};",
			vars:     Vars{"role": "app"},
			want:     dollars + " GRANT SELECT ON thing TO app;",
		},
		{
			name:     "a var with no placeholder is ignored",
			contents: "SELECT 1;",
			vars:     Vars{"role": "app"},
			want:     "SELECT 1;",
		},
		{
			name:     "an unsupplied placeholder is refused",
			contents: "GRANT SELECT ON thing TO ${role};",
			vars:     Vars{"other": "app"},
			wantErr:  "migration 001_test.sql: ${role}",
		},
		{
			name:     "no vars at all refuses too",
			contents: "GRANT SELECT ON thing TO ${role};",
			vars:     nil,
			wantErr:  "migration 001_test.sql: ${role}",
		},
		{
			name:     "an empty value counts as unsupplied",
			contents: "GRANT SELECT ON thing TO ${role};",
			vars:     Vars{"role": ""},
			wantErr:  "migration 001_test.sql: ${role}",
		},
		{
			name:     "a name the substitution does not support is refused, not shipped",
			contents: "GRANT SELECT ON thing TO ${app-role};",
			vars:     Vars{"appRole": "app"},
			wantErr:  "migration 001_test.sql: ${app-role}",
		},
		{
			name:     "an empty name is a placeholder too, and is refused",
			contents: "GRANT SELECT ON thing TO ${};",
			vars:     Vars{"role": "app"},
			wantErr:  "migration 001_test.sql: ${}",
		},
		{
			name:     "a placeholder inside a string literal is refused too",
			contents: "COMMENT ON TABLE thing IS 'owned by ${1}';",
			vars:     Vars{"role": "app"},
			wantErr:  "migration 001_test.sql: ${1}",
		},
		{
			name:     "every missing name is reported once, in order",
			contents: "GRANT ${verb} ON ${table} TO ${role}; REVOKE ${verb} ON ${table} FROM ${role};",
			vars:     Vars{"table": "thing"},
			wantErr:  "migration 001_test.sql: ${role}, ${verb}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := Substitute(newTestSource(t, testMigration, tc.contents), tc.vars)

			record, err := src.Read(testMigration)
			if tc.wantErr != "" {
				require.ErrorIs(t, err, ErrMissingVar)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Nil(t, record)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, record.Contents)
		})
	}
}

// The identity of a migration is the file as shipped, so a deployment that
// changes one of these values does not make its applied migrations look edited.
func TestSubstituteKeepsTheTemplateHash(t *testing.T) {
	const template = "REVOKE DELETE ON thing FROM ${role};"

	plain, err := newTestSource(t, testMigration, template).Read(testMigration)
	require.NoError(t, err)

	first, err := Substitute(newTestSource(t, testMigration, template), Vars{"role": "app"}).Read(testMigration)
	require.NoError(t, err)
	second, err := Substitute(newTestSource(t, testMigration, template), Vars{"role": "other_app"}).Read(testMigration)
	require.NoError(t, err)

	assert.Equal(t, plain.SHA2, first.SHA2)
	assert.Equal(t, plain.SHA2, second.SHA2)
	assert.NotEqual(t, first.Contents, second.Contents)
}

// Names are the wrapped source's; only contents are rewritten.
func TestSubstituteListsWhatItWraps(t *testing.T) {
	names, err := Substitute(newTestSource(t, testMigration, "SELECT 1;"), Vars{"role": "app"}).List()
	require.NoError(t, err)
	assert.Equal(t, []string{testMigration}, names)
}

// Failures of the wrapped source are the caller's to handle, unchanged.
func TestSubstitutePropagatesSourceErrors(t *testing.T) {
	const errSource = utils.Error("source unavailable")
	src := Substitute(&failingSource{err: errSource}, Vars{"role": "app"})

	_, err := src.List()
	require.ErrorIs(t, err, errSource)

	_, err = src.Read(testMigration)
	require.ErrorIs(t, err, errSource)
}
