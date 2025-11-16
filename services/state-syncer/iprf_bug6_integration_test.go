package main

import (
	"testing"
)

// TestNewIPRFFromMasterSecret tests the convenience constructor
func TestNewIPRFFromMasterSecret(t *testing.T) {
	masterSecret := []byte("test-master-secret")
	context := "plinko-iprf-v1"
	n := uint64(10000)
	m := uint64(100)

	// Create using convenience function
	iprf1 := NewIPRFFromMasterSecret(masterSecret, context, n, m)

	// Create using manual key derivation
	key := DeriveIPRFKey(masterSecret, context)
	iprf2 := NewIPRF(key, n, m)

	// Both should produce identical results
	for x := uint64(0); x < 100; x++ {
		y1 := iprf1.Forward(x)
		y2 := iprf2.Forward(x)

		if y1 != y2 {
			t.Errorf("NewIPRFFromMasterSecret inconsistent at x=%d: %d vs %d", x, y1, y2)
		}
	}

	t.Log("✓ NewIPRFFromMasterSecret produces consistent results")
}

// TestKeyDerivationDeterminism tests that key derivation is truly deterministic
func TestKeyDerivationDeterminism(t *testing.T) {
	masterSecret := []byte("production-secret")
	context := "app-v1"

	// Generate key multiple times
	key1 := DeriveIPRFKey(masterSecret, context)
	key2 := DeriveIPRFKey(masterSecret, context)
	key3 := DeriveIPRFKey(masterSecret, context)

	if key1 != key2 || key2 != key3 {
		t.Error("DeriveIPRFKey is not deterministic")
	}

	t.Log("✓ DeriveIPRFKey is deterministic")
}

// TestContextSeparation tests that different contexts produce different keys
func TestContextSeparation(t *testing.T) {
	masterSecret := []byte("shared-secret")

	keys := make(map[PrfKey128]string)

	contexts := []string{
		"plinko-iprf-v1",
		"plinko-iprf-v2",
		"test-context",
		"prod-context",
	}

	for _, ctx := range contexts {
		key := DeriveIPRFKey(masterSecret, ctx)

		if prevCtx, exists := keys[key]; exists {
			t.Errorf("Context collision: %s and %s produce same key", ctx, prevCtx)
		}
		keys[key] = ctx
	}

	t.Logf("✓ %d different contexts produce %d unique keys", len(contexts), len(keys))
}

// TestMasterSecretSeparation tests that different master secrets produce different keys
func TestMasterSecretSeparation(t *testing.T) {
	context := "plinko-iprf-v1"

	secrets := [][]byte{
		[]byte("secret-1"),
		[]byte("secret-2"),
		[]byte("production-secret"),
		[]byte("development-secret"),
	}

	keys := make(map[PrfKey128]string)

	for i, secret := range secrets {
		key := DeriveIPRFKey(secret, context)

		if prevIdx, exists := keys[key]; exists {
			t.Errorf("Master secret collision: secret[%d] and %s produce same key", i, prevIdx)
		}
		keys[key] = string(secret)
	}

	t.Logf("✓ %d different master secrets produce %d unique keys", len(secrets), len(keys))
}

// TestPersistenceScenario tests a realistic server restart scenario
func TestPersistenceScenario(t *testing.T) {
	// Simulate production deployment
	masterSecret := []byte("production-master-key-32-bytes!!")
	context := "plinko-iprf-production-v1"
	n := uint64(8400000)
	m := uint64(1024)

	// Initial server startup
	t.Log("Simulating initial server startup...")
	iprf1 := NewIPRFFromMasterSecret(masterSecret, context, n, m)

	// Server processes some queries and caches hints
	cachedHints := make(map[uint64]uint64)
	sampleIndices := []uint64{0, 100000, 1000000, 5000000, 8000000}
	for _, idx := range sampleIndices {
		hint := iprf1.Forward(idx)
		cachedHints[idx] = hint
		t.Logf("Cached: index %d → hint %d", idx, hint)
	}

	// Server restart (e.g., deployment, crash recovery)
	t.Log("Simulating server restart...")
	iprf2 := NewIPRFFromMasterSecret(masterSecret, context, n, m)

	// Verify cached hints are still valid
	allValid := true
	for idx, cachedHint := range cachedHints {
		newHint := iprf2.Forward(idx)
		if newHint != cachedHint {
			t.Errorf("Hint invalidated after restart: index %d, was %d, now %d",
				idx, cachedHint, newHint)
			allValid = false
		}
	}

	if allValid {
		t.Log("✓ All cached hints remain valid after server restart")
		t.Log("✓ Key persistence working correctly in production scenario")
	}
}

// TestMultipleIPRFInstances tests multiple iPRF instances with different contexts
func TestMultipleIPRFInstances(t *testing.T) {
	// Real-world scenario: Multiple iPRF instances for different purposes
	masterSecret := []byte("global-master-secret")

	// Different iPRF instances for different data types
	userIPRF := NewIPRFFromMasterSecret(masterSecret, "users-v1", 10000000, 1024)
	txIPRF := NewIPRFFromMasterSecret(masterSecret, "transactions-v1", 50000000, 2048)
	stateIPRF := NewIPRFFromMasterSecret(masterSecret, "state-v1", 8400000, 1024)

	// Verify they produce different mappings
	testIndex := uint64(12345)

	userHint := userIPRF.Forward(testIndex)
	txHint := txIPRF.Forward(testIndex)
	stateHint := stateIPRF.Forward(testIndex)

	t.Logf("Index %d maps to:", testIndex)
	t.Logf("  User hint: %d", userHint)
	t.Logf("  Transaction hint: %d", txHint)
	t.Logf("  State hint: %d", stateHint)

	// They should be independent (different contexts)
	if userHint == txHint && txHint == stateHint {
		t.Error("All iPRF instances produce same output - context separation failed")
	}

	t.Log("✓ Multiple iPRF instances with different contexts work independently")
}
