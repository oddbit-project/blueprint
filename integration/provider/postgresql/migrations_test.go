//go:build integration
// +build integration

package postgresql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	assert "github.com/stretchr/testify/assert"
)

func (s *PGIntegrationTestSuite) TestMigrations() {
	_, err := s.client.Conn.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", pgsql.EngineMigrationTable))
	assert.Nil(s.T(), err)

	src := migrations.NewMemorySource()

	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id int);")
	src.Add("sample3.sql", "insert into sample(id) values(1);")

	mgr, err := pgsql.NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = mgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), "sample3.sql", list[2].Name)
}
