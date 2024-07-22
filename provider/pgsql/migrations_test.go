package pgsql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMigrations(t *testing.T) {
	pool := dbClient(t)
	_, err := pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", EngineSchemaTable))
	assert.Nil(t, err)
	src := migrations.NewMemorySource()

	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id int);")
	src.Add("sample3.sql", "insert into sample(id) values(1);")

	mgr := NewMigrationManager(pool)

	list, err := mgr.List(context.Background())
	assert.Zero(t, len(list))
	assert.Nil(t, err)

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(t, err)

	list, err = mgr.List(context.Background())
	assert.Equal(t, 3, len(list))
	assert.Nil(t, err)

	assert.Equal(t, "sample1.sql", list[0].Name)

}
