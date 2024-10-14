package pgsql

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestLockMultipleConnections(t *testing.T) {
	client := dbClient(t)
	assert.Nil(t, client.Connect())
	defer client.Disconnect()

	conn1 := client.Db()
	conn2 := client.Db()

	lockId := 12
	lock1, err := NewAdvisoryLock(context.Background(), conn1, lockId)
	assert.Nil(t, err)
	lock2, err := NewAdvisoryLock(context.Background(), conn2, lockId)
	assert.Nil(t, err)

	// lock using conn1
	assert.Nil(t, lock1.Lock(context.Background()))

	// attempt re-lock with conn2, should fail
	locked, err := lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// unlock using conn1, should work
	assert.Nil(t, lock1.Unlock(context.Background()))

	//  try lock with conn2, should work
	locked, err = lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.True(t, locked)

	// attempt re-lock with conn1, should fail
	locked, err = lock1.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// unlock using conn2, should work
	assert.Nil(t, lock2.Unlock(context.Background()))
}

func TestLockConcurrent(t *testing.T) {
	client := dbClient(t)
	assert.Nil(t, client.Connect())
	defer client.Disconnect()

	conn1 := client.Db()
	conn2 := client.Db()

	lockId := 27
	lock1, err := NewAdvisoryLock(context.Background(), conn1, lockId)
	assert.Nil(t, err)
	lock2, err := NewAdvisoryLock(context.Background(), conn2, lockId)
	assert.Nil(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	assert.Nil(t, lock1.Lock(context.Background()))
	time.AfterFunc(time.Second*1, func() {
		lock1.Unlock(context.Background())
		wg.Done()
	})

	// should not work
	locked, err := lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	wg.Wait()

	// now conn2 can acquire lock
	locked, err = lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.True(t, locked)

	// and conn1 cannot
	locked, err = lock1.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// unlock everything
	assert.Nil(t, lock2.Unlock(context.Background()))
}

func TestLockUnlock(t *testing.T) {
	client := dbClient(t)
	assert.Nil(t, client.Connect())
	defer client.Disconnect()

	lockId := 10
	lock, err := NewAdvisoryLock(context.Background(), client.Db(), lockId)
	assert.Nil(t, err)
	// lock
	assert.Nil(t, lock.Lock(context.Background()))

	// unlock
	assert.Nil(t, lock.Unlock(context.Background()))

	// attempt re-lock again
	locked, err := lock.TryLock(context.Background())
	assert.Nil(t, err)
	assert.True(t, locked) // should succeed

	// finally, unlock
	assert.Nil(t, lock.Unlock(context.Background()))
}
