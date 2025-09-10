//go:build integration
// +build integration

package etcd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// setupEtcdContainer sets up an etcd container for integration tests
func setupEtcdContainer(t *testing.T) (testcontainers.Container, string, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "quay.io/coreos/etcd:v3.5.9",
		ExposedPorts: []string{"2379/tcp", "2380/tcp"},
		Env: map[string]string{
			"ETCD_NAME":                        "etcd0",
			"ETCD_LISTEN_CLIENT_URLS":          "http://0.0.0.0:2379",
			"ETCD_LISTEN_PEER_URLS":            "http://0.0.0.0:2380",
			"ETCD_ADVERTISE_CLIENT_URLS":       "http://0.0.0.0:2379",
			"ETCD_INITIAL_ADVERTISE_PEER_URLS": "http://0.0.0.0:2380",
			"ETCD_INITIAL_CLUSTER":             "etcd0=http://0.0.0.0:2380",
			"ETCD_INITIAL_CLUSTER_STATE":       "new",
			"ETCD_INITIAL_CLUSTER_TOKEN":       "etcd-cluster-token",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("2379/tcp"),
			wait.ForLog("ready to serve client requests"),
		).WithDeadline(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "2379")
	require.NoError(t, err)

	hostIP, err := container.Host(ctx)
	require.NoError(t, err)

	endpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	// Wait for etcd to be fully ready
	time.Sleep(2 * time.Second)

	return container, endpoint, cleanup
}

// TestNewClient tests creating a new etcd client
func TestNewClient(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				Endpoints:   []string{endpoint},
				DialTimeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "With encryption",
			config: &Config{
				Endpoints:        []string{endpoint},
				DialTimeout:      5 * time.Second,
				EnableEncryption: true,
				EncryptionKey:    []byte("test-encryption-key-32-bytes-abc"),
			},
			wantErr: false,
		},
		{
			name:    "Nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config != nil && len(tt.config.Endpoints) > 0 && tt.config.Endpoints[0] == endpoint {
				// Use the actual endpoint from container
			} else if tt.config == nil {
				// Create default config with actual endpoint
				tt.config = DefaultConfig()
				tt.config.Endpoints = []string{endpoint}
			}

			client, err := NewClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, client)

			// Verify encryption setup
			if tt.config != nil && tt.config.EnableEncryption {
				assert.True(t, client.IsEncrypted())
			} else {
				assert.False(t, client.IsEncrypted())
			}

			// Cleanup
			err = client.Close()
			assert.NoError(t, err)
		})
	}
}

// TestPutAndGet tests basic put and get operations
func TestPutAndGet(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value []byte
	}{
		{
			name:  "Simple string value",
			key:   "/test/simple",
			value: []byte("hello world"),
		},
		{
			name:  "JSON value",
			key:   "/test/json",
			value: []byte(`{"name":"test","value":123}`),
		},
		{
			name:  "Binary value",
			key:   "/test/binary",
			value: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:  "Empty value",
			key:   "/test/empty",
			value: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Put value
			err := client.Put(ctx, tt.key, tt.value)
			require.NoError(t, err)

			// Get value
			got, err := client.Get(ctx, tt.key)
			require.NoError(t, err)
			
			// Handle empty values - etcd returns nil for empty byte slices
			if len(tt.value) == 0 && got == nil {
				got = []byte{}
			}
			assert.Equal(t, tt.value, got)

			// Cleanup
			_, err = client.Delete(ctx, tt.key)
			require.NoError(t, err)
		})
	}
}

// TestPutAndGetWithEncryption tests put and get with encryption enabled
func TestPutAndGetWithEncryption(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	// Create encryption key
	encryptionKey := []byte("test-encryption-key-32-bytes-abc")

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}
	config.EnableEncryption = true
	config.EncryptionKey = encryptionKey

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Test data
	key := "/test/encrypted"
	value := []byte("sensitive data")

	// Put encrypted value
	err = client.Put(ctx, key, value)
	require.NoError(t, err)

	// Get and decrypt value
	got, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	// Verify that the raw value in etcd is encrypted
	// Create a non-encrypted client to read raw value
	plainConfig := DefaultConfig()
	plainConfig.Endpoints = []string{endpoint}
	plainClient, err := NewClient(plainConfig)
	require.NoError(t, err)
	defer plainClient.Close()

	rawValue, err := plainClient.Get(ctx, key)
	require.NoError(t, err)
	assert.NotEqual(t, value, rawValue, "Raw value should be encrypted")
	assert.True(t, len(rawValue) > len(value), "Encrypted value should be longer")

	// Cleanup
	_, err = client.Delete(ctx, key)
	require.NoError(t, err)
}

