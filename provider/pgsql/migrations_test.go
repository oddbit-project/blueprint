package pgsql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"time"
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

// A table upgraded from a pre-module version keeps its rows: they belong to the
// base module, and must not look unapplied.
func (s *PGIntegrationTestSuite) TestUpdateMigrationsKeepsLegacyRows() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)

	qry := fmt.Sprintf(`CREATE TABLE %s (
			created TIMESTAMP WITH TIME ZONE,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineMigrationTable)
	_, err = s.client.Db().ExecContext(s.ctx, qry)
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table legacy_sample(id int);")
	record, err := src.Read("sample1.sql")
	require.Nil(s.T(), err)

	// the migration as an old version would have recorded it
	_, err = s.client.Db().ExecContext(s.ctx,
		fmt.Sprintf(`INSERT INTO %s (created, name, sha2, contents) VALUES (now(), $1, $2, $3)`, EngineMigrationTable),
		record.Name, record.SHA2, record.Contents)
	require.Nil(s.T(), err)

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	require.Nil(s.T(), err)
	require.Equal(s.T(), 1, len(list))
	assert.Equal(s.T(), migrations.ModuleBase, list[0].Module)

	// already applied: running again must not re-execute it
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS legacy_sample")
	require.Nil(s.T(), err)

	require.Nil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	exists, err := TableExists(s.ctx, s.client, "legacy_sample", SchemaDefault)
	require.Nil(s.T(), err)
	assert.False(s.T(), exists)
}

// A table whose module column was added but never filled is repaired on the next
// run; those rows would otherwise be invisible and every migration would re-run.
func (s *PGIntegrationTestSuite) TestUpdateMigrationsRepairsNullModule() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)

	qry := fmt.Sprintf(`CREATE TABLE %s (
			created TIMESTAMP WITH TIME ZONE,
			module TEXT,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineMigrationTable)
	_, err = s.client.Db().ExecContext(s.ctx, qry)
	require.Nil(s.T(), err)

	_, err = s.client.Db().ExecContext(s.ctx,
		fmt.Sprintf(`INSERT INTO %s (created, module, name, sha2, contents) VALUES (now(), NULL, $1, $2, $3)`, EngineMigrationTable),
		"sample1.sql", "somehash", "select 1;")
	require.Nil(s.T(), err)

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	require.Nil(s.T(), err)
	require.Equal(s.T(), 1, len(list))
	assert.Equal(s.T(), migrations.ModuleBase, list[0].Module)
}

// The wait for the migration lock honours the caller's context; releasing it
// does not, so the lock is free again for the next run.
func (s *PGIntegrationTestSuite) TestRunHonoursContextWhileLocked() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS locked_sample")
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table locked_sample(id int);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	// hold the migration lock on another session
	holder := s.getTestClient()
	require.NoError(s.T(), holder.Connect())
	defer holder.Disconnect()
	lock, err := NewAdvisoryLock(s.ctx, holder.Db(), MigrationLockId)
	require.NoError(s.T(), err)
	require.NoError(s.T(), lock.Lock(s.ctx))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err = mgr.Run(ctx, src, migrations.DefaultProgressFn)
	assert.ErrorIs(s.T(), err, context.DeadlineExceeded)

	exists, err := TableExists(s.ctx, s.client, "locked_sample", SchemaDefault)
	require.Nil(s.T(), err)
	assert.False(s.T(), exists)

	// release, and the next run acquires the lock and applies the migration
	require.NoError(s.T(), lock.Unlock(s.ctx))
	lock.Close()

	require.Nil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	exists, err = TableExists(s.ctx, s.client, "locked_sample", SchemaDefault)
	require.Nil(s.T(), err)
	assert.True(s.T(), exists)

	// the lock the successful run took was released
	probe, err := NewAdvisoryLock(s.ctx, s.client.Db(), MigrationLockId)
	require.NoError(s.T(), err)
	defer probe.Close()
	locked, err := probe.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), locked)
	require.NoError(s.T(), probe.Unlock(s.ctx))
}

