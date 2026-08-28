package config

import "testing"

func TestNormalizeAccountSchedule(t *testing.T) {
	cases := map[string]string{
		"":            ScheduleRoundRobin,
		"round_robin": ScheduleRoundRobin,
		"RR":          ScheduleRoundRobin,
		"fill":        ScheduleFill,
		"sticky":      ScheduleFill,
		"least_used":  ScheduleLeastUsed,
		"least-used":  ScheduleLeastUsed,
		"random":      ScheduleRandom,
		"nope":        "",
	}
	for in, want := range cases {
		if got := NormalizeAccountSchedule(in); got != want {
			t.Fatalf("NormalizeAccountSchedule(%q)=%q want %q", in, got, want)
		}
	}
}

func TestValidateRuntimeConfigRejectsUnknownSchedule(t *testing.T) {
	err := ValidateRuntimeConfig(RuntimeConfig{AccountSchedule: "burst"})
	if err == nil {
		t.Fatal("expected invalid schedule error")
	}
}

func TestRuntimeAccountScheduleDefaultsToRoundRobin(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"],"accounts":[{"email":"u@example.com","password":"p"}]}`)
	store := LoadStore()
	if got := store.RuntimeAccountSchedule(); got != ScheduleRoundRobin {
		t.Fatalf("schedule=%q", got)
	}
	if got := store.RuntimeAccountDailyLimit(); got != 0 {
		t.Fatalf("daily limit=%d", got)
	}
}