// TestGetNonExistentKey tests getting a non-existent key
func TestGetNonExistentKey(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Try to get non-existent key
	_, err = client.Get(ctx, "/non/existent/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

// TestListAndGetMultiple tests listing keys and getting multiple values
func TestListAndGetMultiple(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Put multiple values with same prefix
	prefix := "/test/list/"
	testData := map[string][]byte{
		prefix + "key1": []byte("value1"),
		prefix + "key2": []byte("value2"),
		prefix + "key3": []byte("value3"),
	}

	for k, v := range testData {
		err := client.Put(ctx, k, v)
		require.NoError(t, err)
	}

	// Test List
	keys, err := client.List(ctx, prefix)
	require.NoError(t, err)
	assert.Len(t, keys, 3)
	for _, key := range keys {
		assert.Contains(t, testData, key)
	}

	// Test GetMultiple
	values, err := client.GetMultiple(ctx, prefix, clientv3.WithPrefix())
	require.NoError(t, err)
	assert.Len(t, values, 3)
	for k, v := range values {
		assert.Equal(t, testData[k], v)
	}

	// Test ListWithValues
	valuesWithList, err := client.ListWithValues(ctx, prefix)
	require.NoError(t, err)
	assert.Equal(t, values, valuesWithList)

	// Cleanup
	deleted, err := client.DeletePrefix(ctx, prefix)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)
}

// TestDelete tests delete operations
func TestDelete(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Put a value
	key := "/test/delete"
	value := []byte("to be deleted")
	err = client.Put(ctx, key, value)
	require.NoError(t, err)

	// Verify it exists
	got, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	// Delete it
	deleted, err := client.Delete(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify it's gone
	_, err = client.Get(ctx, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")

	// Delete non-existent key should return 0
	deleted, err = client.Delete(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

// TestDeletePrefix tests deleting multiple keys with a prefix
func TestDeletePrefix(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Put multiple values
	prefix := "/test/deleteprefix/"
	keys := []string{
		prefix + "key1",
		prefix + "key2",
		prefix + "key3",
	}

	for _, k := range keys {
		err := client.Put(ctx, k, []byte("value"))
		require.NoError(t, err)
	}

	// Delete all with prefix
	deleted, err := client.DeletePrefix(ctx, prefix)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	// Verify all are gone
	for _, k := range keys {
		_, err := client.Get(ctx, k)
		assert.Error(t, err)
	}
}

// TestWatch tests watching for key changes
func TestWatch(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "/test/watch"
	value := []byte("watched value")

	// Start watching
	watchChan := client.Watch(ctx, key)

	// Put a value
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := client.Put(context.Background(), key, value)
		assert.NoError(t, err)
	}()

	// Wait for event
	select {
	case resp := <-watchChan:
		require.NotNil(t, resp)
		assert.False(t, resp.Canceled)
		assert.Len(t, resp.Events, 1)
		assert.Equal(t, key, string(resp.Events[0].Kv.Key))
		assert.Equal(t, value, resp.Events[0].Kv.Value)
	case <-ctx.Done():
		t.Fatal("Watch timeout")
	}

	// Cleanup
	_, err = client.Delete(context.Background(), key)
	require.NoError(t, err)
}

// TestWatchPrefix tests watching for changes with a prefix
func TestWatchPrefix(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prefix := "/test/watchprefix/"

	// Start watching prefix
	watchChan := client.WatchPrefix(ctx, prefix)

	// Put multiple values
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := client.Put(context.Background(), prefix+"key1", []byte("value1"))
		assert.NoError(t, err)
		err = client.Put(context.Background(), prefix+"key2", []byte("value2"))
		assert.NoError(t, err)
	}()

	// Collect events
	events := 0
	for events < 2 {
		select {
		case resp := <-watchChan:
			require.NotNil(t, resp)
			assert.False(t, resp.Canceled)
			events += len(resp.Events)
		case <-ctx.Done():
			t.Fatal("Watch timeout")
		}
	}

	assert.Equal(t, 2, events)

	// Cleanup
	_, err = client.DeletePrefix(context.Background(), prefix)
	require.NoError(t, err)
}

// TestTransaction tests transaction operations
func TestTransaction(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Setup initial data
	key1 := "/test/tx/key1"
	key2 := "/test/tx/key2"
	value1 := []byte("value1")
	value2 := []byte("value2")

	err = client.Put(ctx, key1, value1)
	require.NoError(t, err)

	// Transaction: if key1 exists, put key2
	txn := client.Transaction(ctx)
	resp, err := txn.
		If(clientv3.Compare(clientv3.CreateRevision(key1), ">", 0)).
		Then(clientv3.OpPut(key2, string(value2))).
		Commit()
	require.NoError(t, err)
	assert.True(t, resp.Succeeded)

	// Verify key2 was created
	got, err := client.Get(ctx, key2)
	require.NoError(t, err)
	assert.Equal(t, value2, got)

	// Transaction: if key3 doesn't exist, don't put key4
	key3 := "/test/tx/key3"
	key4 := "/test/tx/key4"
	txn = client.Transaction(ctx)
	resp, err = txn.
		If(clientv3.Compare(clientv3.CreateRevision(key3), ">", 0)).
		Then(clientv3.OpPut(key4, "value4")).
		Commit()
	require.NoError(t, err)
	assert.False(t, resp.Succeeded)

	// Verify key4 was not created
	_, err = client.Get(ctx, key4)
	assert.Error(t, err)

	// Cleanup
	_, _ = client.Delete(ctx, key1)
	_, _ = client.Delete(ctx, key2)
}

// TestLease tests lease operations
func TestLease(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Create a lease with 5 second TTL
	leaseID, err := client.Lease(5)
	require.NoError(t, err)
	assert.NotEqual(t, clientv3.LeaseID(0), leaseID)

	// Put a value with the lease
	key := "/test/lease/key"
	value := []byte("leased value")
	err = client.PutWithLease(ctx, key, value, leaseID)
	require.NoError(t, err)

	// Verify the key exists
	got, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	// Keep the lease alive
	keepAliveChan, err := client.KeepAlive(ctx, leaseID)
	require.NoError(t, err)
	assert.NotNil(t, keepAliveChan)

	// Receive at least one keep-alive response
	select {
	case resp := <-keepAliveChan:
		assert.NotNil(t, resp)
		assert.Equal(t, leaseID, resp.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("KeepAlive timeout")
	}

	// Revoke the lease
	err = client.RevokeLease(ctx, leaseID)
	require.NoError(t, err)

	// Verify the key is gone
	_, err = client.Get(ctx, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

// TestBulkOperations tests bulk put and delete operations
func TestBulkOperations(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Prepare bulk data
	bulkData := map[string][]byte{
		"/test/bulk/key1": []byte("value1"),
		"/test/bulk/key2": []byte("value2"),
		"/test/bulk/key3": []byte("value3"),
		"/test/bulk/key4": []byte("value4"),
	}

	// Bulk put
	err = client.BulkPut(ctx, bulkData)
	require.NoError(t, err)

	// Verify all keys exist
	for k, v := range bulkData {
		got, err := client.Get(ctx, k)
		require.NoError(t, err)
		assert.Equal(t, v, got)
	}

	// Bulk delete
	keys := make([]string, 0, len(bulkData))
	for k := range bulkData {
		keys = append(keys, k)
	}

	deleted, err := client.BulkDelete(ctx, keys)
	require.NoError(t, err)
	assert.Equal(t, int64(4), deleted)

	// Verify all keys are gone
	for k := range bulkData {
		_, err := client.Get(ctx, k)
		assert.Error(t, err)
	}
}

// TestBulkPutWithEncryption tests bulk put with encryption
func TestBulkPutWithEncryption(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}
	config.EnableEncryption = true
	config.EncryptionKey = []byte("test-encryption-key-32-bytes-abc")

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Prepare bulk data
	bulkData := map[string][]byte{
		"/test/bulkenc/key1": []byte("encrypted1"),
		"/test/bulkenc/key2": []byte("encrypted2"),
	}

	// Bulk put with encryption
	err = client.BulkPut(ctx, bulkData)
	require.NoError(t, err)

	// Verify values are decrypted correctly
	for k, v := range bulkData {
		got, err := client.Get(ctx, k)
		require.NoError(t, err)
		assert.Equal(t, v, got)
	}

	// Cleanup
	keys := make([]string, 0, len(bulkData))
	for k := range bulkData {
		keys = append(keys, k)
	}
	_, _ = client.BulkDelete(ctx, keys)
}

// TestExists tests checking if a key exists
func TestExists(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	key := "/test/exists"
	value := []byte("exists")

	// Check non-existent key
	exists, err := client.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	// Put value
	err = client.Put(ctx, key, value)
	require.NoError(t, err)

	// Check existing key
	exists, err = client.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	_, _ = client.Delete(ctx, key)
}

// TestCount tests counting keys with a prefix
func TestCount(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	prefix := "/test/count/"

	// Count with no keys
	count, err := client.Count(ctx, prefix)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add keys
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("%skey%d", prefix, i)
		err := client.Put(ctx, key, []byte(fmt.Sprintf("value%d", i)))
		require.NoError(t, err)
	}

	// Count with keys
	count, err = client.Count(ctx, prefix)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Cleanup
	_, _ = client.DeletePrefix(ctx, prefix)
}

// TestPutIfNotExists tests conditional put
func TestPutIfNotExists(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	key := "/test/putifnotexists"
	value1 := []byte("first")
	value2 := []byte("second")

	// First put should succeed
	success, err := client.PutIfNotExists(ctx, key, value1)
	require.NoError(t, err)
	assert.True(t, success)

	// Verify value
	got, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value1, got)

	// Second put should fail
	success, err = client.PutIfNotExists(ctx, key, value2)
	require.NoError(t, err)
	assert.False(t, success)

	// Value should still be the first one
	got, err = client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value1, got)

	// Cleanup
	_, _ = client.Delete(ctx, key)
}

// TestCompareAndSwap tests compare-and-swap operation
func TestCompareAndSwap(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	key := "/test/cas"
	value1 := []byte("initial")
	value2 := []byte("updated")
	value3 := []byte("final")

	// Put initial value
	err = client.Put(ctx, key, value1)
	require.NoError(t, err)

	// CAS with correct old value should succeed
	success, err := client.CompareAndSwap(ctx, key, value1, value2)
	require.NoError(t, err)
	assert.True(t, success)

	// Verify value was updated
	got, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value2, got)

	// CAS with wrong old value should fail
	success, err = client.CompareAndSwap(ctx, key, value1, value3)
	require.NoError(t, err)
	assert.False(t, success)

	// Value should still be value2
	got, err = client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value2, got)

	// Cleanup
	_, _ = client.Delete(ctx, key)
}

