package accounts

import (
	"strings"

	"ds2api/internal/config"
	adminshared "ds2api/internal/httpapi/admin/shared"
)

const (
	defaultAccountJobConcurrency = 10
	maxAccountJobConcurrency     = 32
)

func accountJobConcurrency(v any) int {
	n := adminshared.IntFrom(v)
	if n <= 0 {
		return defaultAccountJobConcurrency
	}
	if n > maxAccountJobConcurrency {
		return maxAccountJobConcurrency
	}
	return n
}

func accountJobIdentifiers(v any) []string {
	ids, ok := adminshared.ToStringSlice(v)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (h *Handler) accountsForJob(identifiers []string) []config.Account {
	all := h.Store.Snapshot().Accounts
	if len(identifiers) == 0 {
		return all
	}
	byID := make(map[string]config.Account, len(all))
	for _, acc := range all {
		if id := acc.Identifier(); id != "" {
			byID[id] = acc
		}
	}
	out := make([]config.Account, 0, len(identifiers))
	for _, id := range identifiers {
		if acc, ok := byID[id]; ok {
			out = append(out, acc)
			continue
		}
		if acc, ok := findAccountByIdentifier(h.Store, id); ok {
			out = append(out, acc)
		}
	}
	return out
}
