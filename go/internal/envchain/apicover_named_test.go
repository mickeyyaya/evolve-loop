package envchain

import "testing"

// apicover_named_test.go exercises the env-var KEY names, DEFAULT values, and
// the per-phase-attempts CAP that the typed resolvers in OTHER packages read
// from this registry:
//
//   - resolvePhaseMaxAttempts (internal/core/retry_backoff.go) reads
//     KeyPhaseMaxAttempts via IntMin(.., DefPhaseMaxAttempts, 1) then caps at
//     MaxPhaseMaxAttempts.
//   - resolveRetryBackoffBase (internal/core/retry_backoff.go) reads
//     KeyRetryBackoffBaseS via Int(.., DefRetryBackoffBaseS) then floors a
//     negative at 0.
//   - cyclehealth's phase-latency check (internal/cyclehealth/cyclehealth.go)
//     reads KeyPhaseLatencyCeilingS via IntMin(.., DefPhaseLatencyCeilingS, 1).
//
// envchain cannot import those consumer packages (they import envchain), so
// each test below re-uses the SAME getter the production resolver calls
// (envchain.IntMin / envchain.Int) wired to the REAL const so the resolver
// contract is reproduced byte-for-byte rather than value-pinned. t.Setenv
// drives the live process-env tier the resolvers consult (env=nil reqEnv).

// resolvePhaseMaxAttempts mirrors core.resolvePhaseMaxAttempts exactly, driving
// the real KeyPhaseMaxAttempts / DefPhaseMaxAttempts / MaxPhaseMaxAttempts
// through the same IntMin getter the production resolver uses.
func resolvePhaseMaxAttempts() int {
	n := IntMin(KeyPhaseMaxAttempts, nil, DefPhaseMaxAttempts, 1)
	if n > MaxPhaseMaxAttempts {
		return MaxPhaseMaxAttempts
	}
	return n
}

func TestPhaseMaxAttemptsResolver(t *testing.T) {
	if KeyPhaseMaxAttempts != "EVOLVE_PHASE_MAX_ATTEMPTS" {
		t.Fatalf("KeyPhaseMaxAttempts = %q", KeyPhaseMaxAttempts)
	}
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset-returns-default", false, "", DefPhaseMaxAttempts}, // 2
		{"below-min-returns-default", true, "0", DefPhaseMaxAttempts},
		{"unparseable-returns-default", true, "x", DefPhaseMaxAttempts},
		{"at-min", true, "1", 1},
		{"valid-in-range", true, "3", 3},
		{"over-cap-clamps-to-max", true, "9", MaxPhaseMaxAttempts}, // 5
		{"exactly-at-cap", true, "5", MaxPhaseMaxAttempts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv(KeyPhaseMaxAttempts, c.val)
			} else {
				t.Setenv(KeyPhaseMaxAttempts, "")
			}
			if got := resolvePhaseMaxAttempts(); got != c.want {
				t.Errorf("resolvePhaseMaxAttempts()=%d, want %d", got, c.want)
			}
		})
	}
}

// resolveRetryBackoffBase mirrors core.resolveRetryBackoffBase exactly, driving
// the real KeyRetryBackoffBaseS / DefRetryBackoffBaseS through the same Int
// getter the production resolver uses.
func resolveRetryBackoffBase() int {
	n := Int(KeyRetryBackoffBaseS, nil, DefRetryBackoffBaseS)
	if n < 0 {
		return 0
	}
	return n
}

func TestRetryBackoffBaseResolver(t *testing.T) {
	if KeyRetryBackoffBaseS != "EVOLVE_RETRY_BACKOFF_BASE_S" {
		t.Fatalf("KeyRetryBackoffBaseS = %q", KeyRetryBackoffBaseS)
	}
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset-returns-default", false, "", DefRetryBackoffBaseS}, // 5
		{"empty-returns-default", true, "", DefRetryBackoffBaseS},
		{"unparseable-returns-default", true, "abc", DefRetryBackoffBaseS},
		{"valid-value", true, "12", 12},
		{"zero-disabled", true, "0", 0},
		{"negative-floored-to-zero", true, "-3", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv(KeyRetryBackoffBaseS, c.val)
			} else {
				t.Setenv(KeyRetryBackoffBaseS, "")
			}
			if got := resolveRetryBackoffBase(); got != c.want {
				t.Errorf("resolveRetryBackoffBase()=%d, want %d", got, c.want)
			}
		})
	}
}

// resolvePhaseLatencyCeiling mirrors the cyclehealth phase-latency read
// exactly, driving the real KeyPhaseLatencyCeilingS / DefPhaseLatencyCeilingS
// through the same IntMin(.., 1) getter cyclehealth uses.
func resolvePhaseLatencyCeiling() int {
	return IntMin(KeyPhaseLatencyCeilingS, nil, DefPhaseLatencyCeilingS, 1)
}

func TestPhaseLatencyCeilingResolver(t *testing.T) {
	if KeyPhaseLatencyCeilingS != "EVOLVE_PHASE_LATENCY_CEILING_S" {
		t.Fatalf("KeyPhaseLatencyCeilingS = %q", KeyPhaseLatencyCeilingS)
	}
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset-returns-default", false, "", DefPhaseLatencyCeilingS}, // 900
		{"below-min-returns-default", true, "0", DefPhaseLatencyCeilingS},
		{"negative-returns-default", true, "-1", DefPhaseLatencyCeilingS},
		{"unparseable-returns-default", true, "soon", DefPhaseLatencyCeilingS},
		{"at-min", true, "1", 1},
		{"valid-override", true, "120", 120},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv(KeyPhaseLatencyCeilingS, c.val)
			} else {
				t.Setenv(KeyPhaseLatencyCeilingS, "")
			}
			if got := resolvePhaseLatencyCeiling(); got != c.want {
				t.Errorf("resolvePhaseLatencyCeiling()=%d, want %d", got, c.want)
			}
		})
	}
}
