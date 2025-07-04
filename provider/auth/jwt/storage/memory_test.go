package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
)

func TestNewMemorySecurityStorage(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	if storage == nil {
		t.Fatal("Expected storage to be created")
	}
	
	// Verify it implements the interface
	var _ SecurityStorage = storage
}

func TestMemoryStorageNonceOperations(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Test storing a nonce
	nonce := "test-nonce-123"
	ttl := 5 * time.Minute
	
	err := storage.StoreNonce(nonce, ttl)
	if err != nil {
		t.Errorf("Failed to store nonce: %v", err)
	}
	
	// Test checking if nonce exists
	if !storage.NonceExists(nonce) {
		t.Error("Expected nonce to exist after storing")
	}
	
	// Test non-existent nonce
	if storage.NonceExists("non-existent-nonce") {
		t.Error("Expected non-existent nonce to not exist")
	}
	
	// Test storing same nonce again (should work)
	err = storage.StoreNonce(nonce, ttl)
	if err != nil {
		t.Errorf("Failed to store same nonce again: %v", err)
	}
}

func TestMemoryStorageNonceExpiration(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Store nonce that expires in the past (already expired)
	nonce := "expired-nonce"
	pastTime := -1 * time.Hour // 1 hour ago
	err := storage.StoreNonce(nonce, pastTime)
	if err != nil {
		t.Errorf("Failed to store nonce: %v", err)
	}
	
	// Should not exist because it's already expired
	if storage.NonceExists(nonce) {
		t.Error("Expected already-expired nonce to not exist")
	}
}

func TestMemoryStorageDeviceFingerprintOperations(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	fp := &fingerprint.DeviceFingerprint{
		UserAgent:   "test-agent",
		AcceptLang:  "en-US",
		AcceptEnc:   "gzip",
		IPAddress:   "192.168.1.100",
		IPSubnet:    "192.168.1.0/24",
		Timezone:    "UTC",
		Fingerprint: "test-fingerprint-hash",
		Country:     "US",
		CreatedAt:   time.Now().Unix(),
	}
	
	sessionID := "session-123"
	
	// Test storing fingerprint
	err := storage.StoreDeviceFingerprint(sessionID, fp)
	if err != nil {
		t.Errorf("Failed to store device fingerprint: %v", err)
	}
	
	// Test retrieving fingerprint
	retrieved, err := storage.GetDeviceFingerprint(sessionID)
	if err != nil {
		t.Errorf("Failed to get device fingerprint: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to retrieve stored fingerprint")
	}
	
	// Verify fingerprint data
	if retrieved.UserAgent != fp.UserAgent {
		t.Errorf("Expected UserAgent '%s', got '%s'", fp.UserAgent, retrieved.UserAgent)
	}
	if retrieved.Fingerprint != fp.Fingerprint {
		t.Errorf("Expected Fingerprint '%s', got '%s'", fp.Fingerprint, retrieved.Fingerprint)
	}
	if retrieved.IPAddress != fp.IPAddress {
		t.Errorf("Expected IPAddress '%s', got '%s'", fp.IPAddress, retrieved.IPAddress)
	}
	
	// Test that retrieved fingerprint is a copy (mutation safety)
	retrieved.UserAgent = "modified"
	retrieved2, _ := storage.GetDeviceFingerprint(sessionID)
	if retrieved2.UserAgent == "modified" {
		t.Error("Expected stored fingerprint to be unmodified (should be copied)")
	}
	
	// Test deleting fingerprint
	err = storage.DeleteDeviceFingerprint(sessionID)
	if err != nil {
		t.Errorf("Failed to delete device fingerprint: %v", err)
	}
	
	// Verify deletion
	deleted, err := storage.GetDeviceFingerprint(sessionID)
	if err != nil {
		t.Errorf("Unexpected error getting deleted fingerprint: %v", err)
	}
	if deleted != nil {
		t.Error("Expected deleted fingerprint to be nil")
	}
	
	// Test getting non-existent fingerprint
	nonExistent, err := storage.GetDeviceFingerprint("non-existent")
	if err != nil {
		t.Errorf("Unexpected error getting non-existent fingerprint: %v", err)
	}
	if nonExistent != nil {
		t.Error("Expected non-existent fingerprint to be nil")
	}
}

