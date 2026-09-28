package dbx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
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

// TestQueryGridWithCount checks that the total counts every row the grid
// query's filters and search match, ignoring its sort and paging (an
// OFFSET past the first row must not zero the count).
func (s *DbxIntegrationSuite) TestQueryGridWithCount() {
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
			r, err := NewRepository[gridRow](q, "grid_rows")
			require.NoError(s.T(), err)
			g, err := NewGrid[gridRow]()
			require.NoError(s.T(), err)

			for i, n := range []string{"ann", "anna", "bob", "annie", "carl"} {
				tag := "a"
				if i == 1 {
					tag = "b"
				}
				require.NoError(s.T(), r.Insert(s.ctx, &gridRow{Name: n, Email: fmt.Sprintf("u%d@x.com", i), Tag: tag}))
			}

			// tag "a" and name/email containing "ann": ann (1) and annie
			// (4); anna has tag "b". Offset 1 leaves one row.
			rows, total, err := r.QueryGridWithCount(s.ctx, g, &GridQuery{
				FilterFields: map[string]any{"tag": "a"},
				SearchType:   SearchAny,
				SearchText:   "ann",
				Sort:         []SortField{{Field: "id", Order: SortAscending}},
				Limit:        1,
				Offset:       1,
			})
			require.NoError(s.T(), err)
			require.Len(s.T(), rows, 1)
			assert.Equal(s.T(), "annie", rows[0].Name)
			assert.Equal(s.T(), int64(2), total)

			rows, total, err = r.QueryGridWithCount(s.ctx, g, &GridQuery{Limit: 2, Offset: 10})
			require.NoError(s.T(), err)
			assert.Empty(s.T(), rows)
			assert.Equal(s.T(), int64(5), total)
		})
	}
}

func (s *DbxIntegrationSuite) TestInsertIgnore() {
	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			s.resetTable()
			q := s.newQuerier(v.dsn)
			r, err := NewRepository[dbxUser](q, "users")
			require.NoError(s.T(), err)

			ok, err := r.InsertIgnore(s.ctx, &dbxUser{Name: "Alice", Email: "alice@x.com"}, "email")
			require.NoError(s.T(), err)
			assert.True(s.T(), ok)

			ok, err = r.InsertIgnore(s.ctx, &dbxUser{Name: "Other", Email: "alice@x.com"}, "email")
			require.NoError(s.T(), err)
			assert.False(s.T(), ok)

			ok, err = r.InsertIgnore(s.ctx, &dbxUser{Name: "Other", Email: "alice@x.com"})
			require.NoError(s.T(), err)
			assert.False(s.T(), ok, "untargeted DO NOTHING ignores the conflict too")

			got, err := r.GetBy(s.ctx, map[string]any{"email": "alice@x.com"})
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Alice", got.Name)
			n, err := r.Count(s.ctx, nil)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), int64(1), n)
		})
	}
}

// retRow is the record type for TestReturning: version is filled by the
// database (a default on insert, a trigger on update).
type retRow struct {
	ID      int64  `db:"id,auto"`
	Email   string `db:"email"`
	Name    string `db:"name"`
	Version int64  `db:"version,auto"`
}

const retRowsDDL = `
create table ret_rows(id serial primary key, email text unique not null, name text not null, version int not null default 1, unique (email, name));
create function ret_rows_bump() returns trigger language plpgsql as $$
begin
	new.version := old.version + 1;
	return new;
end $$;
create trigger ret_rows_bump before update on ret_rows for each row execute function ret_rows_bump();`

