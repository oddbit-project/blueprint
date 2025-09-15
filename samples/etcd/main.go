package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/oddbit-project/blueprint/provider/etcd"
)

func main() {
	fmt.Println("etcd Provider Sample Application")
	fmt.Println("===============================")

	if len(os.Args) > 1 && os.Args[1] == "--help" {
		printHelp()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\nReceived shutdown signal")
		cancel()
	}()

	if err := runSamples(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("All samples completed successfully!")
}

func runSamples(ctx context.Context) error {
	// Check for custom endpoints from environment
	endpoints := []string{"localhost:2379"}
	if envEndpoints := os.Getenv("ETCD_ENDPOINTS"); envEndpoints != "" {
		endpoints = []string{envEndpoints}
	}
	
	fmt.Printf("Using etcd endpoints: %v\n", endpoints)
	
	config := etcd.DefaultConfig().
		WithEndpoints(endpoints...).
		WithTimeout(5)

	client, err := etcd.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer client.Close()

	fmt.Printf("Connected to etcd at %v\n", config.Endpoints)

	if err := basicOperations(ctx, client); err != nil {
		return err
	}

	if err := prefixOperations(ctx, client); err != nil {
		return err
	}

	if err := watchDemo(ctx, client); err != nil {
		return err
	}

	if err := leaseDemo(ctx, client); err != nil {
		return err
	}

	if err := distributedLockDemo(ctx, client); err != nil {
		return err
	}

	if err := transactionDemo(ctx, client); err != nil {
		return err
	}

	if err := encryptionDemo(ctx); err != nil {
		return err
	}

	return nil
}

func basicOperations(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nBasic Operations Demo")
	fmt.Println("---------------------")

	key := "/sample/basic/message"
	value := []byte("Hello from Blueprint etcd provider!")

	if err := client.Put(ctx, key, value); err != nil {
		return fmt.Errorf("put failed: %w", err)
	}
	fmt.Printf("PUT: %s = %s\n", key, string(value))

	retrieved, err := client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("get failed: %w", err)
	}
	fmt.Printf("GET: %s = %s\n", key, string(retrieved))

	exists, err := client.Exists(ctx, key)
	if err != nil {
		return fmt.Errorf("exists check failed: %w", err)
	}
	fmt.Printf("EXISTS: %s = %t\n", key, exists)

	deleted, err := client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	fmt.Printf("DELETE: %s (deleted %d keys)\n", key, deleted)

	return nil
}

func prefixOperations(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nPrefix Operations Demo")
	fmt.Println("----------------------")

	kvs := map[string][]byte{
		"/sample/users/alice": []byte(`{"name": "Alice", "email": "alice@example.com"}`),
		"/sample/users/bob":   []byte(`{"name": "Bob", "email": "bob@example.com"}`),
		"/sample/users/charlie": []byte(`{"name": "Charlie", "email": "charlie@example.com"}`),
		"/sample/config/timeout": []byte("30s"),
		"/sample/config/retries": []byte("3"),
	}

	if err := client.BulkPut(ctx, kvs); err != nil {
		return fmt.Errorf("bulk put failed: %w", err)
	}
	fmt.Printf("BULK_PUT: inserted %d key-value pairs\n", len(kvs))

	userKeys, err := client.List(ctx, "/sample/users/")
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}
	fmt.Printf("LIST KEYS: found %d user keys\n", len(userKeys))
	for _, key := range userKeys {
		fmt.Printf("   - %s\n", key)
	}

	users, err := client.ListWithValues(ctx, "/sample/users/")
	if err != nil {
		return fmt.Errorf("list with values failed: %w", err)
	}
	fmt.Printf("LIST WITH VALUES: found %d users\n", len(users))
	for key, value := range users {
		fmt.Printf("   - %s: %s\n", key, string(value))
	}

	count, err := client.Count(ctx, "/sample/")
	if err != nil {
		return fmt.Errorf("count failed: %w", err)
	}
	fmt.Printf("COUNT: total %d keys under /sample/\n", count)

	deleted, err := client.DeletePrefix(ctx, "/sample/")
	if err != nil {
		return fmt.Errorf("delete prefix failed: %w", err)
	}
	fmt.Printf("DELETE PREFIX: removed %d keys\n", deleted)

	return nil
}

