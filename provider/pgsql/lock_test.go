package pgsql

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestLockMultipleConnections(t *testing.T) {
	pool1 := dbClient(t)
	pool2 := dbClient(t)

	var lockId int64 = 12
	lock1 := NewAdvisoryLock(pool1, lockId)
	lock2 := NewAdvisoryLock(pool2, lockId)

	// lock using conn1
	locked, err := lock1.TryLock(context.Background())
	assert.Nil(t, err)
	assert.True(t, locked)

	// attempt re-lock with conn2, should fail
	locked, err = lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// unlock using conn1, should work
	assert.Nil(t, lock1.Unlock(context.Background()))

	// try lock with conn2, should work
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
	pool1 := dbClient(t)
	pool2 := dbClient(t)

	var lockId int64 = 2354346345
	lock1 := NewAdvisoryLock(pool1, lockId)
	lock2 := NewAdvisoryLock(pool2, lockId)

	wg := sync.WaitGroup{}
	wg.Add(1)

	// lock using pool1
	assert.Nil(t, lock1.Lock(context.Background()))
	time.AfterFunc(time.Second*1, func() {
		// wait 1s to unlock
		assert.Nil(t, lock1.Unlock(context.Background()))
		wg.Done()
	})

	// attempt to lock with pool2 - should not work
	locked, err := lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// wait for lock1 unlock
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
	pool := dbClient(t)

	var lockId int64 = 10
	lock := NewAdvisoryLock(pool, lockId)
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

func TestLockConcurrentDifferentConnections(t *testing.T) {
	pool1 := dbClient(t)
	pool2 := dbClient(t)

	var lockId int64 = 2354346345
	lock1 := NewAdvisoryLock(pool1, lockId)
	lock2 := NewAdvisoryLock(pool2, lockId)

	db, err := pool2.Acquire(context.Background())
	assert.Nil(t, err)
	defer db.Release()

	wg := sync.WaitGroup{}
	wg.Add(1)

	// lock using pool1
	assert.Nil(t, lock1.Lock(context.Background()))
	time.AfterFunc(time.Second*1, func() {
		// wait 1s to unlock
		assert.Nil(t, lock1.Unlock(context.Background()))
		wg.Done()
	})

	// attempt to lock with pool2 - should not work
	locked, err := lock2.TryLock(context.Background())
	assert.Nil(t, err)
	assert.False(t, locked)

	// wait for lock1 unlock
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

func TestLockUnlockNTimes(t *testing.T) {
	pool := dbClient(t)

	var lockId int64 = 10
	lock := NewAdvisoryLock(pool, lockId)
	for i := 0; i < 10; i++ {
		// lock
		assert.Nil(t, lock.Lock(context.Background()))
	}
	assert.Equal(t, 10, lock.(*advisoryLock).counter)
	assert.NotNil(t, lock.(*advisoryLock).db)

	for i := 0; i < 10; i++ {
		// unlock
		assert.Nil(t, lock.Unlock(context.Background()))
	}

	assert.Zero(t, lock.(*advisoryLock).counter)
	assert.Nil(t, lock.(*advisoryLock).db)

	for i := 0; i < 10; i++ {
		// attempt re-lock again
		locked, err := lock.TryLock(context.Background())
		assert.Nil(t, err)
		assert.True(t, locked) // should succeed
	}

	assert.Equal(t, 10, lock.(*advisoryLock).counter)
	assert.NotNil(t, lock.(*advisoryLock).db)

	for i := 0; i < 10; i++ {
		// finally, unlock
		assert.Equal(t, 10-i, lock.(*advisoryLock).counter)
		assert.NotNil(t, lock.(*advisoryLock).db)

		assert.Nil(t, lock.Unlock(context.Background()))
	}

	assert.Zero(t, lock.(*advisoryLock).counter)
	assert.Nil(t, lock.(*advisoryLock).db)

}