// A run cancelled while a migration is executing still releases the lock.
func (s *PGIntegrationTestSuite) TestCancelledRunReleasesTheLock() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "select pg_sleep(5);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	assert.NotNil(s.T(), mgr.Run(ctx, src, migrations.DefaultProgressFn))

	probe, err := NewAdvisoryLock(s.ctx, s.client.Db(), MigrationLockId)
	require.NoError(s.T(), err)
	defer probe.Close()
	locked, err := probe.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), locked, "the migration lock was not released")
	require.NoError(s.T(), probe.Unlock(s.ctx))
}

func (s *PGIntegrationTestSuite) TestMigrationExists() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "select 1;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)
	require.Nil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	applied, err := mgr.List(context.Background())
	require.Nil(s.T(), err)
	require.Equal(s.T(), 1, len(applied))

	exists, err := mgr.MigrationExists(context.Background(), "absent.sql", applied[0].SHA2)
	assert.Nil(s.T(), err)
	assert.False(s.T(), exists)

	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", applied[0].SHA2)
	assert.Nil(s.T(), err)
	assert.True(s.T(), exists)

	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", "not-the-recorded-hash")
	assert.ErrorIs(s.T(), err, migrations.ErrMigrationNameHashMismatch)
	assert.True(s.T(), exists)

	record, err := src.Read("sample1.sql")
	require.Nil(s.T(), err)
	assert.ErrorIs(s.T(), mgr.RunMigration(context.Background(), record), migrations.ErrMigrationExists)

	edited := *record
	edited.SHA2 = "not-the-recorded-hash"
	assert.ErrorIs(s.T(), mgr.RunMigration(context.Background(), &edited), migrations.ErrMigrationNameHashMismatch)
}

// A failed lookup of the applied set must abort the run: treating it as "nothing
// applied" re-executes every migration.
func (s *PGIntegrationTestSuite) TestRunFailsWhenAppliedListIsUnreadable() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS unreadable_list")
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table unreadable_list(id int);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	_, err = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE %s", EngineMigrationTable))
	require.Nil(s.T(), err)

	assert.NotNil(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	exists, err := TableExists(s.ctx, s.client, "unreadable_list", SchemaDefault)
	require.Nil(s.T(), err)
	assert.False(s.T(), exists)
}

// A migration that runs but cannot be recorded is the documented
// ErrRegisterMigration case, and must be told apart from a failed migration.
func (s *PGIntegrationTestSuite) TestRegistrationFailureIsReported() {
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineMigrationTable))
	require.Nil(s.T(), err)
	_, err = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS unregistered_sample")
	require.Nil(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table unregistered_sample(id int);")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.Nil(s.T(), err)

	// make registration, and only registration, fail
	_, err = s.client.Db().ExecContext(s.ctx,
		`CREATE OR REPLACE FUNCTION no_register() RETURNS trigger AS $fn$ BEGIN RAISE EXCEPTION 'no'; END; $fn$ LANGUAGE plpgsql`)
	require.Nil(s.T(), err)
	_, err = s.client.Db().ExecContext(s.ctx, fmt.Sprintf(
		`CREATE TRIGGER no_register BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION no_register()`, EngineMigrationTable))
	require.Nil(s.T(), err)
	defer func() {
		_, _ = s.client.Db().ExecContext(s.ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS no_register ON %s", EngineMigrationTable))
		_, _ = s.client.Db().ExecContext(s.ctx, "DROP FUNCTION IF EXISTS no_register()")
	}()

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.ErrorIs(s.T(), err, migrations.ErrRegisterMigration)

	exists, err := TableExists(s.ctx, s.client, "unregistered_sample", SchemaDefault)
	require.Nil(s.T(), err)
	assert.True(s.T(), exists)
}
