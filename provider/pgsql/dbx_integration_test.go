package pgsql

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/sqlb"
)

// dbxPgUser is the record type for TestDbxFromClient.
type dbxPgUser struct {
	ID    int64  `db:"id,auto"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

// dbxPgOrder/dbxPgRefund back TestDbxJoinsCtesUnion.
type dbxPgOrder struct {
	ID     int64 `db:"id,auto"`
	UserID int64 `db:"user_id"`
	Amount int64 `db:"amount"`
}

type dbxPgRefund struct {
	ID     int64 `db:"id,auto"`
	UserID int64 `db:"user_id"`
	Amount int64 `db:"amount"`
}

const dbxPgUsersDDL = `CREATE TABLE dbx_pg_users (id serial primary key, name text not null, email text not null)`
const dbxPgOrdersDDL = `CREATE TABLE dbx_pg_orders (id serial primary key, user_id int not null, amount int not null)`
const dbxPgRefundsDDL = `CREATE TABLE dbx_pg_refunds (id serial primary key, user_id int not null, amount int not null)`

// dbxDsns returns the two pgx wire-protocol DSNs this file's tests run
// every case against: s.dsn (simple protocol, what the rest of this suite
// uses) and the plain connection string (extended protocol, pgx's
// default), where untyped bound parameters fail with "could not determine
// data type of parameter" unless sqlb types them correctly.
func (s *PGIntegrationTestSuite) dbxDsns() map[string]string {
	extDSN, err := s.pgInstance.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)
	return map[string]string{
		"simple_protocol":   s.dsn,
		"extended_protocol": extDSN,
	}
}

// newDbxClient connects a fresh db.SqlClient/dbx.Querier pair to dsn.
func (s *PGIntegrationTestSuite) newDbxClient(dsn string) (*db.SqlClient, *dbx.SQLQuerier) {
	cfg := NewClientConfig()
	cfg.DSN = dsn
	client, err := NewClient(cfg)
	require.NoError(s.T(), err)
	q, err := dbx.FromClient(client)
	require.NoError(s.T(), err)
	return client, q
}

// resetDbxTables drops and recreates this file's tables via the suite's
// admin connection.
func (s *PGIntegrationTestSuite) resetDbxTables() {
	_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS dbx_pg_orders")
	_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS dbx_pg_refunds")
	_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS dbx_pg_users")
	_, err := s.client.Conn.ExecContext(s.ctx, dbxPgUsersDDL)
	require.NoError(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, dbxPgOrdersDDL)
	require.NoError(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, dbxPgRefundsDDL)
	require.NoError(s.T(), err)
}

func (s *PGIntegrationTestSuite) TestDbxFromClient() {
	for name, dsn := range s.dbxDsns() {
		s.Run(name, func() {
			s.resetDbxTables()
			client, q := s.newDbxClient(dsn)
			defer client.Disconnect()

			r, err := dbx.NewRepository[dbxPgUser](q, "dbx_pg_users")
			require.NoError(s.T(), err)

			require.NoError(s.T(), r.Insert(s.ctx, &dbxPgUser{Name: "Alice", Email: "alice@x.com"}))

			got, err := r.GetBy(s.ctx, map[string]any{"email": "alice@x.com"})
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Alice", got.Name)
			assert.NotZero(s.T(), got.ID)

			list, err := r.List(s.ctx, nil)
			require.NoError(s.T(), err)
			assert.Len(s.T(), list, 1)

			cnt, err := r.Count(s.ctx, nil)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), cnt)

			exists, err := r.Exists(s.ctx, sqlb.Col("id").Eq(got.ID))
			require.NoError(s.T(), err)
			assert.True(s.T(), exists)

			n, err := r.Delete(s.ctx, sqlb.Col("id").Eq(got.ID))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), n)
		})
	}
}

// TestDbxJoinsCtesUnion exercises Join, LeftJoinUsing, With, WithRecursive,
// Union, UnionAll, Offset without Limit, Exists and In subqueries, each
// with at least one bound value, on both pgx wire protocols.
func (s *PGIntegrationTestSuite) TestDbxJoinsCtesUnion() {
	for name, dsn := range s.dbxDsns() {
		s.Run(name, func() {
			s.resetDbxTables()
			client, q := s.newDbxClient(dsn)
			defer client.Disconnect()

			users, err := dbx.NewRepository[dbxPgUser](q, "dbx_pg_users")
			require.NoError(s.T(), err)
			orders, err := dbx.NewRepository[dbxPgOrder](q, "dbx_pg_orders")
			require.NoError(s.T(), err)
			refunds, err := dbx.NewRepository[dbxPgRefund](q, "dbx_pg_refunds")
			require.NoError(s.T(), err)

			require.NoError(s.T(), users.Insert(s.ctx,
				&dbxPgUser{Name: "Alice", Email: "alice@x.com"},
				&dbxPgUser{Name: "Bob", Email: "bob@x.com"},
			))
			alice, err := users.GetBy(s.ctx, map[string]any{"email": "alice@x.com"})
			require.NoError(s.T(), err)
			bob, err := users.GetBy(s.ctx, map[string]any{"email": "bob@x.com"})
			require.NoError(s.T(), err)

			require.NoError(s.T(), orders.Insert(s.ctx,
				&dbxPgOrder{UserID: alice.ID, Amount: 10},
				&dbxPgOrder{UserID: alice.ID, Amount: 20},
				&dbxPgOrder{UserID: bob.ID, Amount: 5},
			))
			require.NoError(s.T(), refunds.Insert(s.ctx, &dbxPgRefund{UserID: alice.ID, Amount: 5}))

			countOf := func(sel *sqlb.SelectBuilder) int64 {
				s.T().Helper()
				sqlStr, args, err := sel.Build(q.Dialect())
				require.NoError(s.T(), err)
				n, err := q.QueryInt64(s.ctx, sqlStr, args...)
				require.NoError(s.T(), err)
				return n
			}

			// Join (INNER): orders joined to refunds by user_id, bound
			// against refunds.amount > 0 - only Alice's 2 order rows match
			// (Bob has no refund).
			joinSel := sqlb.Select(sqlb.CountAll()).
				From(sqlb.Table("dbx_pg_orders").As("o")).
				Join(sqlb.Table("dbx_pg_refunds").As("rf"),
					sqlb.Table("o").Col("user_id").Eq(sqlb.Table("rf").Col("user_id"))).
				Where(sqlb.Col("rf.amount").Gt(0))
			assert.Equal(s.T(), int64(2), countOf(joinSel))

			// LeftJoinUsing: all 3 orders survive (LEFT JOIN), bound
			// against orders.amount > 0.
			leftUsingSel := sqlb.Select(sqlb.CountAll()).
				From("dbx_pg_orders").
				LeftJoinUsing("dbx_pg_refunds", "user_id").
				Where(sqlb.Col("dbx_pg_orders.amount").Gt(0))
			assert.Equal(s.T(), int64(3), countOf(leftUsingSel))

			// With: a CTE of orders with amount over a bound threshold.
			withSel := sqlb.Select("n").From("big").
				With("big", sqlb.Select(sqlb.CountAll().As("n")).From("dbx_pg_orders").Where(sqlb.Col("amount").Gt(9)))
			assert.Equal(s.T(), int64(2), countOf(withSel))

			// WithRecursive: generate 1..3, bound against < 4.
			recSel := sqlb.Select(sqlb.CountAll()).From(
				sqlb.Select("k").From("r").
					WithRecursive("r", sqlb.Select(sqlb.Int(1).As("k")).
						UnionAll(sqlb.Select(sqlb.Raw("k + 1")).From("r").Where(sqlb.Col("k").Lt(4)))).
					As("gen"))
			assert.Equal(s.T(), int64(4), countOf(recSel))

			// Union: de-duplicates; two disjoint bound name filters.
			unionSel := sqlb.Select(sqlb.CountAll()).From(
				sqlb.Select("id").From("dbx_pg_users").Where(sqlb.Col("name").Eq("Alice")).
					Union(sqlb.Select("id").From("dbx_pg_users").Where(sqlb.Col("name").Eq("Bob"))).
					As("u"))
			assert.Equal(s.T(), int64(2), countOf(unionSel))

			// UnionAll: same shape, no de-dup needed since disjoint.
			unionAllSel := sqlb.Select(sqlb.CountAll()).From(
				sqlb.Select("id").From("dbx_pg_users").Where(sqlb.Col("name").Eq("Alice")).
					UnionAll(sqlb.Select("id").From("dbx_pg_users").Where(sqlb.Col("name").Eq("Bob"))).
					As("u"))
			assert.Equal(s.T(), int64(2), countOf(unionAllSel))

			// Offset without Limit: 3 orders with amount > 0, skip 1.
			offsetSel := sqlb.Select(sqlb.CountAll()).From(
				sqlb.Select("id").From("dbx_pg_orders").Where(sqlb.Col("amount").Gt(0)).
					OrderBy("id").Offset(1).As("o"))
			assert.Equal(s.T(), int64(2), countOf(offsetSel))

			// Exists subquery: users with at least one order over a bound
			// amount.
			existsSel := sqlb.Select(sqlb.CountAll()).From("dbx_pg_users").
				Where(sqlb.Exists(sqlb.Select(sqlb.Int(1)).From("dbx_pg_orders").
					Where(sqlb.Col("dbx_pg_orders.user_id").Eq(sqlb.Col("dbx_pg_users.id")),
						sqlb.Col("dbx_pg_orders.amount").Gt(9))))
			assert.Equal(s.T(), int64(1), countOf(existsSel))

			// In subquery: users whose id is in the set of order user_ids
			// with amount over a bound value.
			inSel := sqlb.Select(sqlb.CountAll()).From("dbx_pg_users").
				Where(sqlb.Col("id").In(sqlb.Select("user_id").From("dbx_pg_orders").Where(sqlb.Col("amount").Gt(9))))
			assert.Equal(s.T(), int64(1), countOf(inSel))
		})
	}
}