// TestReturning checks that UpsertReturning/UpdateReturning read back what
// the database stored, not what was sent.
func (s *DbxIntegrationSuite) TestReturning() {
	_, err := s.adminClient.Conn.ExecContext(s.ctx, retRowsDDL)
	require.NoError(s.T(), err, "failed to create ret_rows")
	defer func() {
		_, err := s.adminClient.Conn.ExecContext(s.ctx, "DROP TABLE ret_rows; DROP FUNCTION ret_rows_bump()")
		require.NoError(s.T(), err, "failed to drop ret_rows")
	}()

	for _, v := range s.clientVariants() {
		s.Run(v.name, func() {
			_, err := s.adminClient.Conn.ExecContext(s.ctx, "TRUNCATE ret_rows RESTART IDENTITY")
			require.NoError(s.T(), err)
			q := s.newQuerier(v.dsn)
			r, err := NewRepository[retRow](q, "ret_rows")
			require.NoError(s.T(), err)

			rec := &retRow{Email: "a@x.com", Name: "A"}
			ins, err := r.UpsertReturning(s.ctx, rec, []string{"email"})
			require.NoError(s.T(), err)
			assert.NotZero(s.T(), ins.ID)
			assert.Equal(s.T(), int64(1), ins.Version, "column default")
			assert.Zero(s.T(), rec.ID, "caller's record is not written")

			upd, err := r.UpsertReturning(s.ctx, &retRow{Email: "a@x.com", Name: "A2"}, []string{"email"})
			require.NoError(s.T(), err)
			assert.Equal(s.T(), ins.ID, upd.ID)
			assert.Equal(s.T(), "A2", upd.Name)
			assert.Equal(s.T(), int64(2), upd.Version, "update trigger")

			_, err = r.UpsertReturning(s.ctx, &retRow{Email: "a@x.com", Name: "A2"}, []string{"email", "name"})
			assert.True(s.T(), errors.Is(err, ErrNotFound), "DO NOTHING on an existing row: got %v", err)

			list, err := r.UpdateReturning(s.ctx, map[string]any{"name": "A3"}, gohan.Col("email").Eq("a@x.com"))
			require.NoError(s.T(), err)
			require.Len(s.T(), list, 1)
			assert.Equal(s.T(), retRow{ID: ins.ID, Email: "a@x.com", Name: "A3", Version: 3}, *list[0])

			list, err = r.UpdateReturning(s.ctx, map[string]any{"name": "A4"}, gohan.Col("email").Eq("missing@x.com"))
			require.NoError(s.T(), err)
			assert.Empty(s.T(), list)
		})
	}
}

// grpRow is the record type for TestGroupedInsert: every column is
// omittable, so a record can write any subset, including none.
type grpRow struct {
	ID   int64   `db:"id,auto"`
	Name *string `db:"name" goqu:"omitnil"`
	Bio  *string `db:"bio" goqu:"omitnil"`
}

const grpRowsDDL = `create table grp_rows(id serial primary key, name text unique, bio text not null default 'none')`

func (s *DbxIntegrationSuite) TestGroupedInsert() {
	_, err := s.adminClient.Conn.ExecContext(s.ctx, grpRowsDDL)
	require.NoError(s.T(), err, "failed to create grp_rows")
	defer func() {
		_, err := s.adminClient.Conn.ExecContext(s.ctx, "DROP TABLE grp_rows")
		require.NoError(s.T(), err, "failed to drop grp_rows")
	}()

	str := func(v string) *string { return &v }
	q := s.newQuerier(s.dsn)
	r, err := NewRepository[grpRow](q, "grp_rows")
	require.NoError(s.T(), err)

	records := []*grpRow{{Name: str("a")}, {Name: str("b"), Bio: str("bb")}, {}, {Name: str("c")}, {Bio: str("x")}}
	err = r.Insert(s.ctx, records...)
	assert.True(s.T(), errors.Is(err, gohan.ErrInconsistentOmit), "default Insert: got %v", err)

	require.NoError(s.T(), r.WithGroupedInserts().Insert(s.ctx, records...))

	got, err := r.List(s.ctx, r.Select().OrderBy(gohan.Col("id").Asc()))
	require.NoError(s.T(), err)
	require.Len(s.T(), got, 5)
	type nb struct{ name, bio string }
	flat := make([]nb, len(got))
	for i, g := range got {
		if g.Name != nil {
			flat[i].name = *g.Name
		}
		flat[i].bio = *g.Bio
	}
	// groups in order of their first record: {name}, {name,bio}, {}, {bio}
	assert.Equal(s.T(), []nb{{"a", "none"}, {"c", "none"}, {"b", "bb"}, {"", "none"}, {"", "x"}}, flat)

	// a failing group rolls back the groups already written
	err = r.WithGroupedInserts().Insert(s.ctx, &grpRow{Name: str("d")}, &grpRow{Name: str("a"), Bio: str("dup")})
	require.Error(s.T(), err)
	n, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), n)
}

// --- keyset pagination ---

// ksPgRow is the record type for the keyset pagination tests.
type ksPgRow struct {
	ID           int64     `db:"id,auto" json:"id" grid:"sort,filter"`
	Score        int32     `db:"score" json:"score" grid:"sort,filter"`
	Name         string    `db:"name" json:"name" grid:"sort,search"`
	Created      time.Time `db:"created" json:"created" grid:"sort"`
	CreatedNaive time.Time `db:"created_naive" json:"createdNaive" grid:"sort"`
	UID          uuid.UUID `db:"uid" json:"uid" grid:"sort"`
}

const ksPgRowsDDL = `create table keyset_rows(id bigserial primary key, score int not null, name text not null,
	created timestamptz not null, created_naive timestamp not null, uid uuid not null)`

// ksNames has multi-byte and case-variant names, repeated across rows.
var ksNames = []string{"alice", "Alice", "ALICE", "Ålund", "ábaco", "zebra", "Zoë", "日本", "b", "émile", "Émile"}

