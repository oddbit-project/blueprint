package pgsql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	assert "github.com/stretchr/testify/assert"
)

func (s *PGIntegrationTestSuite) TestMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	src := migrations.NewMemorySource()

	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id int);")
	src.Add("sample3.sql", "drop table if exists sample;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
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

func (s *PGIntegrationTestSuite) TestUpdateMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))

	// create old version table
	qry := fmt.Sprintf(`CREATE TABLE  %s (
			created TIMESTAMP WITH TIME ZONE,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineMigrationTable)
	_, err = s.client.Db().ExecContext(s.ctx, qry)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id int);")
	src.Add("sample3.sql", "drop table if exists sample;")

	// create new migration manager, should update table
	mgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// run migrations
	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), "sample3.sql", list[2].Name)
}

func (s *PGIntegrationTestSuite) TestModuleMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "drop table if exists sample;")
	systemSrc.Add("sample2.sql", "create table sample(id int);")
	systemSrc.Add("sample3.sql", "drop table if exists sample;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("module1.sql", "drop table if exists module;")
	moduleSrc.Add("module2.sql", "create table module(id int);")
	moduleSrc.Add("module3.sql", "drop table if exists module;")

	// migration manager - base module (default)
	sysMgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	list, err := sysMgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = sysMgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), migrations.ModuleBase, list[0].Module)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), migrations.ModuleBase, list[1].Module)

	// migration manager - module
	moduleMgr, err := NewMigrationManager(context.Background(), s.client, WithModule("sample-module"))
	assert.Nil(s.T(), err)

	list, err = moduleMgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = moduleMgr.Run(context.Background(), moduleSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = moduleMgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "module1.sql", list[0].Name)
	assert.Equal(s.T(), "sample-module", list[0].Module)
	assert.Equal(s.T(), "module2.sql", list[1].Name)
	assert.Equal(s.T(), "sample-module", list[1].Module)
	assert.Equal(s.T(), "module3.sql", list[2].Name)
	assert.Equal(s.T(), "sample-module", list[2].Module)
}

func (s *PGIntegrationTestSuite) TestSameNameMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "select 1;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("sample1.sql", "select 1-2;")

	// migration manager - base module (default)
	sysMgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// run migrations
	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// list, should have 1
	sysList, err := sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))
	assert.Nil(s.T(), err)

	// run migrations again, should do nothing
	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// list, should still have 1
	sysList, err = sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))

	// migration manager - module
	moduleMgr, err := NewMigrationManager(context.Background(), s.client, WithModule("sample-module"))
	assert.Nil(s.T(), err)

	moduleList, err := moduleMgr.List(context.Background())
	assert.Zero(s.T(), len(moduleList))
	assert.Nil(s.T(), err)

	err = moduleMgr.Run(context.Background(), moduleSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// module should have 1
	moduleList, err = moduleMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(moduleList))
	assert.Nil(s.T(), err)

	assert.NotEqual(s.T(), sysList[0].SHA2, moduleList[0].SHA2)
	assert.NotEqual(s.T(), sysList[0].Module, moduleList[0].Module)
}
