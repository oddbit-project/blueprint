package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

// sessionCall is one statement recorded by recordingQuerier.
type sessionCall struct {
	query string
	args  []any
}

// recordingQuerier is a dbx.Querier/TxBeginner/TxQuerier that records every
// call instead of talking to a database.
type recordingQuerier struct {
	d        gohan.Dialect
	began    int
	commits  int
	rollback int
	calls    []sessionCall
}

func (r *recordingQuerier) Dialect() gohan.Dialect { return r.d }

func (r *recordingQuerier) Exec(_ context.Context, query string, args ...any) (int64, error) {
	r.calls = append(r.calls, sessionCall{query: query, args: args})
	return 1, nil
}

func (r *recordingQuerier) Get(_ context.Context, _ any, query string, args ...any) error {
	r.calls = append(r.calls, sessionCall{query: query, args: args})
	return nil
}

func (r *recordingQuerier) Select(_ context.Context, _ any, query string, args ...any) error {
	r.calls = append(r.calls, sessionCall{query: query, args: args})
	return nil
}

func (r *recordingQuerier) QueryInt64(_ context.Context, query string, args ...any) (int64, error) {
	r.calls = append(r.calls, sessionCall{query: query, args: args})
	return 0, nil
}

// recordingTx is the transaction handed out by recordingQuerier.BeginTx; it
// shares the parent's call log.
type recordingTx struct{ *recordingQuerier }

func (t recordingTx) Commit() error   { t.commits++; return nil }
func (t recordingTx) Rollback() error { t.rollback++; return nil }

func (r *recordingQuerier) BeginTx(_ context.Context, _ *sql.TxOptions) (dbx.TxQuerier, error) {
	r.began++
	return recordingTx{r}, nil
}

// joinedQuerier is a recordingQuerier that is already a transaction.
type joinedQuerier struct{ *recordingQuerier }

func (j joinedQuerier) Commit() error   { j.commits++; return nil }
func (j joinedQuerier) Rollback() error { j.rollback++; return nil }

func TestWithTxSessionRejectsInvalidInputBeforeDB(t *testing.T) {
	tests := []struct {
		name     string
		dialect  gohan.Dialect
		settings map[string]string
		role     string
		wantErr  error
	}{
		{"empty name", gohan.Postgres(), map[string]string{"": "x"}, "", ErrInvalidSettingName},
		{"no dot (built-in style)", gohan.Postgres(), map[string]string{"search_path": "x"}, "", ErrInvalidSettingName},
		{"leading dot", gohan.Postgres(), map[string]string{".tenant": "x"}, "", ErrInvalidSettingName},
		{"trailing dot", gohan.Postgres(), map[string]string{"app.": "x"}, "", ErrInvalidSettingName},
		{"empty middle part", gohan.Postgres(), map[string]string{"app..tenant": "x"}, "", ErrInvalidSettingName},
		{"part starts with digit", gohan.Postgres(), map[string]string{"app.1tenant": "x"}, "", ErrInvalidSettingName},
		{"part starts with dollar", gohan.Postgres(), map[string]string{"app.$x": "x"}, "", ErrInvalidSettingName},
		{"quote in name", gohan.Postgres(), map[string]string{`app.te"nant`: "x"}, "", ErrInvalidSettingName},
		{"space in name", gohan.Postgres(), map[string]string{"app.tenant id": "x"}, "", ErrInvalidSettingName},
		{"semicolon in name", gohan.Postgres(), map[string]string{"app.t;drop": "x"}, "", ErrInvalidSettingName},
		{"non-ascii in name", gohan.Postgres(), map[string]string{"app.tenänt": "x"}, "", ErrInvalidSettingName},
		{"one bad among good", gohan.Postgres(), map[string]string{"app.a": "1", "bad": "2", "app.z": "3"}, "", ErrInvalidSettingName},
		{"role with dot", gohan.Postgres(), nil, "a.b", ErrInvalidRoleName},
		{"role with NUL", gohan.Postgres(), nil, "a\x00b", ErrInvalidRoleName},
		{"role star", gohan.Postgres(), nil, "*", ErrInvalidRoleName},
		{"role over 63 bytes", gohan.Postgres(), nil, strings.Repeat("r", 64), ErrInvalidRoleName},
		{"names differing only in case", gohan.Postgres(), map[string]string{"app.tenant": "1", "App.Tenant": "2"}, "", ErrInvalidSettingName},
		{"sqlite dialect", gohan.SQLite(), map[string]string{"app.tenant": "1"}, "", ErrNotPostgres},
		{"clickhouse dialect", gohan.ClickHouse(), nil, "r", ErrNotPostgres},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &recordingQuerier{d: tc.dialect}
			called := false
			err := WithTxSession(context.Background(), q, tc.settings, tc.role, func(dbx.Querier) error {
				called = true
				return nil
			})
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
			assert.False(t, called, "fn must not run")
			assert.Zero(t, q.began, "no transaction may be started")
			assert.Empty(t, q.calls, "no statement may be sent")
		})
	}
}