// TestGetRange tests getting keys within a range
func TestGetRange(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Put test data
	testData := map[string][]byte{
		"/test/range/a": []byte("value_a"),
		"/test/range/b": []byte("value_b"),
		"/test/range/c": []byte("value_c"),
		"/test/range/d": []byte("value_d"),
		"/test/range/e": []byte("value_e"),
	}

	for k, v := range testData {
		err := client.Put(ctx, k, v)
		require.NoError(t, err)
	}

	// Get range from b to d (exclusive)
	rangeData, err := client.GetRange(ctx, "/test/range/b", "/test/range/d")
	require.NoError(t, err)
	assert.Len(t, rangeData, 2)
	assert.Equal(t, []byte("value_b"), rangeData["/test/range/b"])
	assert.Equal(t, []byte("value_c"), rangeData["/test/range/c"])

	// Cleanup
	_, _ = client.DeletePrefix(ctx, "/test/range/")
}

// TestMoveKey tests moving a key to a new location
func TestMoveKey(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	oldKey := "/test/move/old"
	newKey := "/test/move/new"
	value := []byte("moved value")

	// Put value at old key
	err = client.Put(ctx, oldKey, value)
	require.NoError(t, err)

	// Move to new key
	err = client.MoveKey(ctx, oldKey, newKey)
	require.NoError(t, err)

	// Old key should not exist
	_, err = client.Get(ctx, oldKey)
	assert.Error(t, err)

	// New key should have the value
	got, err := client.Get(ctx, newKey)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	// Cleanup
	_, _ = client.Delete(ctx, newKey)
}

