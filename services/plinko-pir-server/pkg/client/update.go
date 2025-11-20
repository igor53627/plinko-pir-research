package client

// UpdateHint updates the client's hints based on a database change
// index: the database index that changed
// delta: the XOR difference (oldValue ^ newValue)
// This is the O(1) update mechanism enabled by iPRF.
func (c *Client) UpdateHint(index uint64, delta [4]uint64) {
	// 1. Update Primary Hint
	// Use iPRF Forward to find which primary hint contains 'index'
	// F(k, index) -> j
	// So index is in block P_j.
	// We update Hint[j].

	j := c.iprf.F(index)

	if j < uint64(len(c.hints)) {
		// Update parity
		for k := 0; k < 4; k++ {
			c.hints[j].Parity[k] ^= delta[k]
		}

		// Note: If the hint was already used, updating it doesn't make it usable again
		// But if we haven't used it yet, we MUST update it so it's correct when we do use it.
	}

	// 2. Update Backup Hints
	for i := range c.backupHints {
		// Check if index is in this backup hint
		for _, idx := range c.backupHints[i].Indices {
			if idx == index {
				for k := 0; k < 4; k++ {
					c.backupHints[i].Parity[k] ^= delta[k]
				}
				break
			}
		}
	}
}
