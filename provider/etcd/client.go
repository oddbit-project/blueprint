package etcd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/crypt/secure"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"strings"
)

type Client struct {
	client *clientv3.Client
	config *Config
	crypto secure.AES256GCM
}

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
		DialTimeout:          cfg.DialTimeout,
		DialKeepAliveTime:    cfg.DialKeepAliveTime,
		DialKeepAliveTimeout: cfg.DialKeepAliveTimeout,
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
		client: client,
		config: cfg,
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

func (c *Client) Put(ctx context.Context, key string, value []byte, opts ...clientv3.OpOption) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	if c.crypto != nil {
		encryptedValue, err := c.crypto.Encrypt(value)
		if err != nil {
			return fmt.Errorf("failed to encrypt value: %w", err)
		}
		value = encryptedValue
	}
	_, err := c.client.Put(ctx, key, string(value), opts...)
	return err
}

func (c *Client) Get(ctx context.Context, key string, opts ...clientv3.OpOption) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.New("key not found")
	}

	value := resp.Kvs[0].Value

	if c.crypto != nil {
		decryptedValue, err := c.crypto.Decrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt value: %w", err)
		}
		value = decryptedValue
	}

	return value, nil
}

func (c *Client) GetMultiple(ctx context.Context, key string, opts ...clientv3.OpOption) (map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		value := kv.Value

		if c.crypto != nil {
			decryptedValue, err := c.crypto.Decrypt(value)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt value for key %s: %w", string(kv.Key), err)
			}
			value = decryptedValue
		}

		result[string(kv.Key)] = value
	}

	return result, nil
}

func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
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

func (c *Client) ListWithValues(ctx context.Context, prefix string) (map[string][]byte, error) {
	return c.GetMultiple(ctx, prefix, clientv3.WithPrefix())
}

func (c *Client) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Delete(ctx, key, opts...)
	if err != nil {
		return 0, err
	}

	return resp.Deleted, nil
}

func (c *Client) DeletePrefix(ctx context.Context, prefix string) (int64, error) {
	return c.Delete(ctx, prefix, clientv3.WithPrefix())
}

func (c *Client) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.Watch(ctx, key, opts...)
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	return c.Watch(ctx, prefix, clientv3.WithPrefix())
}

func (c *Client) Transaction(ctx context.Context) clientv3.Txn {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.Txn(ctx)
}

func (c *Client) Lease(ttl int64) (clientv3.LeaseID, error) {
	ctx := context.Background()
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Grant(ctx, ttl)
	if err != nil {
		return 0, err
	}

	return resp.ID, nil
}

func (c *Client) KeepAlive(ctx context.Context, leaseID clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	return c.client.KeepAlive(ctx, leaseID)
}

func (c *Client) RevokeLease(ctx context.Context, leaseID clientv3.LeaseID) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	_, err := c.client.Revoke(ctx, leaseID)
	return err
}

func (c *Client) PutWithLease(ctx context.Context, key string, value []byte, leaseID clientv3.LeaseID) error {
	return c.Put(ctx, key, value, clientv3.WithLease(leaseID))
}

func (c *Client) CompactRevision(ctx context.Context, revision int64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	_, err := c.client.Compact(ctx, revision)
	return err
}

func (c *Client) Status(ctx context.Context) (*clientv3.StatusResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	if len(c.config.Endpoints) == 0 {
		return nil, errors.New("no endpoints configured")
	}

	return c.client.Status(ctx, c.config.Endpoints[0])
}

func (c *Client) MemberList(ctx context.Context) (*clientv3.MemberListResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	return c.client.MemberList(ctx)
}

func (c *Client) Lock(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	session, err := concurrency.NewSession(c.client)
	if err != nil {
		return err
	}
	defer session.Close()

	mutex := concurrency.NewMutex(session, name)
	return mutex.Lock(ctx)
}

func (c *Client) Unlock(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	session, err := concurrency.NewSession(c.client)
	if err != nil {
		return err
	}
	defer session.Close()

	mutex := concurrency.NewMutex(session, name)
	return mutex.Unlock(ctx)
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) GetClient() *clientv3.Client {
	return c.client
}

func (c *Client) IsEncrypted() bool {
	return c.crypto != nil
}

func (c *Client) BulkPut(ctx context.Context, kvs map[string][]byte) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ops := make([]clientv3.Op, 0, len(kvs))
	for k, v := range kvs {
		value := v
		if c.crypto != nil {
			encryptedValue, err := c.crypto.Encrypt(value)
			if err != nil {
				return fmt.Errorf("failed to encrypt value for key %s: %w", k, err)
			}
			value = encryptedValue
		}
		ops = append(ops, clientv3.OpPut(k, string(value)))
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	_, err := c.client.Txn(ctx).Then(ops...).Commit()
	return err
}

func (c *Client) BulkDelete(ctx context.Context, keys []string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	ops := make([]clientv3.Op, 0, len(keys))
	for _, k := range keys {
		ops = append(ops, clientv3.OpDelete(k))
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
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

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, clientv3.WithCountOnly())
	if err != nil {
		return false, err
	}

	return resp.Count > 0, nil
}

func (c *Client) Count(ctx context.Context, prefix string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithCountOnly())
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

func (c *Client) GetWithRevision(ctx context.Context, key string, revision int64) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, key, clientv3.WithRev(revision))
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.New("key not found")
	}

	value := resp.Kvs[0].Value

	if c.crypto != nil {
		decryptedValue, err := c.crypto.Decrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt value: %w", err)
		}
		value = decryptedValue
	}

	return value, nil
}

func (c *Client) PutIfNotExists(ctx context.Context, key string, value []byte) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	if c.crypto != nil {
		encryptedValue, err := c.crypto.Encrypt(value)
		if err != nil {
			return false, fmt.Errorf("failed to encrypt value: %w", err)
		}
		value = encryptedValue
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

func (c *Client) CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	compareValue := oldValue
	putValue := newValue

	if c.crypto != nil {
		encryptedOldValue, err := c.crypto.Encrypt(oldValue)
		if err != nil {
			return false, fmt.Errorf("failed to encrypt old value: %w", err)
		}
		compareValue = encryptedOldValue

		encryptedNewValue, err := c.crypto.Encrypt(newValue)
		if err != nil {
			return false, fmt.Errorf("failed to encrypt new value: %w", err)
		}
		putValue = encryptedNewValue
	}

	resp, err := c.client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(key), "=", compareValue)).
		Then(clientv3.OpPut(key, string(putValue))).
		Commit()

	if err != nil {
		return false, err
	}

	return resp.Succeeded, nil
}

func (c *Client) GetRange(ctx context.Context, start, end string) (map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	resp, err := c.client.Get(ctx, start, clientv3.WithRange(end))
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		value := kv.Value

		if c.crypto != nil {
			decryptedValue, err := c.crypto.Decrypt(value)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt value for key %s: %w", string(kv.Key), err)
			}
			value = decryptedValue
		}

		result[string(kv.Key)] = value
	}

	return result, nil
}

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

func (c *Client) GetKeysWithPrefix(ctx context.Context, prefix string, limit int64) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
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