func TestMemoryStorageDeviceBlockingOperations(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	deviceFingerprint := "device-fingerprint-123"
	blockUntil := time.Now().Add(1 * time.Hour)
	
	// Initially not blocked
	if storage.IsDeviceBlocked(deviceFingerprint) {
		t.Error("Expected device to not be blocked initially")
	}
	
	// Block device
	err := storage.BlockDevice(deviceFingerprint, blockUntil)
	if err != nil {
		t.Errorf("Failed to block device: %v", err)
	}
	
	// Should be blocked now
	if !storage.IsDeviceBlocked(deviceFingerprint) {
		t.Error("Expected device to be blocked after blocking")
	}
	
	// Unblock device
	err = storage.UnblockDevice(deviceFingerprint)
	if err != nil {
		t.Errorf("Failed to unblock device: %v", err)
	}
	
	// Should not be blocked anymore
	if storage.IsDeviceBlocked(deviceFingerprint) {
		t.Error("Expected device to not be blocked after unblocking")
	}
}

func TestMemoryStorageDeviceBlockingExpiration(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	deviceFingerprint := "expired-block"
	// Block device until 1 hour ago (already expired)
	blockUntil := time.Now().Add(-1 * time.Hour)
	
	// Block device with expired time
	err := storage.BlockDevice(deviceFingerprint, blockUntil)
	if err != nil {
		t.Errorf("Failed to block device: %v", err)
	}
	
	// Should not be blocked because it's already expired
	if storage.IsDeviceBlocked(deviceFingerprint) {
		t.Error("Expected already-expired device block to not be blocked")
	}
}

func TestMemoryStorageUserSessionTracking(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	userID := "user-123"
	session1 := "session-1"
	session2 := "session-2"
	session3 := "session-3"
	
	// Initially no sessions
	sessions := storage.GetUserSessions(userID)
	if len(sessions) != 0 {
		t.Error("Expected no sessions initially")
	}
	
	// Track first session
	err := storage.TrackUserSession(userID, session1)
	if err != nil {
		t.Errorf("Failed to track session: %v", err)
	}
	
	sessions = storage.GetUserSessions(userID)
	if len(sessions) != 1 || sessions[0] != session1 {
		t.Error("Expected one tracked session")
	}
	
	// Track second session
	err = storage.TrackUserSession(userID, session2)
	if err != nil {
		t.Errorf("Failed to track second session: %v", err)
	}
	
	sessions = storage.GetUserSessions(userID)
	if len(sessions) != 2 {
		t.Error("Expected two tracked sessions")
	}
	
	// Track duplicate session (should not add again)
	err = storage.TrackUserSession(userID, session1)
	if err != nil {
		t.Errorf("Failed to track duplicate session: %v", err)
	}
	
	sessions = storage.GetUserSessions(userID)
	if len(sessions) != 2 {
		t.Error("Expected duplicate session tracking to not increase count")
	}
	
	// Test that returned sessions is a copy (mutation safety)
	sessions[0] = "modified"
	sessions2 := storage.GetUserSessions(userID)
	if sessions2[0] == "modified" {
		t.Error("Expected returned sessions to be a copy")
	}
	
	// Remove first session
	err = storage.RemoveUserSession(userID, session1)
	if err != nil {
		t.Errorf("Failed to remove session: %v", err)
	}
	
	sessions = storage.GetUserSessions(userID)
	if len(sessions) != 1 || sessions[0] != session2 {
		t.Error("Expected first session to be removed")
	}
	
	// Remove non-existent session (should not error)
	err = storage.RemoveUserSession(userID, session3)
	if err != nil {
		t.Errorf("Unexpected error removing non-existent session: %v", err)
	}
	
	// Remove last session
	err = storage.RemoveUserSession(userID, session2)
	if err != nil {
		t.Errorf("Failed to remove last session: %v", err)
	}
	
	sessions = storage.GetUserSessions(userID)
	if len(sessions) != 0 {
		t.Error("Expected no sessions after removing all")
	}
	
	// Test removing from non-existent user (should not error)
	err = storage.RemoveUserSession("non-existent-user", "some-session")
	if err != nil {
		t.Errorf("Unexpected error removing session from non-existent user: %v", err)
	}
}

