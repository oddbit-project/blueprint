package dbx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/gohan"
)

const dbxUsersDDL = `create table users(id serial primary key, name text not null, email text unique not null)`

const gridRowsDDL = `create table grid_rows(id serial primary key, name text not null, email text not null, tag text not null, note text not null)`

// gridRow is the record type for TestGridIntegration.
type gridRow struct {
	ID    int64  `db:"id,auto" json:"id" grid:"sort,filter"`
	Name  string `db:"name" json:"name" grid:"search,sort"`
	Email string `db:"email" json:"email" grid:"search"`
	Tag   string `db:"tag" json:"tag" grid:"filter"`
	Note  string `db:"note" json:"note"`
}

// dbxUser is the record type for the integration suite's "users" table.
type dbxUser struct {
	ID    int64  `db:"id,auto"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

// clientVariant names one of the three PostgreSQL wire-protocol/setting
// combinations the suite runs every test against, mirroring
// provider/pgsql/integration_test.go's TestStringEscaping.
type clientVariant struct {
	name string
	dsn  string
	scs  string // expected "SHOW standard_conforming_strings" value
}

// DbxIntegrationSuite runs dbx.Repository/Querier against a real PostgreSQL
// testcontainer, exercising what sqlmock cannot: real value round-tripping
// (backslash/quote payloads) under both simple and extended protocol, and
// with standard_conforming_strings both on and off.
type DbxIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	container   testcontainers.Container
	pgInstance  *postgres.PostgresContainer
	dsn         string
	adminClient *db.SqlClient
}

func TestDbxIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("integration suite requires Docker; skipped under -short")
	}
	suite.Run(t, new(DbxIntegrationSuite))
}

func (s *DbxIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	var err error
	s.pgInstance, err = postgres.Run(s.ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("dbx"),
		postgres.WithUsername("dbx"),
		postgres.WithPassword("password"),
		// The postgres:alpine image restarts once during first-run init:
		// it binds 5432, runs initdb, then restarts. Wait for the "ready"
		// log line to appear twice (init + real startup) to avoid racing
		// the restart.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(s.T(), err, "failed to start PostgreSQL container")
	s.container = s.pgInstance.Container

	s.dsn, err = s.pgInstance.ConnectionString(s.ctx, "sslmode=disable", "default_query_exec_mode=simple_protocol")
	require.NoError(s.T(), err, "failed to get connection string")

	s.adminClient = db.NewSqlClient(s.dsn, "pgx", nil)
	require.NoError(s.T(), s.adminClient.Connect())
	_, err = s.adminClient.Conn.ExecContext(s.ctx, dbxUsersDDL)
	require.NoError(s.T(), err, "failed to create users table")
}

func (s *DbxIntegrationSuite) TearDownSuite() {
	if s.adminClient != nil && s.adminClient.IsConnected() {
		s.adminClient.Disconnect()
	}
	if s.container != nil {
		if err := s.container.Terminate(s.ctx); err != nil {
			s.T().Logf("failed to terminate container: %v", err)
		}
	}
}

// resetTable truncates users and restarts its identity sequence, so each
// subtest starts from an empty table regardless of client variant.
func (s *DbxIntegrationSuite) resetTable() {
	_, err := s.adminClient.Conn.ExecContext(s.ctx, "TRUNCATE users RESTART IDENTITY")
	require.NoError(s.T(), err)
}

// clientVariants returns the three DSN variants every test runs against.
func (s *DbxIntegrationSuite) clientVariants() []clientVariant {
	baseDSN, err := s.pgInstance.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)
	return []clientVariant{
		{"simple protocol", s.dsn, "on"},
		{"extended protocol", baseDSN, "on"},
		{"extended protocol, standard_conforming_strings off", baseDSN + "&standard_conforming_strings=off", "off"},
	}
}

// newQuerier builds a *SQLQuerier for dsn via FromClient, registering the
// underlying client's Disconnect as test cleanup.
func (s *DbxIntegrationSuite) newQuerier(dsn string) *SQLQuerier {
	c := db.NewSqlClient(dsn, "pgx", nil)
	q, err := FromClient(c)
	require.NoError(s.T(), err)
	s.T().Cleanup(c.Disconnect)
	return q
}

// assertSCS asserts that q's connection reports the expected
// standard_conforming_strings setting, proving the DSN variant actually
// took effect.
func (s *DbxIntegrationSuite) assertSCS(q *SQLQuerier, want string) {
	var scs string
	require.NoError(s.T(), q.db.QueryRowContext(s.ctx, "SHOW standard_conforming_strings").Scan(&scs))
	require.Equal(s.T(), want, scs)
}

func (s *DbxIntegrationSuite) TestRoundTrip() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "Alice", Email: "alice@x.com"}))

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

			exists, err := r.Exists(s.ctx, gohan.Col("email").Eq("alice@x.com"))
			require.NoError(s.T(), err)
			assert.True(s.T(), exists)

			n, err := r.Update(s.ctx, &dbxUser{Name: "Alice2", Email: "alice@x.com"}, gohan.Col("id").Eq(got.ID))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), n)

			n, err = r.UpdateFields(s.ctx, map[string]any{"name": "Alice3"}, gohan.Col("id").Eq(got.ID))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), n)

			after, err := r.Get(s.ctx, r.Select().Where(gohan.Col("id").Eq(got.ID)))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Alice3", after.Name)

			n, err = r.Delete(s.ctx, gohan.Col("id").Eq(got.ID))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), n)

			cnt, err = r.Count(s.ctx, nil)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(0), cnt)
		})
	}
}

func (s *DbxIntegrationSuite) TestInjectionPayload() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			labels := []string{`a\'b`, `a\nb`, `a\\b`, "plain"}
			for i, label := range labels {
				require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: label, Email: fmt.Sprintf("u%d@x.com", i)}), label)
			}

			for _, label := range labels {
				list, err := r.ListBy(s.ctx, map[string]any{"name": label})
				require.NoError(s.T(), err, label)
				require.Len(s.T(), list, 1, label)
				assert.Equal(s.T(), label, list[0].Name, label)
			}

			const payload = `x\' OR 1=1 --`
			list, err := r.ListBy(s.ctx, map[string]any{"name": payload})
			require.NoError(s.T(), err)
			assert.Len(s.T(), list, 0)

			n, err := r.Delete(s.ctx, gohan.Col("name").Eq(payload))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(0), n)
		})
	}
}

