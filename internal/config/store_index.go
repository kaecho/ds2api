package config

import "strings"

// rebuildIndexes must be called with the lock already held (or during init).
func (s *Store) rebuildIndexes() {
	prevStatus := s.accTest
	prevBanned := s.accBanned
	prevMute := s.accMuteUntil
	prevUses := s.accDailyUses
	prevDay := s.accDailyDate
	s.keyMap = make(map[string]struct{}, len(s.cfg.Keys))
	for _, k := range s.cfg.Keys {
		s.keyMap[k] = struct{}{}
	}
	s.accMap = make(map[string]int, len(s.cfg.Accounts))
	s.accTest = make(map[string]string, len(s.cfg.Accounts))
	s.accBanned = make(map[string]bool, len(s.cfg.Accounts))
	s.accMuteUntil = make(map[string]int64, len(s.cfg.Accounts))
	s.accDailyDate = prevDay
	s.accDailyUses = make(map[string]int, len(s.cfg.Accounts))
	for i, acc := range s.cfg.Accounts {
		id := acc.Identifier()
		if id == "" {
			continue
		}
		s.accMap[id] = i
		if acc.TestStatus != "" {
			s.setAccountTestStatusLocked(acc, acc.TestStatus, "")
		}
		if acc.Banned {
			s.accBanned[id] = true
		}
		if acc.MuteUntil != 0 {
			s.accMuteUntil[id] = acc.MuteUntil
		}
		if status, ok := prevStatus[id]; ok {
			s.setAccountTestStatusLocked(acc, status, "")
		}
		if banned, ok := prevBanned[id]; ok && banned {
			s.accBanned[id] = true
		}
		if until, ok := prevMute[id]; ok && until != 0 {
			s.accMuteUntil[id] = until
		}
		if n, ok := prevUses[id]; ok && n > 0 {
			s.accDailyUses[id] = n
		}
	}
}

// findAccountIndexLocked expects the store lock to already be held.
func (s *Store) findAccountIndexLocked(identifier string) (int, bool) {
	if idx, ok := s.accMap[identifier]; ok && idx >= 0 && idx < len(s.cfg.Accounts) {
		return idx, true
	}
	// Fallback for token-only accounts whose derived identifier changed after
	// a token refresh; this preserves correctness on map misses.
	for i, acc := range s.cfg.Accounts {
		if acc.Identifier() == identifier {
			return i, true
		}
	}
	return -1, false
}

func (s *Store) setAccountTestStatusLocked(acc Account, status, hintedIdentifier string) {
	status = lower(status)
	if status == "" {
		return
	}
	id := acc.Identifier()
	if id == "" {
		id = strings.TrimSpace(hintedIdentifier)
	}
	if idx, ok := s.findAccountIndexLocked(id); ok {
		s.cfg.Accounts[idx].TestStatus = status
		acc = s.cfg.Accounts[idx]
	}
	if id := acc.Identifier(); id != "" {
		s.accTest[id] = status
	}
	if email := acc.Email; email != "" {
		s.accTest[email] = status
	}
	if mobile := CanonicalMobileKey(acc.Mobile); mobile != "" {
		s.accTest[mobile] = status
	}
	if hintedIdentifier = lower(hintedIdentifier); hintedIdentifier != "" {
		s.accTest[hintedIdentifier] = status
	}
}
