package main

import (
	"testing"
)

// TestBug6KeyPersistence tests that iPRF behavior persists across restarts
// with deterministic key derivation from master secret
//
// BUG #6: Using GenerateRandomKey() means each server restart produces different
// iPRF mappings, invalidating all cached hints. Paper specifies deterministic
// key derivation from master secret.
//
// This test MUST FAIL if DeriveIPRFKey is not implemented
func TestBug6KeyPersistence(t *testing.T) {
	// Simulate server restart scenario
	masterSecret := []byte("test-master-secret-32-bytes-long!")
	n := uint64(100000)
	m := uint64(256)

	// First "server session"
	key1 := DeriveIPRFKey(masterSecret, "session1")
	iprf1 := NewIPRF(key1, n, m)

	// Compute some forward values
	testValues := []uint64{0, 100, 1000, 10000, 50000}
	results1 := make(map[uint64]uint64)
	for _, x := range testValues {
		results1[x] = iprf1.Forward(x)
	}

	// "Server restart" - new iPRF instance with same master secret
	key2 := DeriveIPRFKey(masterSecret, "session1")
	iprf2 := NewIPRF(key2, n, m)

	// Verify all values unchanged
	for _, x := range testValues {
		result2 := iprf2.Forward(x)
		if result2 != results1[x] {
			t.Errorf("Persistence failure: Forward(%d) changed after restart: %d → %d",
				x, results1[x], result2)
		}
	}

	t.Log("✓ iPRF behavior persists across restarts with deterministic key")

	// Test that random keys would NOT persist
	randomKey1 := GenerateRandomKey()
	randomKey2 := GenerateRandomKey()

	if randomKey1 == randomKey2 {
		t.Error("Random keys should be different (extremely unlikely to be same)")
	}

	t.Log("✓ Random keys are different (would break persistence)")
}

// TestBug6KeyDerivationNeeded documents the need for DeriveIPRFKey function
func TestBug6KeyDerivationNeeded(t *testing.T) {
	// Test the implemented DeriveIPRFKey function

	masterSecret := []byte("test-master-secret")
	context1 := "iprf-session-1"
	context2 := "iprf-session-2"

	// Test key derivation
	key1 := DeriveIPRFKey(masterSecret, context1)
	key2 := DeriveIPRFKey(masterSecret, context1) // Same context
	key3 := DeriveIPRFKey(masterSecret, context2) // Different context

	if key1 != key2 {
		t.Error("Same context should produce same key")
	}

	if key1 == key3 {
		t.Error("Different contexts should produce different keys")
	}

	t.Log("✓ DeriveIPRFKey implemented and working correctly")
}

// TestBug6ProductionScenario tests the production use case
func TestBug6ProductionScenario(t *testing.T) {
	// Production scenario: Server has master secret, derives iPRF key

	// Simulate loading master secret from config
	masterSecret := []byte("production-master-secret-must-be-32-bytes!")

	// Use DeriveIPRFKey for production-safe key generation
	key := DeriveIPRFKey(masterSecret, "plinko-iprf-v1")

	n := uint64(8400000)
	m := uint64(1024)

	iprf := NewIPRF(key, n, m)

	// Compute some values
	testInputs := []uint64{0, 1000000, 5000000, 8000000}
	results := make(map[uint64]uint64)

	for _, x := range testInputs {
		results[x] = iprf.Forward(x)
		t.Logf("Forward(%d) = %d", x, results[x])
	}

	// Simulate server restart with same key
	iprf2 := NewIPRF(key, n, m)

	// Verify persistence
	allMatch := true
	for _, x := range testInputs {
		result2 := iprf2.Forward(x)
		if result2 != results[x] {
			t.Errorf("Production persistence failure: Forward(%d) changed after restart", x)
			allMatch = false
		}
	}

	if allMatch {
		t.Log("✓ Production scenario: iPRF persists across restarts")
	}
}

// TestBug6RandomKeyBreaksPersistence demonstrates the bug
func TestBug6RandomKeyBreaksPersistence(t *testing.T) {
	// This test shows WHY random keys are bad for production

	n := uint64(10000)
	m := uint64(100)

	// First server session with random key
	randomKey1 := GenerateRandomKey()
	iprf1 := NewIPRF(randomKey1, n, m)

	// Compute some hints
	hints1 := make(map[uint64]uint64)
	for x := uint64(0); x < 100; x++ {
		hints1[x] = iprf1.Forward(x)
	}

	// Server restart with NEW random key (simulating the bug)
	randomKey2 := GenerateRandomKey()
	iprf2 := NewIPRF(randomKey2, n, m)

	// Check how many hints changed
	changedCount := 0
	for x := uint64(0); x < 100; x++ {
		hint2 := iprf2.Forward(x)
		if hint2 != hints1[x] {
			changedCount++
		}
	}

	// With random keys, almost ALL hints will change
	percentChanged := float64(changedCount) / 100.0 * 100

	t.Logf("BUG #6 DEMONSTRATION: Random keys caused %.0f%% of hints to change after restart",
		percentChanged)

	if percentChanged > 50 {
		t.Logf("This would invalidate all cached hints in production!")
		t.Logf("Solution: Use DeriveIPRFKey with master secret for deterministic keys")
	}
}
