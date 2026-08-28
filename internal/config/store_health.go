package config

import (
	"strings"
	"time"
)

const mutedUntilUnknown int64 = -1

// UpdateAccountMuteUntil records a temporary chat mute.
// until > 0 is a unix deadline, until == -1 means muted with no known end,
// until == 0 clears the mute.
func (s *Store) UpdateAccountMuteUntil(identifier string, until int64) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accMuteUntil == nil {
		s.accMuteUntil = map[string]int64{}
	}
	if until == 0 {
		delete(s.accMuteUntil, identifier)
		return
	}
	s.accMuteUntil[identifier] = until
}

// AccountMuteUntil returns the remaining mute deadline in unix seconds.
// 0 means not muted. Expired deadlines are cleared.
func (s *Store) AccountMuteUntil(identifier string) int64 {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accountMuteUntilLocked(identifier, time.Now().Unix())
}

// AccountMuted reports whether the account is currently paused for a chat mute.
func (s *Store) AccountMuted(identifier string) bool {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	until := s.accountMuteUntilLocked(identifier, time.Now().Unix())
	return until != 0
}

// ClearExpiredMutes drops mute deadlines that have passed and returns those identifiers.
// Unknown-end mutes (until == -1) are left in place until a later health check.
func (s *Store) ClearExpiredMutes(now int64) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.accMuteUntil) == 0 {
		return nil
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	var expired []string
	for id, until := range s.accMuteUntil {
		if until > 0 && until <= now {
			delete(s.accMuteUntil, id)
			expired = append(expired, id)
		}
	}
	return expired
}

func (s *Store) accountMuteUntilLocked(identifier string, now int64) int64 {
	until := s.accMuteUntil[identifier]
	if until == 0 {
		return 0
	}
	if until == mutedUntilUnknown {
		return mutedUntilUnknown
	}
	if until <= now {
		delete(s.accMuteUntil, identifier)
		return 0
	}
	return until
}