func watchDemo(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nWatch Demo")
	fmt.Println("-----------")

	watchKey := "/sample/watch/counter"
	
	watchCtx, watchCancel := context.WithTimeout(ctx, 10*time.Second)
	defer watchCancel()

	watchChan := client.Watch(watchCtx, watchKey)
	fmt.Printf("Started watching key: %s\n", watchKey)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		eventCount := 0
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				eventCount++
				fmt.Printf("Event %d: %s %s = %s\n", 
					eventCount, event.Type, string(event.Kv.Key), string(event.Kv.Value))
				
				if eventCount >= 3 {
					watchCancel()
					return
				}
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	for i := 1; i <= 3; i++ {
		value := []byte(fmt.Sprintf("value-%d", i))
		if err := client.Put(ctx, watchKey, value); err != nil {
			return fmt.Errorf("put during watch failed: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	wg.Wait()

	if _, err := client.Delete(ctx, watchKey); err != nil {
		return fmt.Errorf("cleanup delete failed: %w", err)
	}

	fmt.Println("Watch demo completed")
	return nil
}

func leaseDemo(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nLease Demo")
	fmt.Println("-----------")

	leaseID, err := client.Lease(5) // 5 seconds TTL
	if err != nil {
		return fmt.Errorf("create lease failed: %w", err)
	}
	fmt.Printf("Created lease with ID: %d (TTL: 5s)\n", leaseID)

	key := "/sample/lease/temp-data"
	value := []byte("This data will expire in 5 seconds")

	if err := client.PutWithLease(ctx, key, value, leaseID); err != nil {
		return fmt.Errorf("put with lease failed: %w", err)
	}
	fmt.Printf("Stored key with lease: %s\n", key)

	exists, err := client.Exists(ctx, key)
	if err != nil {
		return fmt.Errorf("exists check failed: %w", err)
	}
	fmt.Printf("Key exists: %t\n", exists)

	fmt.Println("Waiting 6 seconds for lease to expire...")
	time.Sleep(6 * time.Second)

	exists, err = client.Exists(ctx, key)
	if err != nil {
		return fmt.Errorf("exists check after expiry failed: %w", err)
	}
	fmt.Printf("Key exists after expiry: %t\n", exists)

	fmt.Println("Lease demo completed")
	return nil
}

func distributedLockDemo(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nDistributed Lock Demo")
	fmt.Println("---------------------")

	lockName := "/sample/locks/demo-lock"
	
	lock1, err := client.NewLock(lockName)
	if err != nil {
		return fmt.Errorf("create lock failed: %w", err)
	}
	defer lock1.Close()

	fmt.Printf("Created lock: %s\n", lockName)

	// First, try to acquire the lock in blocking mode to ensure it works
	fmt.Println("Testing blocking lock acquisition...")
	
	lockCtx, lockCancel := context.WithTimeout(ctx, 10*time.Second)
	defer lockCancel()

	if err := lock1.Lock(lockCtx); err != nil {
		return fmt.Errorf("lock acquisition failed: %w", err)
	}
	fmt.Println("Lock acquired successfully")
	fmt.Println("Performing critical section work...")
	time.Sleep(2 * time.Second)

	if err := lock1.Unlock(ctx); err != nil {
		return fmt.Errorf("unlock failed: %w", err)
	}
	fmt.Println("Lock released")

	// Now test TryLock with appropriate timeout for containerized environments
	fmt.Println("Testing TryLock (non-blocking) with 100ms timeout...")
	acquired, err := lock1.TryLock(ctx, etcd.WithTTL(100*time.Millisecond))
	if err != nil {
		return fmt.Errorf("try lock failed: %w", err)
	}
	fmt.Printf("TryLock result: %t\n", acquired)
	
	if acquired {
		fmt.Println("TryLock succeeded")
		time.Sleep(500 * time.Millisecond) // Simulate work
		if err := lock1.Unlock(ctx); err != nil {
			return fmt.Errorf("unlock after trylock failed: %w", err)
		}
		fmt.Println("TryLock released")
	} else {
		fmt.Println("TryLock failed (lock may be held by another process)")
	}

	// Test TryLock with very short timeout to demonstrate failure
	fmt.Println("Testing TryLock with 1ms timeout (should fail)...")
	acquired2, err := lock1.TryLock(ctx, etcd.WithTTL(1*time.Millisecond))
	if err != nil {
		return fmt.Errorf("try lock with short timeout failed: %w", err)
	}
	fmt.Printf("TryLock with 1ms timeout result: %t\n", acquired2)
	
	if acquired2 {
		if err := lock1.Unlock(ctx); err != nil {
			return fmt.Errorf("unlock after short trylock failed: %w", err)
		}
		fmt.Println("Short TryLock released")
	}

	// Test concurrent lock access
	fmt.Println("Testing concurrent lock access...")
	lock2, err := client.NewLock(lockName)
	if err != nil {
		return fmt.Errorf("create second lock failed: %w", err)
	}
	defer lock2.Close()

	// First, acquire lock1 to demonstrate blocking
	fmt.Println("Acquiring lock1 for 3 seconds...")
	if err := lock1.Lock(ctx); err != nil {
		return fmt.Errorf("lock1 acquisition failed: %w", err)
	}

	// Try to acquire lock2 with TryLock while lock1 is held
	fmt.Println("Testing TryLock on lock2 while lock1 is held...")
	acquired3, err := lock2.TryLock(ctx, etcd.WithTTL(100*time.Millisecond))
	if err != nil {
		return fmt.Errorf("concurrent try lock failed: %w", err)
	}
	fmt.Printf("TryLock while other lock held: %t (should be false)\n", acquired3)

	// Release lock1 and then try lock2
	if err := lock1.Unlock(ctx); err != nil {
		return fmt.Errorf("lock1 unlock failed: %w", err)
	}
	fmt.Println("Lock1 released")

	// Now lock2 should be able to acquire the lock
	fmt.Println("Testing lock2 acquisition after lock1 release...")
	lockCtx2, lockCancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer lockCancel2()

	if err := lock2.Lock(lockCtx2); err != nil {
		return fmt.Errorf("lock2 acquisition failed: %w", err)
	}
	fmt.Println("Lock2 acquired successfully")

	if err := lock2.Unlock(ctx); err != nil {
		return fmt.Errorf("lock2 unlock failed: %w", err)
	}
	fmt.Println("Lock2 released")

	fmt.Println("Distributed lock demo completed")
	return nil
}

func transactionDemo(ctx context.Context, client *etcd.Client) error {
	fmt.Println("\nTransaction Demo")
	fmt.Println("----------------")

	key := "/sample/txn/counter"
	initialValue := []byte("10")

	if err := client.Put(ctx, key, initialValue); err != nil {
		return fmt.Errorf("initial put failed: %w", err)
	}
	fmt.Printf("Initial counter value: %s\n", string(initialValue))

	oldValue := []byte("10")
	newValue := []byte("20")

	success, err := client.CompareAndSwap(ctx, key, oldValue, newValue)
	if err != nil {
		return fmt.Errorf("compare and swap failed: %w", err)
	}
	fmt.Printf("CompareAndSwap (10->20): %t\n", success)

	retrieved, err := client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("get after CAS failed: %w", err)
	}
	fmt.Printf("Counter value after CAS: %s\n", string(retrieved))

	wrongOldValue := []byte("15")
	nextValue := []byte("30")

	success, err = client.CompareAndSwap(ctx, key, wrongOldValue, nextValue)
	if err != nil {
		return fmt.Errorf("second compare and swap failed: %w", err)
	}
	fmt.Printf("CompareAndSwap (15->30): %t (should be false)\n", success)

	retrieved, err = client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("final get failed: %w", err)
	}
	fmt.Printf("Final counter value: %s\n", string(retrieved))

	if _, err := client.Delete(ctx, key); err != nil {
		return fmt.Errorf("cleanup delete failed: %w", err)
	}

	fmt.Println("Transaction demo completed")
	return nil
}

func encryptionDemo(ctx context.Context) error {
	fmt.Println("\nEncryption Demo")
	fmt.Println("---------------")

	// Use same endpoint resolution as main client
	endpoints := []string{"localhost:2379"}
	if envEndpoints := os.Getenv("ETCD_ENDPOINTS"); envEndpoints != "" {
		endpoints = []string{envEndpoints}
	}
	
	fmt.Printf("Using etcd endpoints for encryption demo: %v\n", endpoints)

	encryptionKey := []byte("this-is-a-32-byte-key-for-demo!")

	config := etcd.DefaultConfig().
		WithEndpoints(endpoints...).
		WithEncryption(encryptionKey).
		WithTimeout(5)

	client, err := etcd.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create encrypted client: %w", err)
	}
	defer client.Close()

	fmt.Printf("Created encrypted client (encryption: %t)\n", client.IsEncrypted())

	key := "/sample/encrypted/secret"
	sensitiveData := []byte(`{"password": "super-secret-password", "api_key": "sk-1234567890"}`)

	if err := client.Put(ctx, key, sensitiveData); err != nil {
		return fmt.Errorf("encrypted put failed: %w", err)
	}
	fmt.Printf("Stored encrypted data: %s\n", key)

	retrieved, err := client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("encrypted get failed: %w", err)
	}
	fmt.Printf("Retrieved decrypted data: %s\n", string(retrieved))

	regularClient, err := etcd.NewClient(etcd.DefaultConfig().WithEndpoints(endpoints...))
	if err != nil {
		return fmt.Errorf("failed to create regular client: %w", err)
	}
	defer regularClient.Close()

	rawEncrypted, err := regularClient.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("raw get failed: %w", err)
	}
	fmt.Printf("Raw encrypted data (first 50 chars): %s...\n", string(rawEncrypted[:min(50, len(rawEncrypted))]))

	if _, err := client.Delete(ctx, key); err != nil {
		return fmt.Errorf("encrypted delete failed: %w", err)
	}

	fmt.Println("Encryption demo completed")
	return nil
}