func TestWithTxSessionAcceptsValidNames(t *testing.T) {
	names := []string{"app.tenant_id", "App.Tenant", "_x._y", "a.b.c", "app.t$1", "my_ext.v2_opt"}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			q := &recordingQuerier{d: gohan.Postgres()}
			err := WithTxSession(context.Background(), q, map[string]string{n: "v"}, "", func(dbx.Querier) error { return nil })
			require.NoError(t, err)
			require.Len(t, q.calls, 1)
			assert.Equal(t, []any{n, "v"}, q.calls[0].args)
		})
	}
}

func TestWithTxSessionStatementsInOrder(t *testing.T) {
	q := &recordingQuerier{d: gohan.Postgres()}
	settings := map[string]string{"app.z": "3", "app.a": "1", "app.m": "x'; DROP TABLE t; --"}
	var fnCallsAtEntry int
	err := WithTxSession(context.Background(), q, settings, `tenant "role"`, func(tx dbx.Querier) error {
		fnCallsAtEntry = len(q.calls)
		_, _ = tx.Exec(context.Background(), "SELECT 1")
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, 1, q.began)
	assert.Equal(t, 1, q.commits)
	assert.Zero(t, q.rollback)
	assert.Equal(t, 4, fnCallsAtEntry, "settings and role must be applied before fn")
	assert.Equal(t, []sessionCall{
		{query: "SELECT set_config($1, $2, true)", args: []any{"app.a", "1"}},
		{query: "SELECT set_config($1, $2, true)", args: []any{"app.m", "x'; DROP TABLE t; --"}},
		{query: "SELECT set_config($1, $2, true)", args: []any{"app.z", "3"}},
		{query: `SET LOCAL ROLE "tenant ""role"""`},
		{query: "SELECT 1"},
	}, q.calls)
}

func TestWithTxSessionNoRoleNoSettings(t *testing.T) {
	q := &recordingQuerier{d: gohan.Postgres()}
	called := false
	err := WithTxSession(context.Background(), q, nil, "", func(dbx.Querier) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1, q.began)
	assert.Empty(t, q.calls)
}

func TestWithTxSessionFnErrorRollsBack(t *testing.T) {
	q := &recordingQuerier{d: gohan.Postgres()}
	boom := errors.New("boom")
	err := WithTxSession(context.Background(), q, map[string]string{"app.t": "1"}, "", func(dbx.Querier) error {
		return boom
	})
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 1, q.rollback)
	assert.Zero(t, q.commits)
}

func TestWithTxSessionJoinsOpenTx(t *testing.T) {
	inner := &recordingQuerier{d: gohan.Postgres()}
	q := joinedQuerier{inner}
	err := WithTxSession(context.Background(), q, map[string]string{"app.t": "1"}, "r", func(dbx.Querier) error { return nil })
	require.NoError(t, err)
	assert.Zero(t, inner.began, "must not begin a nested transaction")
	assert.Zero(t, inner.commits, "the outer transaction owns commit")
	assert.Zero(t, inner.rollback)
	assert.Equal(t, []sessionCall{
		{query: "SELECT set_config($1, $2, true)", args: []any{"app.t", "1"}},
		{query: `SET LOCAL ROLE "r"`},
	}, inner.calls)
}