// ksPgSeed inserts n rows: scores repeat every 13 rows, created takes 20
// distinct microsecond values, created_naive 7 distinct values.
func ksPgSeed(n int) []*ksPgRow {
	base := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := make([]*ksPgRow, n)
	for i := range rows {
		rows[i] = &ksPgRow{
			Score:        int32(i % 13),
			Name:         ksNames[i%len(ksNames)],
			Created:      base.Add(time.Duration(i%20) * 1234567 * time.Microsecond),
			CreatedNaive: base.Add(time.Duration(i%7) * 1500 * time.Microsecond),
			UID:          uuid.New(),
		}
	}
	return rows
}

// ksWalk fetches pages until HasMore is false, checking the page shape:
// every page but the last is full, HasMore is set exactly when NextCursor
// is, and the last page has no cursor.
func ksWalk[T any](t *testing.T, pageSize int, fetch func(cursor string) (*KeysetPage[T], error)) []*T {
	t.Helper()
	var all []*T
	cursor := ""
	for i := 0; ; i++ {
		require.Less(t, i, 10000, "walk does not end")
		page, err := fetch(cursor)
		require.NoError(t, err)
		require.NotNil(t, page.Items)
		assert.Equal(t, page.HasMore, page.NextCursor != "")
		all = append(all, page.Items...)
		if !page.HasMore {
			assert.LessOrEqual(t, len(page.Items), pageSize)
			return all
		}
		require.Len(t, page.Items, pageSize)
		cursor = page.NextCursor
	}
}

// ksOrdered lists every row of r ordered by keys.
func ksOrdered[T any](t *testing.T, ctx context.Context, r *Repository[T], keys []KeysetKey) []*T {
	t.Helper()
	sel := r.Select()
	for _, k := range keys {
		if k.Desc {
			sel = sel.OrderBy(gohan.Col(k.Column).Desc())
		} else {
			sel = sel.OrderBy(gohan.Col(k.Column).Asc())
		}
	}
	rows, err := r.List(ctx, sel)
	require.NoError(t, err)
	return rows
}

