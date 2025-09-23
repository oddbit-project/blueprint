package etcd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/crypt/secure"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strings"
	"time"
)

// Client is a wrapper around etcd's clientv3.Client that provides additional functionality
// including automatic encryption/decryption, request timeouts, and simplified APIs.
type Client struct {
	client         *clientv3.Client
	crypto         secure.EncryptionProvider
	requestTimeout time.Duration
	endpoints      []string
}

// NewClient creates a new etcd client with the given configuration.
// If cfg is nil, DefaultConfig() is used. The client handles connection management,
// authentication, TLS, and optionally client-side encryption of values.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	password, err := cfg.Fetch()
	if err != nil {
		return nil, err
	}

	etcdConfig := clientv3.Config{
		Endpoints:            cfg.Endpoints,
		DialTimeout:          time.Duration(cfg.DialTimeout) * time.Second,
		DialKeepAliveTime:    time.Duration(cfg.DialKeepAliveTime) * time.Second,
		DialKeepAliveTimeout: time.Duration(cfg.DialKeepAliveTimeout) * time.Second,
		Username:             cfg.Username,
		Password:             password,
		MaxCallSendMsgSize:   cfg.MaxCallSendMsgSize,
		MaxCallRecvMsgSize:   cfg.MaxCallRecvMsgSize,
		PermitWithoutStream:  cfg.PermitWithoutStream,
		RejectOldCluster:     cfg.RejectOldCluster,
	}

	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		return nil, err
	}
	etcdConfig.TLS = tlsConfig

	client, err := clientv3.New(etcdConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	c := &Client{
		client:         client,
		requestTimeout: time.Duration(cfg.RequestTimeout) * time.Second,
		endpoints:      cfg.Endpoints,
	}

	if cfg.EnableEncryption && len(cfg.EncryptionKey) > 0 {
		hasher := sha256.New()
		hasher.Write(cfg.EncryptionKey)
		hashedKey := hasher.Sum(nil)
		crypto, err := secure.NewAES256GCM(hashedKey)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to setup encryption: %w", err)
		}
		c.crypto = crypto
	}

	return c, nil
}

// PrepareValue prepares a value for writing
func (c *Client) PrepareValue(value []byte) ([]byte, error) {
	if c.crypto != nil {
		encryptedValue, err := c.crypto.Encrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt value: %w", err)
		}
		return encryptedValue, nil
	}
	return value, nil
}

// DecodeValue decodes a value for reading
func (c *Client) DecodeValue(value []byte) ([]byte, error) {
	if c.crypto != nil {
		decryptedValue, err := c.crypto.Decrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt value: %w", err)
		}
		return decryptedValue, nil
	}
	return value, nil
}

// Put stores a key-value pair in etcd.
// The value is automatically encrypted if encryption is enabled on the client.
// Additional etcd options can be passed through opts (e.g., clientv3.WithLease).
func (c *Client) Put(ctx context.Context, key string, value []byte, opts ...clientv3.OpOption) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	var err error
	value, err = c.PrepareValue(value)
	if err != nil {
		return err
	}
	_, err = c.client.Put(ctx, key, string(value), opts...)
	return err
}

// Get retrieves the value for a given key from etcd.
// If encryption is enabled, the value is automatically decrypted.
// Returns an error if the key is not found.
func (c *Client) Get(ctx context.Context, key string, opts ...clientv3.OpOption) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.New("key not found")
	}

	return c.DecodeValue(resp.Kvs[0].Value)
}

// GetMultiple retrieves multiple key-value pairs matching the given key pattern.
// Typically used with clientv3.WithPrefix() to get all keys with a common prefix.
// Values are automatically decrypted if encryption is enabled.
func (c *Client) GetMultiple(ctx context.Context, key string, opts ...clientv3.OpOption) (map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)], err = c.DecodeValue(kv.Value)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// List returns all keys that start with the given prefix.
// This is a keys-only operation that does not retrieve values.
func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}

	return keys, nil
}

