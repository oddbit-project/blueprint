package storage

import (
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
)

func TestSecurityStorageInterface(t *testing.T) {
	// Test that our memory implementation satisfies the interface
	var storage SecurityStorage = NewMemorySecurityStorage()
	if storage == nil {
		t.Fatal("Expected storage to implement SecurityStorage interface")
	}
}

func TestSessionSecurityContext(t *testing.T) {
	// Test SessionSecurityContext struct
	fp := &fingerprint.DeviceFingerprint{
		UserAgent:   "test-agent",
		Fingerprint: "test-hash",
		CreatedAt:   time.Now().Unix(),
	}
	
	ctx := &SessionSecurityContext{
		DeviceFingerprint: fp,
		FirstSeen:         time.Now().Unix(),
		LastActivity:      time.Now().Unix(),
		FailedAttempts:    0,
		SuspiciousFlags:   []string{},
		UserID:            "user123",
	}
	
	if ctx.DeviceFingerprint != fp {
		t.Error("Expected DeviceFingerprint to be set correctly")
	}
	if ctx.UserID != "user123" {
		t.Error("Expected UserID to be set correctly")
	}
	if ctx.FailedAttempts != 0 {
		t.Error("Expected FailedAttempts to be initialized to 0")
	}
	if len(ctx.SuspiciousFlags) != 0 {
		t.Error("Expected SuspiciousFlags to be empty initially")
	}
}

func TestSecurityStorageBasicOperations(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Test that storage was created
	if storage == nil {
		t.Fatal("Expected storage to be created")
	}
	
	// Test basic interface compliance by calling all methods
	// (specific functionality is tested in memory_test.go)
	
	// Nonce operations
	err := storage.StoreNonce("test-nonce", 5*time.Minute)
	if err != nil {
		t.Errorf("StoreNonce failed: %v", err)
	}
	
	exists := storage.NonceExists("test-nonce")
	if !exists {
		t.Error("Expected nonce to exist")
	}
	
	// Device fingerprint operations
	fp := &fingerprint.DeviceFingerprint{
		UserAgent:   "test-agent",
		Fingerprint: "test-hash",
		CreatedAt:   time.Now().Unix(),
	}
	
	err = storage.StoreDeviceFingerprint("session1", fp)
	if err != nil {
		t.Errorf("StoreDeviceFingerprint failed: %v", err)
	}
	
	retrieved, err := storage.GetDeviceFingerprint("session1")
	if err != nil {
		t.Errorf("GetDeviceFingerprint failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Expected to retrieve stored fingerprint")
	}
	
	err = storage.DeleteDeviceFingerprint("session1")
	if err != nil {
		t.Errorf("DeleteDeviceFingerprint failed: %v", err)
	}
	
	// Device blocking operations
	blockUntil := time.Now().Add(1 * time.Hour)
	err = storage.BlockDevice("fingerprint1", blockUntil)
	if err != nil {
		t.Errorf("BlockDevice failed: %v", err)
	}
	
	blocked := storage.IsDeviceBlocked("fingerprint1")
	if !blocked {
		t.Error("Expected device to be blocked")
	}
	
	err = storage.UnblockDevice("fingerprint1")
	if err != nil {
		t.Errorf("UnblockDevice failed: %v", err)
	}
	
	// User session tracking operations
	err = storage.TrackUserSession("user1", "session1")
	if err != nil {
		t.Errorf("TrackUserSession failed: %v", err)
	}
	
	sessions := storage.GetUserSessions("user1")
	if len(sessions) != 1 || sessions[0] != "session1" {
		t.Error("Expected to retrieve tracked session")
	}
	
	err = storage.RemoveUserSession("user1", "session1")
	if err != nil {
		t.Errorf("RemoveUserSession failed: %v", err)
	}
	
	// Security context operations
	ctx := &SessionSecurityContext{
		DeviceFingerprint: fp,
		FirstSeen:         time.Now().Unix(),
		LastActivity:      time.Now().Unix(),
		FailedAttempts:    1,
		SuspiciousFlags:   []string{"test-flag"},
		UserID:            "user1",
	}
	
	err = storage.StoreSecurityContext("session1", ctx)
	if err != nil {
		t.Errorf("StoreSecurityContext failed: %v", err)
	}
	
	retrievedCtx, err := storage.GetSecurityContext("session1")
	if err != nil {
		t.Errorf("GetSecurityContext failed: %v", err)
	}
	if retrievedCtx == nil {
		t.Error("Expected to retrieve stored security context")
	}
	
	err = storage.DeleteSecurityContext("session1")
	if err != nil {
		t.Errorf("DeleteSecurityContext failed: %v", err)
	}
	
	// Cleanup operations
	err = storage.PruneExpired()
	if err != nil {
		t.Errorf("PruneExpired failed: %v", err)
	}
	
	err = storage.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}