func ksPgIDs(rows []*ksPgRow) []int64 {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

// ksForge returns cursor with its key values replaced by keys (fingerprint
// kept), as a client could.
func ksForge(t *testing.T, cursor string, keys ...string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	require.NoError(t, err)
	var pl cursorPayload
	require.NoError(t, json.Unmarshal(raw, &pl))
	pl.K = keys
	b, err := marshalPayload(pl)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *DbxIntegrationSuite) TestKeysetPagination() {
	_, err := s.adminClient.Conn.ExecContext(s.ctx, ksPgRowsDDL)
	require.NoError(s.T(), err, "failed to create keyset_rows")
	defer func() {
		_, err := s.adminClient.Conn.ExecContext(s.ctx, "DROP TABLE keyset_rows")
		require.NoError(s.T(), err, "failed to drop keyset_rows")
	}()

	const total = 257
	keySets := [][]KeysetKey{
		{KeyAsc("score"), KeyAsc("id")},
		{KeyDesc("score"), KeyAsc("id")},
		{KeyDesc("created"), KeyDesc("id")},
		{KeyAsc("created"), KeyDesc("score"), KeyAsc("id")},
		{KeyAsc("name"), KeyDesc("id")},
		{KeyDesc("created_naive"), KeyAsc("id")},
		{KeyAsc("uid")},
	}

	for _, v := range s.clientVariants()[:2] {
		s.Run(v.name, func() {
			t := s.T()
			_, err := s.adminClient.Conn.ExecContext(s.ctx, "TRUNCATE keyset_rows RESTART IDENTITY")
			require.NoError(t, err)
			q := s.newQuerier(v.dsn)
			r, err := NewRepository[ksPgRow](q, "keyset_rows")
			require.NoError(t, err)
			require.NoError(t, r.Insert(s.ctx, ksPgSeed(total)...))

			// I1: every key set and page size walks the table exactly once,
			// in List's order.
			for _, keys := range keySets {
				want := ksPgIDs(ksOrdered(t, s.ctx, r, keys))
				require.Len(t, want, total)
				for _, size := range []int{1, 7, 50, 300} {
					got := ksWalk(t, size, func(c string) (*KeysetPage[ksPgRow], error) {
						return r.ListKeyset(s.ctx, nil, keys, size, c)
					})
					assert.Equal(t, want, ksPgIDs(got), "keys %v, page size %d", keys, size)
				}
			}

			// the filter is applied with the seek predicate
			where := gohan.Col("score").Lt(5)
			keys := keySets[0]
			got := ksWalk(t, 9, func(c string) (*KeysetPage[ksPgRow], error) {
				return r.ListKeyset(s.ctx, where, keys, 9, c)
			})
			wantRows, err := r.List(s.ctx, r.Select().Where(where).OrderBy(gohan.Col("score").Asc(), gohan.Col("id").Asc()))
			require.NoError(t, err)
			assert.Equal(t, ksPgIDs(wantRows), ksPgIDs(got))

			// I2: rows inserted before the cursor are not seen, rows inserted
			// after it are seen once, deleted rows are not seen.
			first, err := r.ListKeyset(s.ctx, nil, keys, 10, "")
			require.NoError(t, err)
			ordered := ksOrdered(t, s.ctx, r, keys)
			doomed := ordered[len(ordered)-1].ID
			before := &ksPgRow{Score: -1, Name: "before", Created: time.Now().UTC(), CreatedNaive: time.Now().UTC(), UID: uuid.New()}
			afterRow := &ksPgRow{Score: 6, Name: "after", Created: time.Now().UTC(), CreatedNaive: time.Now().UTC(), UID: uuid.New()}
			require.NoError(t, r.Insert(s.ctx, before, afterRow))
			_, err = r.Delete(s.ctx, gohan.Col("id").Eq(doomed))
			require.NoError(t, err)
			rest := ksWalk(t, 10, func(c string) (*KeysetPage[ksPgRow], error) {
				if c == "" {
					c = first.NextCursor
				}
				return r.ListKeyset(s.ctx, nil, keys, 10, c)
			})
			names := map[string]int{}
			for _, row := range append(first.Items, rest...) {
				names[row.Name]++
				assert.NotEqual(t, doomed, row.ID)
			}
			assert.Zero(t, names["before"])
			assert.Equal(t, 1, names["after"])
			assert.Len(t, append(first.Items, rest...), total) // +1 after, -1 deleted

			// I3: a grid walk returns what QueryGrid returns with the cap
			// raised.
			g, err := NewGrid[ksPgRow]()
			require.NoError(t, err)
			g.WithTiebreaker("id").WithMaxLimit(0)
			gq := &GridQuery{
				FilterFields: map[string]any{"score": []any{float64(1), float64(2), float64(3), float64(6)}},
				Sort:         []SortField{{Field: "created", Order: SortDescending}, {Field: "name", Order: SortAscending}},
			}
			wantGrid, err := r.QueryGrid(s.ctx, g, gq)
			require.NoError(t, err)
			require.NotEmpty(t, wantGrid)
			pq := *gq
			pq.Limit = 7
			gotGrid := ksWalk(t, 7, func(c string) (*KeysetPage[ksPgRow], error) {
				return r.QueryGridKeyset(s.ctx, g, &pq, c)
			})
			assert.Equal(t, ksPgIDs(wantGrid), ksPgIDs(gotGrid))

			// I4: forged extreme values give a page or ErrInvalidCursor, never
			// a database error.
			forged := []struct {
				keys []KeysetKey
				vals []string
			}{
				{[]KeysetKey{KeyAsc("score"), KeyAsc("id")}, []string{"2147483647", "9223372036854775807"}},
				{[]KeysetKey{KeyAsc("score"), KeyAsc("id")}, []string{"-2147483648", "-9223372036854775808"}},
				{[]KeysetKey{KeyDesc("created"), KeyDesc("id")}, []string{"0001-01-01T00:00:00Z", "1"}},
				{[]KeysetKey{KeyDesc("created"), KeyDesc("id")}, []string{"9999-12-31T23:59:59.999999Z", "1"}},
				{[]KeysetKey{KeyAsc("created"), KeyDesc("score"), KeyAsc("id")}, []string{"9999-12-31T23:59:59.999999999Z", "0", "0"}},
				{[]KeysetKey{KeyDesc("created_naive"), KeyAsc("id")}, []string{"0001-01-01T00:00:00Z", "0"}},
				{[]KeysetKey{KeyDesc("created_naive"), KeyAsc("id")}, []string{"9999-12-31T23:59:59.999999999Z", "0"}},
				{[]KeysetKey{KeyAsc("uid")}, []string{"ffffffff-ffff-ffff-ffff-ffffffffffff"}},
				{[]KeysetKey{KeyAsc("uid")}, []string{"00000000-0000-0000-0000-000000000000"}},
			}
			for _, f := range forged {
				page, err := r.ListKeyset(s.ctx, nil, f.keys, 1, "")
				require.NoError(t, err)
				cursor := ksForge(t, page.NextCursor, f.vals...)
				_, err = r.ListKeyset(s.ctx, nil, f.keys, 5, cursor)
				if err != nil {
					assert.ErrorIs(t, err, ErrInvalidCursor, "keys %v vals %v", f.keys, f.vals)
				}
			}
		})
	}
}