func (s *DbxIntegrationSuite) TestContainsLiteralWildcards() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "50%", Email: "a@x.com"}))
			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "50x", Email: "b@x.com"}))

			list, err := r.List(s.ctx, r.Select().Where(gohan.Col("name").Contains("50%")))
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 1)
			assert.Equal(s.T(), "50%", list[0].Name)
		})
	}
}

func (s *DbxIntegrationSuite) TestCaseSumWithInt() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "plain", Email: "a@x.com"}))
			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "other", Email: "b@x.com"}))
			require.NoError(s.T(), r.Insert(s.ctx, &dbxUser{Name: "plain", Email: "c@x.com"}))

			st := gohan.Select(gohan.Sum(gohan.Case().
				When(gohan.Col("name").Eq("plain"), gohan.Int(1)).
				Else(gohan.Int(0)))).From("users")
			sqlStr, args, err := st.Build(q.Dialect())
			require.NoError(s.T(), err)

			n, err := q.QueryInt64(s.ctx, sqlStr, args...)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(2), n)
		})
	}
}

func (s *DbxIntegrationSuite) TestUpsertAndReturning() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			got, err := r.InsertReturning(s.ctx, &dbxUser{Name: "Alice", Email: "alice@x.com"})
			require.NoError(s.T(), err)
			assert.NotZero(s.T(), got.ID)

			require.NoError(s.T(), r.Upsert(s.ctx, &dbxUser{Name: "Alice Updated", Email: "alice@x.com"}, []string{"email"}))

			after, err := r.GetBy(s.ctx, map[string]any{"email": "alice@x.com"})
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Alice Updated", after.Name)
			assert.Equal(s.T(), got.ID, after.ID)

			cnt, err := r.Count(s.ctx, nil)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), cnt)
		})
	}
}

