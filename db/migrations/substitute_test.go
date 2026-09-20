package migrations

import (
	"errors"
	"testing"
)

// source builds a memory source holding one migration.
func source(t *testing.T, contents string) Source {
	t.Helper()
	src := NewMemorySource()
	src.Add("001_test.sql", contents)
	return src
}

func read(t *testing.T, src Source) *MigrationRecord {
	t.Helper()
	record, err := src.Read("001_test.sql")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return record
}

func TestSubstituteReplacesEveryPlaceholder(t *testing.T) {
	src := Substitute(source(t, "GRANT SELECT ON ${table} TO ${role};"),
		Vars{"table": "thing", "role": "app"})

	if got, want := read(t, src).Contents, "GRANT SELECT ON thing TO app;"; got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

// The identity of a migration is the file as shipped, so a deployment that
// changes one of these values does not make its applied migrations look edited.
func TestSubstituteKeepsTheTemplateHash(t *testing.T) {
	template := "REVOKE DELETE ON thing FROM ${role};"
	plain := read(t, source(t, template))

	first := read(t, Substitute(source(t, template), Vars{"role": "app"}))
	second := read(t, Substitute(source(t, template), Vars{"role": "other_app"}))

	if first.SHA2 != plain.SHA2 || second.SHA2 != plain.SHA2 {
		t.Errorf("hash changed with the value: %s / %s, want %s",
			first.SHA2, second.SHA2, plain.SHA2)
	}
	if first.Contents == second.Contents {
		t.Error("contents did not change with the value")
	}
}

// An unresolved name would otherwise reach the server as literal text.
func TestSubstituteRefusesAnUnsuppliedPlaceholder(t *testing.T) {
	_, err := Substitute(source(t, "GRANT SELECT ON thing TO ${role};"),
		Vars{"other": "app"}).Read("001_test.sql")

	if !errors.Is(err, ErrMissingVar) {
		t.Fatalf("err = %v, want %v", err, ErrMissingVar)
	}
}

// $1 and $$ are SQL's own; only ${...} is ours.
func TestSubstituteLeavesOtherDollarsAlone(t *testing.T) {
	const untouched = `DO $$ BEGIN
	IF to_regclass('old') IS NOT NULL THEN
		EXECUTE 'DELETE FROM thing WHERE id = $1';
	END IF;
END $$;`
	src := Substitute(source(t, untouched), Vars{"role": "app"})

	if got := read(t, src).Contents; got != untouched {
		t.Errorf("contents = %q, want them unchanged", got)
	}
}

// Wrapping is a no-op when there is nothing to substitute.
func TestSubstituteWithoutVarsIsTheSourceItself(t *testing.T) {
	src := source(t, "SELECT 1;")
	if got := Substitute(src, nil); got != src {
		t.Error("a source with no vars was wrapped anyway")
	}
	if got := Substitute(nil, Vars{"role": "app"}); got != nil {
		t.Error("a nil source was wrapped")
	}
}

// Names are the wrapped source's; only contents are rewritten.
func TestSubstituteListsWhatItWraps(t *testing.T) {
	names, err := Substitute(source(t, "SELECT 1;"), Vars{"role": "app"}).List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 1 || names[0] != "001_test.sql" {
		t.Errorf("names = %v", names)
	}
}
