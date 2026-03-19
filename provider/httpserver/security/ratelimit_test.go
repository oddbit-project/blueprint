package security

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestClientRateLimiter_MaxClientsEnforced(t *testing.T) {
	rl := &ClientRateLimiter{
		clients:      make(map[string]*clientEntry),
		rate:         rate.Limit(10),
		burst:        1,
		clientExpiry: 1<<63 - 1, // very long expiry so nothing expires
		maxClients:   3,
	}
	defer rl.Stop()

	// Fill to capacity
	for i := 0; i < 3; i++ {
		rl.GetLimiter(fmt.Sprintf("1.0.0.%d", i))
	}
	assert.Equal(t, 3, len(rl.clients))

	// Add one more — should evict oldest and stay at maxClients
	rl.GetLimiter("2.0.0.1")
	assert.LessOrEqual(t, len(rl.clients), rl.maxClients,
		"client count should not exceed maxClients")
}