// ListWithValues returns all key-value pairs that start with the given prefix.
// This is equivalent to GetMultiple with WithPrefix option.
// Values are automatically decrypted if encryption is enabled.
func (c *Client) ListWithValues(ctx context.Context, prefix string) (map[string][]byte, error) {
	return c.GetMultiple(ctx, prefix, clientv3.WithPrefix())
}

// Delete removes a key from etcd.
// Returns the number of keys deleted (0 if key didn't exist, 1 if deleted).
func (c *Client) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Delete(ctx, key, opts...)
	if err != nil {
		return 0, err
	}

	return resp.Deleted, nil
}

// DeletePrefix removes all keys that start with the given prefix.
// Returns the total number of keys deleted.
func (c *Client) DeletePrefix(ctx context.Context, prefix string) (int64, error) {
	return c.Delete(ctx, prefix, clientv3.WithPrefix())
}

// Watch creates a watcher for changes to a specific key.
// Returns a channel that receives watch events when the key is modified.
// The context controls the lifetime of the watcher.
func (c *Client) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.Watch(ctx, key, opts...)
}

// WatchPrefix creates a watcher for changes to all keys with the given prefix.
// Returns a channel that receives watch events for any key starting with the prefix.
func (c *Client) WatchPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	return c.Watch(ctx, prefix, clientv3.WithPrefix())
}

// Transaction creates a new etcd transaction for atomic operations.
// Use this to perform multiple operations atomically with if/then/else conditions.
func (c *Client) Transaction(ctx context.Context) clientv3.Txn {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.Txn(ctx)
}

// Lease creates a new lease with the specified time-to-live in seconds.
// Returns a lease ID that can be attached to keys for automatic expiration.
func (c *Client) Lease(ttl int64) (clientv3.LeaseID, error) {
	ctx := context.Background()
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Grant(ctx, ttl)
	if err != nil {
		return 0, err
	}

	return resp.ID, nil
}

// KeepAlive sends periodic keep-alive requests to prevent a lease from expiring.
// Returns a channel that receives keep-alive responses from etcd.
func (c *Client) KeepAlive(ctx context.Context, leaseID clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.KeepAlive(ctx, leaseID)
}

// RevokeLease immediately revokes a lease, causing all associated keys to be deleted.
func (c *Client) RevokeLease(ctx context.Context, leaseID clientv3.LeaseID) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	_, err := c.client.Revoke(ctx, leaseID)
	return err
}

// PutWithLease stores a key-value pair with an associated lease for automatic expiration.
// The key will be automatically deleted when the lease expires.
func (c *Client) PutWithLease(ctx context.Context, key string, value []byte, leaseID clientv3.LeaseID) error {
	return c.Put(ctx, key, value, clientv3.WithLease(leaseID))
}

// CompactRevision compacts etcd's revision history up to the given revision.
// This reclaims storage space by removing old key revisions.
func (c *Client) CompactRevision(ctx context.Context, revision int64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	_, err := c.client.Compact(ctx, revision)
	return err
}

// Status returns the status of the etcd server including version, database size, and leader info.
// Connects to the first configured endpoint to retrieve status.
func (c *Client) Status(ctx context.Context) (*clientv3.StatusResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	if len(c.endpoints) == 0 {
		return nil, errors.New("no endpoints configured")
	}

	return c.client.Status(ctx, c.endpoints[0])
}

// MemberList returns information about all members in the etcd cluster.
func (c *Client) MemberList(ctx context.Context) (*clientv3.MemberListResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	return c.client.MemberList(ctx)
}

// Close closes the etcd client connection and releases all associated resources.
func (c *Client) Close() error {
	return c.client.Close()
}

// GetClient returns the underlying etcd client for advanced operations.
// Use with caution as it bypasses encryption and other wrapper functionality.
func (c *Client) GetClient() *clientv3.Client {
	return c.client
}

// IsEncrypted returns true if the client has encryption enabled.
func (c *Client) IsEncrypted() bool {
	return c.crypto != nil
}