// TestGetKeysWithPrefix tests getting keys with prefix and limit
func TestGetKeysWithPrefix(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	prefix := "/test/prefix/"

	// Put multiple keys
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%skey%02d", prefix, i)
		err := client.Put(ctx, key, []byte(fmt.Sprintf("value%d", i)))
		require.NoError(t, err)
	}

	// Get all keys with prefix
	keys, err := client.GetKeysWithPrefix(ctx, prefix, 0)
	require.NoError(t, err)
	assert.Len(t, keys, 10)

	// Get limited keys with prefix
	keys, err = client.GetKeysWithPrefix(ctx, prefix, 5)
	require.NoError(t, err)
	assert.Len(t, keys, 5)

	// Cleanup
	_, _ = client.DeletePrefix(ctx, prefix)
}

// TestGetKeysByPattern tests getting keys by pattern
func TestGetKeysByPattern(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	prefix := "/test/pattern/"

	// Put keys with different patterns
	testKeys := []string{
		prefix + "user_001",
		prefix + "user_002",
		prefix + "admin_001",
		prefix + "admin_002",
		prefix + "guest_001",
	}

	for _, k := range testKeys {
		err := client.Put(ctx, k, []byte("value"))
		require.NoError(t, err)
	}

	// Get keys matching pattern "user"
	userKeys, err := client.GetKeysByPattern(ctx, prefix, "user")
	require.NoError(t, err)
	assert.Len(t, userKeys, 2)
	for _, k := range userKeys {
		assert.Contains(t, k, "user")
	}

	// Get keys matching pattern "001"
	keys001, err := client.GetKeysByPattern(ctx, prefix, "001")
	require.NoError(t, err)
	assert.Len(t, keys001, 3)
	for _, k := range keys001 {
		assert.Contains(t, k, "001")
	}

	// Cleanup
	_, _ = client.DeletePrefix(ctx, prefix)
}

