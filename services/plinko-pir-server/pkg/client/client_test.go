package client

import (
	"testing"
)

func newDeterministicRandSource() func(max uint64) uint64 {
	var counter uint64
	return func(max uint64) uint64 {
		if max == 0 {
			return 0
		}
		v := counter % max
		counter++
		return v
	}
}

func TestClientEndToEnd(t *testing.T) {
	// 1. Setup Mock Database
	n := uint64(1000)
	db := make([][4]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i][0] = i * 100 // Some value
	}

	// 2. Initialize Client
	m := uint64(50) // Number of hints
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)
	keyBeta[0] = 1

	c := NewClient(n, m, keyAlpha, keyBeta)
	c.randSource = newDeterministicRandSource()

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

	indices, hint, ok := c.Query(targetIndex)
	if !ok {
		t.Fatalf("Query failed: no hint found")
	}

	if indices == nil {
		t.Fatalf("Query returned nil request")
	}

	// 5. Simulate Server
	// Server computes parity of indices in req.Indices
	var serverParity [4]uint64
	for _, idx := range indices {
		if idx >= n {
			t.Fatalf("Client requested out of bounds index: %d", idx)
		}
		serverParity[0] ^= db[idx][0]
	}

	// 6. Reconstruction
	val := c.Reconstruct(serverParity, hint)

	if val[0] != expectedValue[0] {
		t.Errorf("Reconstruction failed: got %d, want %d", val[0], expectedValue[0])
	}
}

func TestClientBackupHints(t *testing.T) {
	n := uint64(100)
	db := make([][4]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i][0] = uint64(i)
	}

	m := uint64(10)
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)

	c := NewClient(n, m, keyAlpha, keyBeta)
	c.randSource = newDeterministicRandSource()

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
	indices1, hint1, ok := c.Query(target)
	if !ok {
		t.Fatalf("First query failed: no hint found")
	}
	if hint1.Used != true {
		t.Errorf("Primary hint should be marked used")
	}

	// Verify result 1
	var parity1 [4]uint64
	for _, idx := range indices1 {
		parity1[0] ^= db[idx][0]
	}
	val1 := c.Reconstruct(parity1, hint1)
	if val1[0] != db[target][0] {
		t.Errorf("First query result wrong: got %d, want %d", val1[0], db[target][0])
	}

	// 2. Second Query (Backup)
	// Should use backup hint because primary is used
	indices2, hint2, ok := c.Query(target)
	if !ok {
		t.Fatalf("Second query failed (backup not found?)")
	}

	// Verify it's a backup hint (Indices should be populated)
	if len(hint2.Indices) == 0 {
		t.Errorf("Second query used a hint without indices (likely primary reused?)")
	}
	if hint2.Used != true {
		t.Errorf("Backup hint should be marked used")
	}

	// Verify result 2
	var parity2 [4]uint64
	for _, idx := range indices2 {
		parity2[0] ^= db[idx][0]
	}
	val2 := c.Reconstruct(parity2, hint2)
	if val2[0] != db[target][0] {
		t.Errorf("Second query result wrong: got %d, want %d", val2[0], db[target][0])
	}
}

func TestClientUpdate(t *testing.T) {
	n := uint64(100)
	db := make([][4]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i][0] = uint64(i)
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
	oldValue := db[target][0]
	newValue := uint64(999)

	// 1. Verify initial query works
	// We don't consume the hint yet, or we reset?
	// Let's just query.
	indices1, hint1, ok := c.Query(target)
	if !ok {
		t.Fatalf("Initial query failed: no hint found")
	}

	// Compute parity 1
	var parity1 [4]uint64
	for _, idx := range indices1 {
		parity1[0] ^= db[idx][0]
	}
	val1 := c.Reconstruct(parity1, hint1)
	if val1[0] != oldValue {
		t.Errorf("Initial value wrong: got %d, want %d", val1[0], oldValue)
	}

	// 2. Perform Update
	// Update DB
	db[target][0] = newValue
	var delta [4]uint64
	delta[0] = oldValue ^ newValue

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
	db[target][0] = oldValue // Reset DB
	c = NewClient(n, m, keyAlpha, keyBeta)
	c.randSource = newDeterministicRandSource()
	c.HintInit(getStream())
	c.InitBackupHints(50, 10, getStream())

	// Now update
	db[target][0] = newValue
	delta[0] = oldValue ^ newValue
	c.UpdateHint(target, delta)

	// Query again (should use primary hint if not used, or backup)
	// Since we created a NEW client, primary hint is unused.
	// It should reflect the update.
	indices2, hint2, ok := c.Query(target)
	if !ok {
		t.Fatalf("Post-update query failed: no hint found")
	}

	// Compute parity 2 (using NEW DB)
	var parity2 [4]uint64
	for _, idx := range indices2 {
		parity2[0] ^= db[idx][0]
	}
	val2 := c.Reconstruct(parity2, hint2)
	if val2[0] != newValue {
		t.Errorf("Post-update value wrong: got %d, want %d", val2[0], newValue)
	}

	// 4. Verify Backup Hint Update
	// Consume primary hint
	// (hint2 was primary and is now used)

	// Query again -> should use backup hint
	indices3, hint3, ok := c.Query(target)
	if !ok {
		t.Fatalf("Backup query failed: no hint found")
	}

	// Verify it's backup
	if len(hint3.Indices) == 0 {
		t.Errorf("Expected backup hint")
	}

	// Compute parity 3
	var parity3 [4]uint64
	for _, idx := range indices3 {
		parity3[0] ^= db[idx][0]
	}
	val3 := c.Reconstruct(parity3, hint3)
	if val3[0] != newValue {
		t.Errorf("Backup hint value wrong after update: got %d, want %d", val3[0], newValue)
	}
}

func TestBackupHintsSanity(t *testing.T) {
	n := uint64(100)
	db := make([][4]uint64, n)
	for i := uint64(0); i < n; i++ {
		db[i][0] = uint64(i)
	}

	m := uint64(10)
	keyAlpha := make([]byte, 16)
	keyBeta := make([]byte, 16)

	c := NewClient(n, m, keyAlpha, keyBeta)
	c.randSource = newDeterministicRandSource()

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
	c.InitBackupHints(20, 10, getStream())

	for _, hint := range c.backupHints {
		seen := make(map[uint64]bool)
		for _, idx := range hint.Indices {
			if idx >= n {
				t.Errorf("backup hint index out of range: got %d, n=%d", idx, n)
			}
			if seen[idx] {
				t.Errorf("duplicate index %d within a single backup hint", idx)
			}
			seen[idx] = true
		}
	}
}
