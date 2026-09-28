package clickhouse

import (
	"errors"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
)

// chkSwapped maps a<->b differently under ch and db: the driver silently
// scans column a into B and b into A.
type chkSwapped struct {
	A string `ch:"b" db:"a"`
	B string `ch:"a" db:"b"`
}

// TestCheckRecordAgainstDriver pins CheckRecord's verdicts to what
// clickhouse-go actually does: an accepted embedded record round-trips
// through a repository, and a rejected record really is scanned into the
// wrong fields by the driver.
func (s *ClickhouseRepositoryTestSuite) TestCheckRecordAgainstDriver() {
	const table = "chk_driver"
	s.createTable(table, `
CREATE TABLE %s (
	id      UInt32,
	created DateTime64(3, 'UTC'),
	owner   String
) ENGINE = MergeTree ORDER BY id
`)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[chkNested](q, table)
	require.NoError(s.T(), err)
	at := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	in := &chkNested{ID: 7}
	in.Owner = "alice"
	in.Created = at
	require.NoError(s.T(), r.Insert(s.ctx, in))
	got, err := r.Get(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint32(7), got.ID)
	assert.Equal(s.T(), "alice", got.Owner)
	assert.True(s.T(), at.Equal(got.Created), "got %v", got.Created)

	_, err = dbx.NewRepository[chkSwapped](q, table)
	assert.True(s.T(), errors.Is(err, ErrRecordMapping), "got %v", err)

	var out chkSwapped
	require.NoError(s.T(), q.Get(s.ctx, &out, "SELECT 'va' AS a, 'vb' AS b"))
	assert.Equal(s.T(), chkSwapped{A: "vb", B: "va"}, out, "driver maps by ch tag")
}