func TestMemoryStorageSecurityContextOperations(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	fp := &fingerprint.DeviceFingerprint{
		UserAgent:   "test-agent",
		Fingerprint: "test-hash",
		CreatedAt:   time.Now().Unix(),
	}
	
	ctx := &SessionSecurityContext{
		DeviceFingerprint: fp,
		FirstSeen:         time.Now().Unix(),
		LastActivity:      time.Now().Unix(),
		FailedAttempts:    2,
		SuspiciousFlags:   []string{"flag1", "flag2"},
		UserID:            "user123",
	}
	
	sessionID := "session-456"
	
	// Test storing security context
	err := storage.StoreSecurityContext(sessionID, ctx)
	if err != nil {
		t.Errorf("Failed to store security context: %v", err)
	}
	
	// Test retrieving security context
	retrieved, err := storage.GetSecurityContext(sessionID)
	if err != nil {
		t.Errorf("Failed to get security context: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to retrieve stored security context")
	}
	
	// Verify context data
	if retrieved.UserID != ctx.UserID {
		t.Errorf("Expected UserID '%s', got '%s'", ctx.UserID, retrieved.UserID)
	}
	if retrieved.FailedAttempts != ctx.FailedAttempts {
		t.Errorf("Expected FailedAttempts %d, got %d", ctx.FailedAttempts, retrieved.FailedAttempts)
	}
	if len(retrieved.SuspiciousFlags) != len(ctx.SuspiciousFlags) {
		t.Errorf("Expected %d flags, got %d", len(ctx.SuspiciousFlags), len(retrieved.SuspiciousFlags))
	}
	if retrieved.DeviceFingerprint.UserAgent != fp.UserAgent {
		t.Error("Expected device fingerprint to be copied correctly")
	}
	
	// Test that retrieved context is a copy (mutation safety)
	retrieved.UserID = "modified"
	retrieved.SuspiciousFlags[0] = "modified"
	retrieved.DeviceFingerprint.UserAgent = "modified"
	
	retrieved2, _ := storage.GetSecurityContext(sessionID)
	if retrieved2.UserID == "modified" {
		t.Error("Expected stored context UserID to be unmodified")
	}
	if retrieved2.SuspiciousFlags[0] == "modified" {
		t.Error("Expected stored context flags to be unmodified")
	}
	if retrieved2.DeviceFingerprint.UserAgent == "modified" {
		t.Error("Expected stored context fingerprint to be unmodified")
	}
	
	// Test deleting security context
	err = storage.DeleteSecurityContext(sessionID)
	if err != nil {
		t.Errorf("Failed to delete security context: %v", err)
	}
	
	// Verify deletion
	deleted, err := storage.GetSecurityContext(sessionID)
	if err != nil {
		t.Errorf("Unexpected error getting deleted context: %v", err)
	}
	if deleted != nil {
		t.Error("Expected deleted context to be nil")
	}
	
	// Test getting non-existent context
	nonExistent, err := storage.GetSecurityContext("non-existent")
	if err != nil {
		t.Errorf("Unexpected error getting non-existent context: %v", err)
	}
	if nonExistent != nil {
		t.Error("Expected non-existent context to be nil")
	}
}

func TestMemoryStorageSecurityContextWithNilFingerprint(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	ctx := &SessionSecurityContext{
		DeviceFingerprint: nil, // Test with nil fingerprint
		FirstSeen:         time.Now().Unix(),
		LastActivity:      time.Now().Unix(),
		FailedAttempts:    0,
		SuspiciousFlags:   []string{},
		UserID:            "user456",
	}
	
	sessionID := "session-nil-fp"
	
	// Should work with nil fingerprint
	err := storage.StoreSecurityContext(sessionID, ctx)
	if err != nil {
		t.Errorf("Failed to store context with nil fingerprint: %v", err)
	}
	
	retrieved, err := storage.GetSecurityContext(sessionID)
	if err != nil {
		t.Errorf("Failed to get context with nil fingerprint: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to retrieve stored context")
	}
	if retrieved.DeviceFingerprint != nil {
		t.Error("Expected retrieved fingerprint to be nil")
	}
}

