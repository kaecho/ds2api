package config

import "strings"

const (
	ScheduleRoundRobin = "round_robin"
	ScheduleFill       = "fill"
	ScheduleLeastUsed  = "least_used"
	ScheduleRandom     = "random"
)

// NormalizeAccountSchedule maps aliases to a canonical schedule name.
// Empty input becomes round_robin. Unknown values stay empty so validation can reject them.
func NormalizeAccountSchedule(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ScheduleRoundRobin, "rr", "round-robin", "roundrobin":
		return ScheduleRoundRobin
	case ScheduleFill, "sticky", "fill_sticky", "fill-sticky":
		return ScheduleFill
	case ScheduleLeastUsed, "least-used", "leastused":
		return ScheduleLeastUsed
	case ScheduleRandom:
		return ScheduleRandom
	default:
		return ""
	}
}

func ValidAccountSchedule(raw string) bool {
	return NormalizeAccountSchedule(raw) != "" || strings.TrimSpace(raw) == ""
}
