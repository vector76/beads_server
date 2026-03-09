package store

// StatusMap returns a map of bead ID → status string for the provided IDs.
// IDs not found in the store are absent from the result map.
// Deduplication of input is the caller's responsibility.
func (s *Store) StatusMap(ids []string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for _, id := range ids {
		if b, ok := s.beads[id]; ok {
			result[id] = string(b.Status)
		}
	}
	return result
}
