package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/jwtprovider"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-user-tracking")

	// Create JWT provider with user tracking enabled
	signingKey := []byte("my-secret-key-for-jwt-signing-must-be-32-bytes-long!")
	cfg, err := jwtprovider.NewJWTConfigWithKey(signingKey)
	if err != nil {
		logger.Fatal(err, "Failed to create config")
	}
	cfg.TrackUserTokens = true    // Enable user token tracking
	cfg.MaxUserSessions = 3       // Maximum 3 concurrent sessions per user
	
	// Create revocation manager
	revocationMgr := jwtprovider.NewRevocationManager(
		jwtprovider.NewMemoryRevocationBackend(),
	)
	
	// Create provider with tracking enabled
	provider, err := jwtprovider.NewProvider(
		cfg,
		jwtprovider.WithRevocationManager(revocationMgr),
	)
	if err != nil {
		logger.Fatal(err, "Failed to create provider")
	}
	defer revocationMgr.Close()

	// Demonstrate user token tracking
	userID := "user123"
	
	fmt.Printf("=== JWT User Token Tracking Demo ===\n\n")
	
	// Generate multiple tokens for the same user
	fmt.Printf("Generating tokens for user '%s'...\n", userID)
	
	var tokens []string
	for i := 1; i <= 4; i++ {
		token, err := provider.GenerateToken(userID, map[string]any{
			"session_id": fmt.Sprintf("session_%d", i),
		})
		
		if err != nil {
			if err == jwtprovider.ErrMaxSessionsExceeded {
				fmt.Printf("  Token %d: FAILED - %v\n", i, err)
				continue
			}
			logger.Fatal(err, "Failed to generate token")
		}
		
		tokens = append(tokens, token)
		fmt.Printf("  Token %d: SUCCESS (length: %d)\n", i, len(token))
		
		// Show current session count
		count := provider.GetUserSessionCount(userID)
		fmt.Printf("    Current sessions: %d\n", count)
	}
	
	fmt.Println()
	
	// Show active tokens
	activeTokens, err := provider.GetActiveUserTokens(userID)
	if err != nil {
		logger.Fatal(err, "Failed to get active tokens")
	}
	
	fmt.Printf("Active tokens for user '%s': %d\n", userID, len(activeTokens))
	for i, tokenID := range activeTokens {
		fmt.Printf("  %d. %s\n", i+1, tokenID)
	}
	
	fmt.Println()
	
	// Revoke one token manually
	if len(tokens) > 0 {
		fmt.Printf("Revoking one token manually...\n")
		err = provider.RevokeToken(tokens[0])
		if err != nil {
			logger.Fatal(err, "Failed to revoke token")
		}
		
		count := provider.GetUserSessionCount(userID)
		fmt.Printf("Sessions after revocation: %d\n\n", count)
	}
	
	// Revoke all user tokens
	fmt.Printf("Revoking all tokens for user '%s'...\n", userID)
	err = provider.RevokeAllUserTokens(userID)
	if err != nil {
		logger.Fatal(err, "Failed to revoke all user tokens")
	}
	
	count := provider.GetUserSessionCount(userID)
	fmt.Printf("Sessions after revoking all: %d\n\n", count)
	
	// Test token parsing after revocation
	fmt.Printf("Testing token parsing after revocation...\n")
	for i, token := range tokens {
		_, err := provider.ParseToken(token)
		if err != nil {
			fmt.Printf("  Token %d: REVOKED (%v)\n", i+1, err)
		} else {
			fmt.Printf("  Token %d: VALID\n", i+1)
		}
	}
	
	fmt.Printf("\n=== Demo Complete ===\n")
}