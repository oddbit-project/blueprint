package sqlite

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *SQLiteIntegrationTestSuite) TestMigrations() {
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

func (s *SQLiteIntegrationTestSuite) TestUpdateMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))

	// create old version table (no module column)
	qry := fmt.Sprintf(`CREATE TABLE %s (
			created DATETIME,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		MigrationTable)
	_, err = s.client.Db().ExecContext(s.ctx, qry)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id int);")
	src.Add("sample3.sql", "drop table if exists sample;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), "sample3.sql", list[2].Name)
}

func (s *SQLiteIntegrationTestSuite) TestModuleMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "drop table if exists sample;")
	systemSrc.Add("sample2.sql", "create table sample(id int);")
	systemSrc.Add("sample3.sql", "drop table if exists sample;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("module1.sql", "drop table if exists module;")
	moduleSrc.Add("module2.sql", "create table module(id int);")
	moduleSrc.Add("module3.sql", "drop table if exists module;")

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

func (s *SQLiteIntegrationTestSuite) TestSameNameMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "select 1;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("sample1.sql", "select 1-2;")

	sysMgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	sysList, err := sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))
	assert.Nil(s.T(), err)

	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	sysList, err = sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))

	moduleMgr, err := NewMigrationManager(context.Background(), s.client, WithModule("sample-module"))
	assert.Nil(s.T(), err)

	moduleList, err := moduleMgr.List(context.Background())
	assert.Zero(s.T(), len(moduleList))
	assert.Nil(s.T(), err)

	err = moduleMgr.Run(context.Background(), moduleSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	moduleList, err = moduleMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(moduleList))
	assert.Nil(s.T(), err)

	assert.NotEqual(s.T(), sysList[0].SHA2, moduleList[0].SHA2)
	assert.NotEqual(s.T(), sysList[0].Module, moduleList[0].Module)
}

func (s *SQLiteIntegrationTestSuite) TestSubstitutedMigrations() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	assert.Nil(s.T(), err)

	const template = "create table ${tableName}(id int);"
	src := migrations.NewMemorySource()
	src.Add("sample1.sql", template)

	plain, err := src.Read("sample1.sql")
	assert.Nil(s.T(), err)

	mgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// a placeholder nothing supplies is an error, and this migration is not applied
	// (Run applies migrations one by one; earlier ones stay applied)
	err = mgr.Run(context.Background(), migrations.Substitute(src, migrations.Vars{"other": "x"}), migrations.DefaultProgressFn)
	assert.ErrorIs(s.T(), err, migrations.ErrMissingVar)

	list, err := mgr.List(context.Background())
	assert.Nil(s.T(), err)
	assert.Zero(s.T(), len(list))

	err = mgr.Run(context.Background(), migrations.Substitute(src, migrations.Vars{"tableName": "substituted"}), migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = mgr.List(context.Background())
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), 1, len(list))
	// contents are what ran, the hash is the template's
	assert.Equal(s.T(), "create table substituted(id int);", list[0].Contents)
	assert.Equal(s.T(), plain.SHA2, list[0].SHA2)

	// the table the substituted DDL named exists
	exists, err := TableExists(s.ctx, s.client, "substituted")
	assert.Nil(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *SQLiteIntegrationTestSuite) TestMigrationExists() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "select 1;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)
	require.Nil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	applied, err := mgr.List(context.Background())
	require.Nil(s.T(), err)
	require.Equal(s.T(), 1, len(applied))

	// never applied
	exists, err := mgr.MigrationExists(context.Background(), "absent.sql", applied[0].SHA2)
	assert.Nil(s.T(), err)
	assert.False(s.T(), exists)

	// applied, same contents
	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", applied[0].SHA2)
	assert.Nil(s.T(), err)
	assert.True(s.T(), exists)

	// applied under the same name, different contents
	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", "not-the-recorded-hash")
	assert.ErrorIs(s.T(), err, migrations.ErrMigrationNameHashMismatch)
	assert.True(s.T(), exists)

	// RunMigration refuses both an already-applied migration and an edited one
	record, err := src.Read("sample1.sql")
	require.Nil(s.T(), err)
	assert.ErrorIs(s.T(), mgr.RunMigration(context.Background(), record), migrations.ErrMigrationExists)

	edited := *record
	edited.SHA2 = "not-the-recorded-hash"
	assert.ErrorIs(s.T(), mgr.RunMigration(context.Background(), &edited), migrations.ErrMigrationNameHashMismatch)

	// a migration under a name nothing recorded is applied and registered
	fresh := migrations.NewMemorySource()
	fresh.Add("sample2.sql", "select 2;")
	record, err = fresh.Read("sample2.sql")
	require.Nil(s.T(), err)
	assert.Nil(s.T(), mgr.RunMigration(context.Background(), record))

	applied, err = mgr.List(context.Background())
	require.Nil(s.T(), err)
	assert.Equal(s.T(), 2, len(applied))
}

// A failed lookup of the applied set must abort the run: treating it as "nothing
// applied" re-executes every migration.
func (s *SQLiteIntegrationTestSuite) TestRunFailsWhenAppliedListIsUnreadable() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	require.Nil(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS unreadable_list")
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table unreadable_list(id int);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	// remove the migration table under the manager
	_, err = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE %s", MigrationTable))
	require.Nil(s.T(), err)

	assert.NotNil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	// the migration must not have run
	exists, err := TableExists(s.ctx, s.client, "unreadable_list")
	require.Nil(s.T(), err)
	assert.False(s.T(), exists)
}

// A migration that runs but cannot be recorded is the documented
// ErrRegisterMigration case, and must be told apart from a failed migration.
func (s *SQLiteIntegrationTestSuite) TestRegistrationFailureIsReported() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	require.Nil(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS unregistered_sample")
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table unregistered_sample(id int);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	// make registration, and only registration, fail
	_, err = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf(
		`CREATE TRIGGER no_register BEFORE INSERT ON %s BEGIN SELECT RAISE(ABORT, 'no'); END`, MigrationTable))
	require.Nil(s.T(), err)
	defer func() {
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TRIGGER IF EXISTS no_register")
	}()

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.ErrorIs(s.T(), err, migrations.ErrRegisterMigration)

	// the migration itself did run
	exists, err := TableExists(s.ctx, s.client, "unregistered_sample")
	require.Nil(s.T(), err)
	assert.True(s.T(), exists)
}