func TestMemoryStoragePruneExpired(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Add some expired and non-expired data
	
	// Expired nonce (TTL in the past)
	storage.StoreNonce("expired-nonce", -1*time.Hour)
	
	// Non-expired nonce
	storage.StoreNonce("valid-nonce", 1*time.Hour)
	
	// Expired device block
	storage.BlockDevice("expired-device", time.Now().Add(-1*time.Hour))
	
	// Non-expired device block
	storage.BlockDevice("valid-device", time.Now().Add(1*time.Hour))
	
	// Run cleanup
	err := storage.PruneExpired()
	if err != nil {
		t.Errorf("Failed to prune expired data: %v", err)
	}
	
	// Check that expired items are removed and valid ones remain
	if storage.NonceExists("expired-nonce") {
		t.Error("Expected expired nonce to be removed")
	}
	if !storage.NonceExists("valid-nonce") {
		t.Error("Expected valid nonce to remain")
	}
	
	if storage.IsDeviceBlocked("expired-device") {
		t.Error("Expected expired device block to be removed")
	}
	if !storage.IsDeviceBlocked("valid-device") {
		t.Error("Expected valid device block to remain")
	}
}

func TestMemoryStorageClose(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Add some data
	storage.StoreNonce("test-nonce", 1*time.Hour)
	fp := &fingerprint.DeviceFingerprint{UserAgent: "test", CreatedAt: time.Now().Unix()}
	storage.StoreDeviceFingerprint("session1", fp)
	storage.BlockDevice("device1", time.Now().Add(1*time.Hour))
	storage.TrackUserSession("user1", "session1")
	ctx := &SessionSecurityContext{UserID: "user1"}
	storage.StoreSecurityContext("session1", ctx)
	
	// Verify data exists
	if !storage.NonceExists("test-nonce") {
		t.Error("Expected test data to exist before close")
	}
	
	// Close storage
	err := storage.Close()
	if err != nil {
		t.Errorf("Failed to close storage: %v", err)
	}
	
	// Verify all data is cleared
	if storage.NonceExists("test-nonce") {
		t.Error("Expected nonce to be cleared after close")
	}
	
	retrieved, _ := storage.GetDeviceFingerprint("session1")
	if retrieved != nil {
		t.Error("Expected fingerprint to be cleared after close")
	}
	
	if storage.IsDeviceBlocked("device1") {
		t.Error("Expected device block to be cleared after close")
	}
	
	sessions := storage.GetUserSessions("user1")
	if len(sessions) != 0 {
		t.Error("Expected user sessions to be cleared after close")
	}
	
	ctxRetrieved, _ := storage.GetSecurityContext("session1")
	if ctxRetrieved != nil {
		t.Error("Expected security context to be cleared after close")
	}
}

func TestMemoryStorageConcurrentAccess(t *testing.T) {
	storage := NewMemorySecurityStorage()
	
	// Test concurrent access to ensure thread safety
	var wg sync.WaitGroup
	
	// Number of concurrent goroutines
	numRoutines := 10
	numOperations := 100
	
	// Concurrent nonce operations
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				nonce := fmt.Sprintf("nonce-%d-%d", routineID, j)
				storage.StoreNonce(nonce, 1*time.Hour)
				storage.NonceExists(nonce)
			}
		}(i)
	}
	
	// Concurrent device fingerprint operations
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sessionID := fmt.Sprintf("session-%d-%d", routineID, j)
				fp := &fingerprint.DeviceFingerprint{
					UserAgent: fmt.Sprintf("agent-%d-%d", routineID, j),
					CreatedAt: time.Now().Unix(),
				}
				storage.StoreDeviceFingerprint(sessionID, fp)
				storage.GetDeviceFingerprint(sessionID)
				storage.DeleteDeviceFingerprint(sessionID)
			}
		}(i)
	}
	
	// Concurrent user session operations
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				userID := fmt.Sprintf("user-%d", routineID)
				sessionID := fmt.Sprintf("session-%d-%d", routineID, j)
				storage.TrackUserSession(userID, sessionID)
				storage.GetUserSessions(userID)
				storage.RemoveUserSession(userID, sessionID)
			}
		}(i)
	}
	
	// Wait for all operations to complete
	wg.Wait()
	
	// If we get here without deadlock or panic, thread safety is working
	t.Log("Concurrent access test completed successfully")
}