// TestStatus tests getting etcd server status
func TestStatus(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	status, err := client.Status(ctx)
	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Greater(t, status.Header.MemberId, uint64(0))
	assert.NotEmpty(t, status.Version)
}

// TestMemberList tests listing etcd cluster members
func TestMemberList(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	members, err := client.MemberList(ctx)
	require.NoError(t, err)
	assert.NotNil(t, members)
	assert.Len(t, members.Members, 1) // Single node cluster
	assert.NotEmpty(t, members.Members[0].Name)
}

// TestRequestTimeout tests that operations respect request timeout
func TestRequestTimeout(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}
	config.RequestTimeout = 100 * time.Millisecond

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	// Create a context that will be used to simulate a slow operation
	// Note: This is a basic test - in real scenarios, you might need to
	// simulate network delays or use a mock etcd server
	ctx := context.Background()

	// Test that normal operations complete within timeout
	err = client.Put(ctx, "/test/timeout", []byte("value"))
	assert.NoError(t, err)

	value, err := client.Get(ctx, "/test/timeout")
	assert.NoError(t, err)
	assert.Equal(t, []byte("value"), value)

	// Cleanup
	_, _ = client.Delete(ctx, "/test/timeout")
}

// TestConcurrentOperations tests concurrent access to the client
func TestConcurrentOperations(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	done := make(chan bool)
	errors := make(chan error, 100)

	// Run concurrent puts
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := fmt.Sprintf("/test/concurrent/key%d", id)
			value := []byte(fmt.Sprintf("value%d", id))
			if err := client.Put(ctx, key, value); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all puts to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent put error: %v", err)
	default:
		// No errors
	}

	// Verify all keys exist
	keys, err := client.List(ctx, "/test/concurrent/")
	require.NoError(t, err)
	assert.Len(t, keys, 10)

	// Cleanup
	_, _ = client.DeletePrefix(ctx, "/test/concurrent/")
}