// BulkPut stores multiple key-value pairs in a single atomic transaction.
// All values are encrypted if encryption is enabled. If any operation fails, none are applied.
func (c *Client) BulkPut(ctx context.Context, kvs map[string][]byte) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ops := make([]clientv3.Op, 0, len(kvs))
	for k, v := range kvs {
		value, err := c.PrepareValue(v)
		if err != nil {
			return err
		}
		ops = append(ops, clientv3.OpPut(k, string(value)))
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	_, err := c.client.Txn(ctx).Then(ops...).Commit()
	return err
}

// BulkDelete removes multiple keys in a single atomic transaction.
// Returns the total number of keys deleted. If any operation fails, none are applied.
func (c *Client) BulkDelete(ctx context.Context, keys []string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	ops := make([]clientv3.Op, 0, len(keys))
	for _, k := range keys {
		ops = append(ops, clientv3.OpDelete(k))
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		return 0, err
	}

	var deleted int64
	for _, r := range resp.Responses {
		deleted += r.GetResponseDeleteRange().Deleted
	}

	return deleted, nil
}

// Exists checks whether a key exists in etcd without retrieving its value.
// This is more efficient than Get when you only need to check presence.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, clientv3.WithCountOnly())
	if err != nil {
		return false, err
	}

	return resp.Count > 0, nil
}

// Count returns the number of keys that start with the given prefix.
// This is more efficient than List when you only need the count.
func (c *Client) Count(ctx context.Context, prefix string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithCountOnly())
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// GetWithRevision retrieves the value of a key at a specific historical revision.
// Useful for accessing previous versions of a key's value.
func (c *Client) GetWithRevision(ctx context.Context, key string, revision int64) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, clientv3.WithRev(revision))
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.New("key not found")
	}

	return c.DecodeValue(resp.Kvs[0].Value)
}

// PutIfNotExists stores a key-value pair only if the key does not already exist.
// Returns true if the key was created, false if it already existed.
func (c *Client) PutIfNotExists(ctx context.Context, key string, value []byte) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	var err error
	value, err = c.PrepareValue(value)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, string(value))).
		Commit()

	if err != nil {
		return false, err
	}

	return resp.Succeeded, nil
}

// CompareAndSwap atomically updates a key's value only if it currently matches the expected old value.
// Returns true if the swap was successful, false if the current value didn't match the expected value.
func (c *Client) CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	oldValue, err := c.PrepareValue(oldValue)
	if err != nil {
		return false, err
	}
	newValue, err = c.PrepareValue(newValue)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(key), "=", string(oldValue))).
		Then(clientv3.OpPut(key, string(newValue))).
		Commit()

	if err != nil {
		return false, err
	}

	return resp.Succeeded, nil
}

// GetRange retrieves all key-value pairs within a specified key range.
// The range includes keys >= start and < end (half-open interval).
func (c *Client) GetRange(ctx context.Context, start, end string) (map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, start, clientv3.WithRange(end))
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)], err = c.DecodeValue(kv.Value)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// MoveKey atomically moves a key from one location to another.
// This copies the value to the new key and deletes the old key.
func (c *Client) MoveKey(ctx context.Context, oldKey, newKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	value, err := c.Get(ctx, oldKey)
	if err != nil {
		return fmt.Errorf("failed to get old key: %w", err)
	}

	if err := c.Put(ctx, newKey, value); err != nil {
		return fmt.Errorf("failed to put new key: %w", err)
	}

	if _, err := c.Delete(ctx, oldKey); err != nil {
		return fmt.Errorf("failed to delete old key: %w", err)
	}

	return nil
}

// GetKeysWithPrefix returns keys that start with the given prefix, optionally limited.
// If limit is 0, all matching keys are returned.
func (c *Client) GetKeysWithPrefix(ctx context.Context, prefix string, limit int64) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithKeysOnly()}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}

	resp, err := c.client.Get(ctx, prefix, opts...)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}

	return keys, nil
}

// GetKeysByPattern returns keys that start with the given prefix and contain the specified pattern.
// Uses simple string matching to filter keys based on the pattern.
func (c *Client) GetKeysByPattern(ctx context.Context, prefix, pattern string) ([]string, error) {
	keys, err := c.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var matched []string
	for _, key := range keys {
		if strings.Contains(key, pattern) {
			matched = append(matched, key)
		}
	}

	return matched, nil
}
