package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	injectionPayload = `x\' OR 1=1 --`
	// bound placeholder in either the default ("?") or postgres ("$n") form
	ph = `(?:\?|\$\d+)`
)

func TestValidIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"label", true},
		{"id_sample_table", true},
		{"CamelCase", true},
		{"schema.table", true},
		{"with space", true},
		{"", true},
		{`a" = 1 OR "b`, false},
		{`a\`, false},
		{"a\x00b", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, ValidIdentifier(tt.name), tt.name)
	}
}

func TestFunctionsRejectInvalidIdentifier(t *testing.T) {
	ctx := context.Background()
	bad := `a" = 1 OR "b`
	sel := goqu.Dialect("postgres").From("t")
	del := goqu.Dialect("postgres").Delete("t")
	var target []TestUser

	assert.ErrorIs(t, FetchRecord(ctx, nil, sel, map[string]any{bad: 1}, &target), ErrInvalidIdentifier)
	assert.ErrorIs(t, FetchByKey(ctx, nil, sel, bad, 1, &target), ErrInvalidIdentifier)
	assert.ErrorIs(t, FetchWhere(ctx, nil, sel, map[string]any{bad: 1}, &target), ErrInvalidIdentifier)
	assert.ErrorIs(t, DeleteWhere(ctx, nil, del, map[string]any{bad: 1}), ErrInvalidIdentifier)
	assert.ErrorIs(t, DeleteByKey(ctx, nil, del, bad, 1), ErrInvalidIdentifier)

	_, err := Exists(ctx, nil, sel, bad, 1)
	assert.ErrorIs(t, err, ErrInvalidIdentifier)
	_, err = Exists(ctx, nil, sel, "label", 1, bad, 1)
	assert.ErrorIs(t, err, ErrInvalidIdentifier)
	_, err = Exists(ctx, nil, sel, "label", 1, 1, 1)
	assert.ErrorIs(t, err, ErrInvalidParameters)

	repo := &repository{ctx: ctx, tableName: "t", dialect: goqu.Dialect("postgres")}
	_, err = repo.CountWhere(map[string]any{bad: 1})
	assert.ErrorIs(t, err, ErrInvalidIdentifier)
}

func TestDeleteWhereEmptyMap(t *testing.T) {
	del := goqu.Dialect("postgres").Delete("t")
	assert.ErrorIs(t, DeleteWhere(context.Background(), nil, del, map[string]any{}), ErrInvalidParameters)
	assert.ErrorIs(t, DeleteWhere(context.Background(), nil, del, nil), ErrInvalidParameters)
}

// TestFunctionsBindValues checks that values are sent as bound parameters, not inlined in the SQL text
func TestFunctionsBindValues(t *testing.T) {
	h := CreateMockRepository(t, "users")
	defer h.cleanup()
	r := h.repo
	var users []TestUser

	h.mock.ExpectQuery(`SELECT \* FROM "users" WHERE \("name" = ` + ph + `\)$`).
		WithArgs(injectionPayload).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	require.NoError(t, r.FetchWhere(map[string]any{"name": injectionPayload}, &users))

	h.mock.ExpectQuery(`SELECT \* FROM "users" WHERE \("name" = `+ph+`\) LIMIT `+ph+`$`).
		WithArgs(injectionPayload, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	require.NoError(t, r.FetchByKey("name", injectionPayload, &TestUser{}))

	h.mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "users" WHERE \(\("name" = `+ph+`\) AND \("id" != `+ph+`\)\)$`).
		WithArgs(injectionPayload, 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	_, err := r.Exists("name", injectionPayload, "id", 1)
	require.NoError(t, err)

	h.mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "users" WHERE \("name" = ` + ph + `\)$`).
		WithArgs(injectionPayload).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	_, err = r.CountWhere(map[string]any{"name": injectionPayload})
	require.NoError(t, err)

	h.mock.ExpectExec(`DELETE FROM "users" WHERE \("name" = ` + ph + `\)$`).
		WithArgs(injectionPayload).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, r.DeleteWhere(map[string]any{"name": injectionPayload}))

	h.mock.ExpectExec(`SELECT \* FROM "users" WHERE \("name" = ` + ph + `\)$`).
		WithArgs(injectionPayload).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, r.Exec(r.SqlSelect().Where(goqu.C("name").Eq(injectionPayload))))
}