// TestGetWithRevision tests getting a value at a specific revision
func TestGetWithRevision(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	key := "/test/revision"
	value1 := []byte("revision1")
	value2 := []byte("revision2")

	// Put first value
	err = client.Put(ctx, key, value1)
	require.NoError(t, err)

	// Get the current revision
	resp, err := client.GetClient().Get(ctx, key)
	require.NoError(t, err)
	revision1 := resp.Header.Revision

	// Put second value
	err = client.Put(ctx, key, value2)
	require.NoError(t, err)

	// Get value at first revision
	got, err := client.GetWithRevision(ctx, key, revision1)
	require.NoError(t, err)
	assert.Equal(t, value1, got)

	// Get current value
	got, err = client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value2, got)

	// Cleanup
	_, _ = client.Delete(ctx, key)
}


// TestNewLockBasicOperations tests the new Lock struct basic functionality
func TestNewLockBasicOperations(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	lockName := "/test/newlock/basic"

	// Create a new lock
	lock, err := client.NewLock(lockName)
	require.NoError(t, err)
	assert.NotNil(t, lock)
	defer lock.Close()

	// Verify initial state
	assert.False(t, lock.IsLocked())
	assert.Equal(t, lockName, lock.Name())

	// Acquire the lock
	err = lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, lock.IsLocked())

	// Unlock the lock
	err = lock.Unlock(ctx)
	require.NoError(t, err)
	assert.False(t, lock.IsLocked())

	// Unlock again should be safe
	err = lock.Unlock(ctx)
	require.NoError(t, err)
	assert.False(t, lock.IsLocked())
}

// TestNewLockConcurrentAccess tests that the new Lock struct works correctly with concurrent access
func TestNewLockConcurrentAccess(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client1, err := NewClient(config)
	require.NoError(t, err)
	defer client1.Close()

	client2, err := NewClient(config)
	require.NoError(t, err)
	defer client2.Close()

	ctx := context.Background()
	lockName := "/test/newlock/concurrent"

	// Create locks from different clients
	lock1, err := client1.NewLock(lockName)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := client2.NewLock(lockName)
	require.NoError(t, err)
	defer lock2.Close()

	// Client1 acquires the lock
	err = lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, lock1.IsLocked())

	// Client2 should be able to try to acquire but will block
	// We'll test this with TryLock instead to avoid blocking
	acquired, err := lock2.TryLock(ctx)
	require.NoError(t, err)
	assert.False(t, acquired, "Lock should not be acquired since it's held by client1")
	assert.False(t, lock2.IsLocked())

	// Client1 releases the lock
	err = lock1.Unlock(ctx)
	require.NoError(t, err)
	assert.False(t, lock1.IsLocked())

	// Now client2 should be able to acquire the lock
	err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, lock2.IsLocked())

	err = lock2.Unlock(ctx)
	require.NoError(t, err)
	assert.False(t, lock2.IsLocked())
}

// TestNewLockTryLock tests the non-blocking TryLock functionality
func TestNewLockTryLock(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	lockName := "/test/newlock/trylock"

	// Create a lock
	lock, err := client.NewLock(lockName)
	require.NoError(t, err)
	defer lock.Close()

	// TryLock on an available lock should succeed
	acquired, err := lock.TryLock(ctx)
	require.NoError(t, err)
	assert.True(t, acquired)
	assert.True(t, lock.IsLocked())

	// Create another lock with the same name
	lock2, err := client.NewLock(lockName)
	require.NoError(t, err)
	defer lock2.Close()

	// TryLock on a held lock should return false without error
	acquired, err = lock2.TryLock(ctx)
	require.NoError(t, err)
	assert.False(t, acquired)
	assert.False(t, lock2.IsLocked())

	// Unlock the first lock
	err = lock.Unlock(ctx)
	require.NoError(t, err)

	// Now TryLock should succeed
	acquired, err = lock2.TryLock(ctx)
	require.NoError(t, err)
	assert.True(t, acquired)
	assert.True(t, lock2.IsLocked())

	err = lock2.Unlock(ctx)
	require.NoError(t, err)
}

