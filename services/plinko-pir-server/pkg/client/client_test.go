package client

import (
	"testing"
)

func TestClientEndToEnd(t *testing.T) {
	// 1. Setup Mock Database
	n := uint64(1000)
	db := make([]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i] = i * 100 // Some value
	}

	// 2. Initialize Client
	m := uint64(50) // Number of hints
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)
	keyBeta[0] = 1

	c := NewClient(n, m, keyAlpha, keyBeta)

	// 3. Offline Phase: HintInit
	// Stream the database
	iter := 0
	dbStream := func() (DBEntry, bool) {
		if iter >= int(n) {
			return DBEntry{}, false
		}
		entry := DBEntry{
			Index: uint64(iter),
			Value: db[iter],
		}
		iter++
		return entry, true
	}

	c.HintInit(dbStream)

	// 4. Online Phase: Query
	targetIndex := uint64(42)
	expectedValue := db[targetIndex]

	req, hint, err := c.Query(targetIndex)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if req == nil {
		t.Fatalf("Query returned nil request")
	}

	// 5. Simulate Server
	// Server computes parity of indices in req.Indices
	serverParity := uint64(0)
	for _, idx := range req.Indices {
		if idx >= n {
			t.Fatalf("Client requested out of bounds index: %d", idx)
		}
		serverParity ^= db[idx]
	}

	// 6. Reconstruction
	val := c.Reconstruct(serverParity, hint)

	if val != expectedValue {
		t.Errorf("Reconstruction failed: got %d, want %d", val, expectedValue)
	}
}

func TestClientBackupHints(t *testing.T) {
	n := uint64(100)
	db := make([]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i] = uint64(i)
	}

	m := uint64(10)
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)

	c := NewClient(n, m, keyAlpha, keyBeta)

	// Helper to reset stream
	getStream := func() func() (DBEntry, bool) {
		iter := 0
		return func() (DBEntry, bool) {
			if iter >= int(n) {
				return DBEntry{}, false
			}
			entry := DBEntry{Index: uint64(iter), Value: db[iter]}
			iter++
			return entry, true
		}
	}

	c.HintInit(getStream())

	// Initialize Backup Hints
	// We need enough backup hints to cover our target index with high probability
	// With n=100, setSize=10, prob is 0.1 per hint.
	// 50 hints -> expected 5 hits.
	c.InitBackupHints(50, 10, getStream())

	target := uint64(55)

	// 1. First Query (Primary)
	req1, hint1, err := c.Query(target)
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}
	if hint1.Used != true {
		t.Errorf("Primary hint should be marked used")
	}

	// Verify result 1
	parity1 := uint64(0)
	for _, idx := range req1.Indices {
		parity1 ^= db[idx]
	}
	val1 := c.Reconstruct(parity1, hint1)
	if val1 != db[target] {
		t.Errorf("First query result wrong: got %d, want %d", val1, db[target])
	}

	// 2. Second Query (Backup)
	// Should use backup hint because primary is used
	req2, hint2, err := c.Query(target)
	if err != nil {
		t.Fatalf("Second query failed (backup not found?): %v", err)
	}

	// Verify it's a backup hint (Indices should be populated)
	if len(hint2.Indices) == 0 {
		t.Errorf("Second query used a hint without indices (likely primary reused?)")
	}
	if hint2.Used != true {
		t.Errorf("Backup hint should be marked used")
	}

	// Verify result 2
	parity2 := uint64(0)
	for _, idx := range req2.Indices {
		parity2 ^= db[idx]
	}
	val2 := c.Reconstruct(parity2, hint2)
	if val2 != db[target] {
		t.Errorf("Second query result wrong: got %d, want %d", val2, db[target])
	}
}

func TestClientUpdate(t *testing.T) {
	n := uint64(100)
	db := make([]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i] = uint64(i)
	}

	m := uint64(10)
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)

	c := NewClient(n, m, keyAlpha, keyBeta)

	// Helper to reset stream
	getStream := func() func() (DBEntry, bool) {
		iter := 0
		return func() (DBEntry, bool) {
			if iter >= int(n) {
				return DBEntry{}, false
			}
			entry := DBEntry{Index: uint64(iter), Value: db[iter]}
			iter++
			return entry, true
		}
	}

	c.HintInit(getStream())

	target := uint64(42)
	oldValue := db[target]
	newValue := uint64(999)

	// 1. Verify initial query works
	// We don't consume the hint yet, or we reset?
	// Let's just query.
	req1, hint1, err := c.Query(target)
	if err != nil {
		t.Fatalf("Initial query failed: %v", err)
	}

	// Compute parity 1
	parity1 := uint64(0)
	for _, idx := range req1.Indices {
		parity1 ^= db[idx]
	}
	val1 := c.Reconstruct(parity1, hint1)
	if val1 != oldValue {
		t.Errorf("Initial value wrong: got %d, want %d", val1, oldValue)
	}

	// 2. Perform Update
	// Update DB
	db[target] = newValue
	delta := oldValue ^ newValue

	// Update Client Hints
	c.UpdateHint(target, delta)

	// 3. Verify query after update
	// Note: hint1 is marked used. We need another hint or we reset usage for testing?
	// Or we rely on backup hints?
	// Let's initialize backup hints first.
	c.InitBackupHints(10, 10, getStream()) // Note: getStream uses NEW db values?
	// Wait, InitBackupHints streams the DB. If we call it NOW, it will use the NEW DB values.
	// So backup hints will be correct for the NEW DB.
	// But we want to test UpdateHint on EXISTING hints.
	// So we should init backup hints BEFORE update.

	// Reset and start over for clean test
	db[target] = oldValue // Reset DB
	c = NewClient(n, m, keyAlpha, keyBeta)
	c.HintInit(getStream())
	c.InitBackupHints(50, 10, getStream())

	// Now update
	db[target] = newValue
	delta = oldValue ^ newValue
	c.UpdateHint(target, delta)

	// Query again (should use primary hint if not used, or backup)
	// Since we created a NEW client, primary hint is unused.
	// It should reflect the update.
	req2, hint2, err := c.Query(target)
	if err != nil {
		t.Fatalf("Post-update query failed: %v", err)
	}

	// Compute parity 2 (using NEW DB)
	parity2 := uint64(0)
	for _, idx := range req2.Indices {
		parity2 ^= db[idx]
	}
	val2 := c.Reconstruct(parity2, hint2)
	if val2 != newValue {
		t.Errorf("Post-update value wrong: got %d, want %d", val2, newValue)
	}

	// 4. Verify Backup Hint Update
	// Consume primary hint
	// (hint2 was primary and is now used)

	// Query again -> should use backup hint
	req3, hint3, err := c.Query(target)
	if err != nil {
		t.Fatalf("Backup query failed: %v", err)
	}

	// Verify it's backup
	if len(hint3.Indices) == 0 {
		t.Errorf("Expected backup hint")
	}

	// Compute parity 3
	parity3 := uint64(0)
	for _, idx := range req3.Indices {
		parity3 ^= db[idx]
	}
	val3 := c.Reconstruct(parity3, hint3)
	if val3 != newValue {
		t.Errorf("Backup hint value wrong after update: got %d, want %d", val3, newValue)
	}
}
