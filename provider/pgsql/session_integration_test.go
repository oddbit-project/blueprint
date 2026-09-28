package pgsql

import (
	"errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/dbx"
)

const (
	// sessRole is a non-superuser role (so row-level security applies) whose
	// name needs quoting: it contains a space and a double quote.
	sessRole    = `pgsess "tenant" role`
	sessRoleSQL = `"pgsess ""tenant"" role"`
	sessTable   = "pgsess_docs"
)

// setupSessionFixtures (re)creates the RLS table and the tenant role via
// the suite's admin (superuser) connection.
func (s *PGIntegrationTestSuite) setupSessionFixtures() {
	stmts := []string{
		"DROP TABLE IF EXISTS " + sessTable,
		"DROP ROLE IF EXISTS " + sessRoleSQL,
		"CREATE ROLE " + sessRoleSQL + " NOLOGIN",
		"CREATE TABLE " + sessTable + " (id serial primary key, tenant_id int not null, body text not null)",
		"INSERT INTO " + sessTable + " (tenant_id, body) VALUES (1, 'a1'), (1, 'a2'), (2, 'b1')",
		"ALTER TABLE " + sessTable + " ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY tenant_isolation ON " + sessTable +
			" USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::int)",
		"GRANT SELECT ON " + sessTable + " TO " + sessRoleSQL,
	}
	for _, st := range stmts {
		_, err := s.client.Conn.ExecContext(s.ctx, st)
		require.NoError(s.T(), err, st)
	}
	s.T().Cleanup(func() {
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS "+sessTable)
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP ROLE IF EXISTS "+sessRoleSQL)
	})
}

// newSingleConnDbxClient connects a dbx.Querier whose pool holds exactly
// one connection, so every statement (inside and after a transaction) runs
// on the same PostgreSQL backend.
func (s *PGIntegrationTestSuite) newSingleConnDbxClient(dsn string) (*db.SqlClient, *dbx.SQLQuerier) {
	cfg := NewClientConfig()
	cfg.DSN = dsn
	cfg.MaxOpenConns = 1
	cfg.MaxIdleConns = 1
	client, err := NewClient(cfg)
	require.NoError(s.T(), err)
	q, err := dbx.FromClient(client)
	require.NoError(s.T(), err)
	return client, q
}

// sessionSnapshot is what the tests read back from the backend.
type sessionSnapshot struct {
	Pid    int64  `db:"pid"`
	User   string `db:"usr"`
	Tenant string `db:"tenant"`
}

const sessionSnapshotSQL = `SELECT pg_backend_pid() AS pid, current_user AS usr,
	coalesce(current_setting('app.tenant_id', true), '<null>') AS tenant`

func (s *PGIntegrationTestSuite) snapshot(q dbx.Querier) sessionSnapshot {
	var snap sessionSnapshot
	require.NoError(s.T(), q.Get(s.ctx, &snap, sessionSnapshotSQL))
	return snap
}

func (s *PGIntegrationTestSuite) TestWithTxSessionSettingsAndRole() {
	s.setupSessionFixtures()
	for name, dsn := range s.dbxDsns() {
		s.Run(name, func() {
			client, q := s.newSingleConnDbxClient(dsn)
			defer client.Disconnect()

			before := s.snapshot(q)
			require.Equal(s.T(), "blueprint", before.User)

			var inside sessionSnapshot
			var rows []string
			err := WithTxSession(s.ctx, q, map[string]string{"app.tenant_id": "1", "app.other": "o"}, sessRole,
				func(tx dbx.Querier) error {
					inside = s.snapshot(tx)
					var other string
					if err := tx.Get(s.ctx, &other, "SELECT current_setting('app.other')"); err != nil {
						return err
					}
					assert.Equal(s.T(), "o", other)
					return tx.Select(s.ctx, &rows, "SELECT body FROM "+sessTable+" ORDER BY body")
				})
			require.NoError(s.T(), err)

			assert.Equal(s.T(), "1", inside.Tenant)
			assert.Equal(s.T(), sessRole, inside.User)
			assert.Equal(s.T(), []string{"a1", "a2"}, rows, "RLS must restrict rows to tenant 1")

			after := s.snapshot(q)
			require.Equal(s.T(), before.Pid, after.Pid, "single-connection pool must reuse the backend")
			assert.Equal(s.T(), before.Pid, inside.Pid)
			assert.Equal(s.T(), "blueprint", after.User, "role must revert when the transaction ends")
			// A custom setting that was set transaction-locally reads back as
			// '' (not NULL) afterwards in the same session: hence nullif() in
			// the policy.
			assert.Equal(s.T(), "", after.Tenant, "setting must not leak past the transaction")
		})
	}
}