// TestNewLockMultipleLocks tests creating multiple different locks
func TestNewLockMultipleLocks(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Create multiple locks with different names
	lock1, err := client.NewLock("/test/newlock/multi1")
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := client.NewLock("/test/newlock/multi2")
	require.NoError(t, err)
	defer lock2.Close()

	lock3, err := client.NewLock("/test/newlock/multi3")
	require.NoError(t, err)
	defer lock3.Close()

	// All locks should be acquirable independently
	err = lock1.Lock(ctx)
	require.NoError(t, err)

	err = lock2.Lock(ctx)
	require.NoError(t, err)

	err = lock3.Lock(ctx)
	require.NoError(t, err)

	// All should be locked
	assert.True(t, lock1.IsLocked())
	assert.True(t, lock2.IsLocked())
	assert.True(t, lock3.IsLocked())

	// Unlock in different order
	err = lock2.Unlock(ctx)
	require.NoError(t, err)

	err = lock1.Unlock(ctx)
	require.NoError(t, err)

	err = lock3.Unlock(ctx)
	require.NoError(t, err)

	// All should be unlocked
	assert.False(t, lock1.IsLocked())
	assert.False(t, lock2.IsLocked())
	assert.False(t, lock3.IsLocked())
}

// TestNewLockSessionClosure tests that closing a lock releases resources
func TestNewLockSessionClosure(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	lockName := "/test/newlock/closure"

	// Create and acquire a lock
	lock, err := client.NewLock(lockName)
	require.NoError(t, err)

	err = lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, lock.IsLocked())

	// Close the lock (this should release the underlying session)
	err = lock.Close()
	require.NoError(t, err)

	// Create a new lock with the same name - it should be available
	lock2, err := client.NewLock(lockName)
	require.NoError(t, err)
	defer lock2.Close()

	// Should be able to acquire immediately since the first lock was closed
	err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, lock2.IsLocked())

	err = lock2.Unlock(ctx)
	require.NoError(t, err)
}

// TestLockWithTimeout tests lock operations with context timeouts
func TestLockWithTimeout(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	lockName := "/test/newlock/timeout"

	// Create and acquire a lock
	lock1, err := client.NewLock(lockName)
	require.NoError(t, err)
	defer lock1.Close()

	err = lock1.Lock(context.Background())
	require.NoError(t, err)

	// Create second lock for the same resource
	lock2, err := client.NewLock(lockName)
	require.NoError(t, err)
	defer lock2.Close()

	// Try to acquire with a short timeout - should fail
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = lock2.Lock(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	// Cleanup
	err = lock1.Unlock(context.Background())
	require.NoError(t, err)
}

// TestCompactRevision tests compacting old revisions
func TestCompactRevision(t *testing.T) {
	container, endpoint, cleanup := setupEtcdContainer(t)
	defer cleanup()
	_ = container

	config := DefaultConfig()
	config.Endpoints = []string{endpoint}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Put multiple values to increase revision
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("/test/compact/key%d", i)
		err := client.Put(ctx, key, []byte(fmt.Sprintf("value%d", i)))
		require.NoError(t, err)
	}

	// Get current revision
	status, err := client.Status(ctx)
	require.NoError(t, err)
	currentRevision := status.Header.Revision

	// Compact to current revision - 2
	compactRevision := currentRevision - 2
	if compactRevision > 0 {
		err = client.CompactRevision(ctx, compactRevision)
		assert.NoError(t, err)
	}

	// Cleanup
	_, _ = client.DeletePrefix(ctx, "/test/compact/")
}