func printHelp() {
	fmt.Println("etcd Provider Sample Application")
	fmt.Println("===============================")
	fmt.Println()
	fmt.Println("This sample demonstrates various features of the Blueprint etcd provider:")
	fmt.Println()
	fmt.Println("Basic Operations:")
	fmt.Println("   - Put/Get/Delete operations")
	fmt.Println("   - Exists checks")
	fmt.Println()
	fmt.Println("Prefix Operations:")
	fmt.Println("   - Bulk operations")
	fmt.Println("   - List keys with prefixes")
	fmt.Println("   - Count operations")
	fmt.Println()
	fmt.Println("Watch Demo:")
	fmt.Println("   - Real-time key monitoring")
	fmt.Println("   - Event handling")
	fmt.Println()
	fmt.Println("Lease Demo:")
	fmt.Println("   - TTL-based key expiration")
	fmt.Println("   - Automatic cleanup")
	fmt.Println()
	fmt.Println("Distributed Lock Demo:")
	fmt.Println("   - Mutual exclusion")
	fmt.Println("   - TryLock and blocking Lock")
	fmt.Println()
	fmt.Println("Transaction Demo:")
	fmt.Println("   - Atomic operations")
	fmt.Println("   - Compare-and-swap")
	fmt.Println()
	fmt.Println("Encryption Demo:")
	fmt.Println("   - Client-side encryption")
	fmt.Println("   - Transparent encrypt/decrypt")
	fmt.Println()
	fmt.Println("Prerequisites:")
	fmt.Println("   - etcd server running on localhost:2379")
	fmt.Println("   - Use 'etcd' command or Docker to start etcd")
	fmt.Println()
	fmt.Println("Usage: go run main.go")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}