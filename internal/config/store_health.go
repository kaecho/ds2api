package config

import (
	"errors"
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
	s.applyMuteUntilLocked(identifier, until)
	s.persistHealthLocked()
}

// UpdateAccountHealth writes banned, mute, and test status in one persist.
// Empty testStatus leaves the current test status unchanged.
func (s *Store) UpdateAccountHealth(identifier string, banned bool, muteUntil int64, testStatus string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return errors.New("account not found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.findAccountIndexLocked(identifier)
	if !ok {
		return errors.New("account not found")
	}
	s.applyBannedLocked(identifier, banned)
	s.applyMuteUntilLocked(identifier, muteUntil)
	if strings.TrimSpace(testStatus) != "" {
		s.setAccountTestStatusLocked(s.cfg.Accounts[idx], testStatus, identifier)
	}
	return s.saveLocked()
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
			s.applyMuteUntilLocked(id, 0)
			s.clearMutedTestStatusLocked(id)
			expired = append(expired, id)
		}
	}
	if len(expired) > 0 {
		s.persistHealthLocked()
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
		s.applyMuteUntilLocked(identifier, 0)
		s.clearMutedTestStatusLocked(identifier)
		return 0
	}
	return until
}

func (s *Store) applyBannedLocked(identifier string, banned bool) {
	if s.accBanned == nil {
		s.accBanned = map[string]bool{}
	}
	if banned {
		s.accBanned[identifier] = true
	} else {
		delete(s.accBanned, identifier)
	}
	if idx, ok := s.findAccountIndexLocked(identifier); ok {
		s.cfg.Accounts[idx].Banned = banned
	}
}

func (s *Store) applyMuteUntilLocked(identifier string, until int64) {
	if s.accMuteUntil == nil {
		s.accMuteUntil = map[string]int64{}
	}
	if until == 0 {
		delete(s.accMuteUntil, identifier)
	} else {
		s.accMuteUntil[identifier] = until
	}
	if idx, ok := s.findAccountIndexLocked(identifier); ok {
		s.cfg.Accounts[idx].MuteUntil = until
	}
}

func (s *Store) clearMutedTestStatusLocked(identifier string) {
	idx, ok := s.findAccountIndexLocked(identifier)
	if !ok {
		return
	}
	if lower(s.cfg.Accounts[idx].TestStatus) != "muted" {
		return
	}
	s.cfg.Accounts[idx].TestStatus = ""
	acc := s.cfg.Accounts[idx]
	if id := acc.Identifier(); id != "" {
		delete(s.accTest, id)
	}
	if email := acc.Email; email != "" {
		delete(s.accTest, email)
	}
	if mobile := CanonicalMobileKey(acc.Mobile); mobile != "" {
		delete(s.accTest, mobile)
	}
	delete(s.accTest, identifier)
}

func (s *Store) persistHealthLocked() {
	if err := s.saveLocked(); err != nil {
		Logger.Warn("[config] persist account health failed", "error", err)
	}
}