func (s *DbxIntegrationSuite) TestInsertChunkedLarge() {
	s.resetTable()
	q := s.newQuerier(s.dsn)
	r, err := NewRepository[dbxUser](q, "users")
	require.NoError(s.T(), err)

	const total = 30000
	records := make([]*dbxUser, total)
	for i := range records {
		records[i] = &dbxUser{Name: fmt.Sprintf("u%d", i), Email: fmt.Sprintf("u%d@x.com", i)}
	}
	require.NoError(s.T(), r.Insert(s.ctx, records...))

	n, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(total), n)
}

func (s *DbxIntegrationSuite) TestWithTxRollback() {
	s.resetTable()
	q := s.newQuerier(s.dsn)
	r, err := NewRepository[dbxUser](q, "users")
	require.NoError(s.T(), err)

	fnErr := errors.New("boom")
	err = WithTx(s.ctx, q, nil, func(tx Querier) error {
		if err := r.With(tx).Insert(s.ctx, &dbxUser{Name: "ghost", Email: "ghost@x.com"}); err != nil {
			return err
		}
		return fnErr
	})
	require.Error(s.T(), err)
	assert.True(s.T(), errors.Is(err, fnErr))

	n, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), n)
}

// TestGridIntegration exercises Grid[gridRow]/Repository.QueryGrid end to
// end against PostgreSQL on all three client variants: literal LIKE
// wildcards ("%", "_"), a filter+sort+paging combination, and a
// JSON-decoded GridQuery (as it would arrive in an HTTP request body).
func (s *DbxIntegrationSuite) TestGridIntegration() {
	_, err := s.adminClient.Conn.ExecContext(s.ctx, gridRowsDDL)
	require.NoError(s.T(), err, "failed to create grid_rows table")
	defer func() {
		_, err := s.adminClient.Conn.ExecContext(s.ctx, "DROP TABLE grid_rows")
		require.NoError(s.T(), err, "failed to drop grid_rows table")
	}()

	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			_, err := s.adminClient.Conn.ExecContext(s.ctx, "TRUNCATE grid_rows RESTART IDENTITY")
			require.NoError(s.T(), err)

			q := s.newQuerier(v.dsn)
			s.assertSCS(q, v.scs)

			r, err := NewRepository[gridRow](q, "grid_rows")
			require.NoError(s.T(), err)

			g, err := NewGrid[gridRow]()
			require.NoError(s.T(), err)

			names := []string{"50%", "50x", "a_b", "axb"}
			for i, n := range names {
				require.NoError(s.T(), r.Insert(s.ctx, &gridRow{Name: n, Email: fmt.Sprintf("u%d@x.com", i), Tag: "t"}), n)
			}

			// literal "%" is not a wildcard: only the row literally named
			// "50%" matches, not "50x".
			list, err := r.QueryGrid(s.ctx, g, &GridQuery{SearchType: SearchAny, SearchText: "50%"})
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 1)
			assert.Equal(s.T(), "50%", list[0].Name)

			// literal "_" is not a single-char wildcard: only "a_b"
			// matches, not "axb".
			list, err = r.QueryGrid(s.ctx, g, &GridQuery{SearchType: SearchAny, SearchText: "a_b"})
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 1)
			assert.Equal(s.T(), "a_b", list[0].Name)

			// filter + sort + paging: all four rows share tag "t"; sorted
			// by id ascending, limit 2 offset 1 returns ids 2 and 3.
			list, err = r.QueryGrid(s.ctx, g, &GridQuery{
				FilterFields: map[string]any{"tag": "t"},
				SortFields:   map[string]string{"id": "asc"},
				Limit:        2,
				Offset:       1,
			})
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 2)
			assert.Equal(s.T(), []int64{2, 3}, []int64{list[0].ID, list[1].ID})

			// a GridQuery decoded from a literal JSON request body, end to
			// end: SearchAny "50" matches both "50%" and "50x", ordered by
			// id ascending.
			var jq GridQuery
			body := []byte(`{"searchType":3,"searchText":"50","sortFields":{"id":"asc"}}`)
			require.NoError(s.T(), json.Unmarshal(body, &jq))
			list, err = r.QueryGrid(s.ctx, g, &jq)
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 2)
			assert.Equal(s.T(), "50%", list[0].Name)
			assert.Equal(s.T(), "50x", list[1].Name)
		})
	}
}
