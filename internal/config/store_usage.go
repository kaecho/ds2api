package config

import (
	"strings"
	"time"
)

func (s *Store) NoteAccountUse(identifier string) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDailyUsesLocked()
	s.accDailyUses[identifier]++
}

func (s *Store) AccountDailyUses(identifier string) int {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDailyUsesLocked()
	return s.accDailyUses[identifier]
}

func (s *Store) ensureDailyUsesLocked() {
	day := time.Now().Format("2006-01-02")
	if s.accDailyDate != day {
		s.accDailyDate = day
		s.accDailyUses = map[string]int{}
		return
	}
	if s.accDailyUses == nil {
		s.accDailyUses = map[string]int{}
	}
}