func (s *PGIntegrationTestSuite) TestWithTxSessionRLSOtherTenantAndUnset() {
	s.setupSessionFixtures()
	client, q := s.newDbxClient(s.dsn)
	defer client.Disconnect()

	var rows []string
	err := WithTxSession(s.ctx, q, map[string]string{"app.tenant_id": "2"}, sessRole, func(tx dbx.Querier) error {
		return tx.Select(s.ctx, &rows, "SELECT body FROM "+sessTable+" ORDER BY body")
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), []string{"b1"}, rows)

	rows = nil
	err = WithTxSession(s.ctx, q, nil, sessRole, func(tx dbx.Querier) error {
		return tx.Select(s.ctx, &rows, "SELECT body FROM "+sessTable+" ORDER BY body")
	})
	require.NoError(s.T(), err)
	assert.Empty(s.T(), rows, "no tenant set: the policy must match nothing")
}

// TestWithTxSessionPlainSetLeaks guards the premise of the helper: a
// session-level SET on a pooled connection survives into the next user of
// that connection.
func (s *PGIntegrationTestSuite) TestWithTxSessionPlainSetLeaks() {
	client, q := s.newSingleConnDbxClient(s.dsn)
	defer client.Disconnect()

	_, err := q.Exec(s.ctx, "SET app.tenant_id = '42'")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "42", s.snapshot(q).Tenant)
	_, err = q.Exec(s.ctx, "RESET app.tenant_id")
	require.NoError(s.T(), err)
}

func (s *PGIntegrationTestSuite) TestWithTxSessionValueIsBound() {
	s.setupSessionFixtures()
	for name, dsn := range s.dbxDsns() {
		s.Run(name, func() {
			client, q := s.newDbxClient(dsn)
			defer client.Disconnect()

			evil := `x'); DROP TABLE ` + sessTable + `; -- "q" ; \' $1`
			var got string
			err := WithTxSession(s.ctx, q, map[string]string{"app.tenant_id": evil}, "", func(tx dbx.Querier) error {
				return tx.Get(s.ctx, &got, "SELECT current_setting('app.tenant_id')")
			})
			require.NoError(s.T(), err)
			assert.Equal(s.T(), evil, got)

			exists, err := TableExists(s.ctx, s.client, sessTable, SchemaDefault)
			require.NoError(s.T(), err)
			assert.True(s.T(), exists, "value must be bound, not injected")
		})
	}
}

func (s *PGIntegrationTestSuite) TestWithTxSessionInvalidNameNoDB() {
	// A querier over a closed pool: any statement would fail, so success of
	// the rejection proves nothing was sent.
	client, q := s.newDbxClient(s.dsn)
	client.Disconnect()

	err := WithTxSession(s.ctx, q, map[string]string{"tenant_id": "1"}, "", func(dbx.Querier) error { return nil })
	assert.ErrorIs(s.T(), err, ErrInvalidSettingName)
}

func (s *PGIntegrationTestSuite) TestWithTxSessionFnErrorRollsBack() {
	client, q := s.newSingleConnDbxClient(s.dsn)
	defer client.Disconnect()

	_, err := q.Exec(s.ctx, "CREATE TABLE IF NOT EXISTS session_rollback_probe (v int)")
	require.NoError(s.T(), err)
	defer func() { _, _ = q.Exec(s.ctx, "DROP TABLE IF EXISTS session_rollback_probe") }()

	boom := errors.New("boom")
	err = WithTxSession(s.ctx, q, map[string]string{"app.tenant_id": "9"}, "", func(tx dbx.Querier) error {
		if _, err := tx.Exec(s.ctx, "INSERT INTO session_rollback_probe VALUES (1)"); err != nil {
			return err
		}
		return boom
	})
	assert.ErrorIs(s.T(), err, boom)
	assert.NotEqual(s.T(), "9", s.snapshot(q).Tenant)
	n, err := q.QueryInt64(s.ctx, "SELECT count(*) FROM session_rollback_probe")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), n, "the write inside fn was rolled back")
}

// TestWithTxSessionJoinedTx: inside an already-open transaction the
// settings and role outlive WithTxSession and last until the outer
// transaction ends.
func (s *PGIntegrationTestSuite) TestWithTxSessionJoinedTx() {
	s.setupSessionFixtures()
	client, q := s.newSingleConnDbxClient(s.dsn)
	defer client.Disconnect()

	var afterInner sessionSnapshot
	err := dbx.WithTx(s.ctx, q, nil, func(outer dbx.Querier) error {
		if err := WithTxSession(s.ctx, outer, map[string]string{"app.tenant_id": "1"}, sessRole,
			func(dbx.Querier) error { return nil }); err != nil {
			return err
		}
		afterInner = s.snapshot(outer)
		return nil
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "1", afterInner.Tenant, "still in the outer tx: setting persists")
	assert.Equal(s.T(), sessRole, afterInner.User, "still in the outer tx: role persists")

	after := s.snapshot(q)
	assert.Equal(s.T(), afterInner.Pid, after.Pid)
	assert.Equal(s.T(), "", after.Tenant)
	assert.Equal(s.T(), "blueprint", after.User)
}
