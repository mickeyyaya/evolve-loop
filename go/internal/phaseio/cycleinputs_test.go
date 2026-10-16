package phaseio

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCycleInputs_Getters_ReturnConstructedValues(t *testing.T) {
	c := NewCycleInputs(CycleInputsInit{
		Goal:            "reduce dispatch latency",
		Strategy:        "profile-first",
		CommitMessage:   "perf(core): cache HEAD",
		FleetScope:      "core,bridge",
		ChallengeToken:  "tok-123",
		PreviousVerdict: "FAIL",
		Carryover:       "carried: tighten the digest fallback",
	})

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Goal", c.Goal(), "reduce dispatch latency"},
		{"Strategy", c.Strategy(), "profile-first"},
		{"CommitMessage", c.CommitMessage(), "perf(core): cache HEAD"},
		{"FleetScope", c.FleetScope(), "core,bridge"},
		{"ChallengeToken", c.ChallengeToken(), "tok-123"},
		{"PreviousVerdict", c.PreviousVerdict(), "FAIL"},
		{"Carryover", c.Carryover(), "carried: tighten the digest fallback"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s() = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestCycleInputs_Zero_AllGettersEmpty(t *testing.T) {
	var c CycleInputs
	if c.Goal() != "" || c.Strategy() != "" || c.CommitMessage() != "" || c.FleetScope() != "" || c.ChallengeToken() != "" || c.PreviousVerdict() != "" || c.Carryover() != "" {
		t.Fatalf("zero CycleInputs not empty: %+v", c)
	}
}

func overCap(r rune, extra int) string {
	n := (MaxFieldBytes + extra + len(string(r)) - 1) / len(string(r))
	return strings.Repeat(string(r), n)
}

func assertBounded(t *testing.T, label, in, got string) {
	t.Helper()
	if len(got) > MaxFieldBytes {
		t.Errorf("%s: len(CapField)=%d > MaxFieldBytes=%d", label, len(got), MaxFieldBytes)
	}
	if !strings.Contains(got, TruncationMarker) {
		t.Errorf("%s: capped value carries no visible TruncationMarker %q", label, TruncationMarker)
	}
	if !utf8.ValidString(got) {
		t.Errorf("%s: capped value is not valid UTF-8 (rune split at the cap boundary)", label)
	}
	if got == in {
		t.Errorf("%s: over-cap input passed through unchanged (cap inert)", label)
	}
	if i := strings.Index(got, TruncationMarker); i >= 0 && !strings.HasPrefix(in, got[:i]) {
		t.Errorf("%s: retained text %q is not a prefix of the input", label, got[:i])
	}
}

func TestNewCycleInputs_CapsOversizedFreeTextFields(t *testing.T) {
	if MaxFieldBytes <= 0 || MaxFieldBytes > 64*1024 {
		t.Fatalf("MaxFieldBytes=%d, want 0 < cap <= 64KiB (cycle-1593 measured a 218,480-byte carryover; a cap above that is no cap)", MaxFieldBytes)
	}
	if TruncationMarker == "" || len(TruncationMarker) >= MaxFieldBytes || !utf8.ValidString(TruncationMarker) {
		t.Fatalf("TruncationMarker=%q must be non-empty valid UTF-8 and shorter than the cap", TruncationMarker)
	}
	bigCarry := overCap('c', 1)
	bigVerdict := overCap('v', 218480-MaxFieldBytes)
	c := NewCycleInputs(CycleInputsInit{Carryover: bigCarry, PreviousVerdict: bigVerdict})

	assertBounded(t, "Carryover", bigCarry, c.Carryover())
	assertBounded(t, "PreviousVerdict", bigVerdict, c.PreviousVerdict())
	if c.Carryover() != CapField(bigCarry) {
		t.Errorf("Carryover() != CapField(input): DTO and helper diverged")
	}
	if c.PreviousVerdict() != CapField(bigVerdict) {
		t.Errorf("PreviousVerdict() != CapField(input): DTO and helper diverged")
	}
	if c.Carryover() == bigCarry[:MaxFieldBytes] {
		t.Errorf("Carryover() is a silent prefix cut with no marker")
	}
}

func TestNewCycleInputs_AtOrUnderCapRoundTripsUnchanged(t *testing.T) {
	exact := strings.Repeat("e", MaxFieldBytes)
	under := strings.Repeat("u", MaxFieldBytes-1)
	cases := []struct{ name, in string }{
		{"empty", ""},
		{"under", under},
		{"exact-boundary", exact},
		{"multibyte-exact", strings.Repeat("é", MaxFieldBytes/2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCycleInputs(CycleInputsInit{Carryover: tc.in, PreviousVerdict: tc.in})
			if c.Carryover() != tc.in || c.PreviousVerdict() != tc.in {
				t.Errorf("at/under cap must round-trip byte-identical: carryover=%d previous_verdict=%d want=%d bytes", len(c.Carryover()), len(c.PreviousVerdict()), len(tc.in))
			}
			if CapField(tc.in) != tc.in {
				t.Errorf("CapField altered an at/under-cap value (len=%d)", len(tc.in))
			}
			if tc.in != "" && strings.Contains(c.Carryover(), TruncationMarker) {
				t.Errorf("marker leaked into an at/under-cap value")
			}
		})
	}
}

func TestNewCycleInputs_CapsMultibyteInputAtRuneBoundary(t *testing.T) {
	cases := []struct {
		name string
		r    rune
	}{
		{"2-byte", 'é'},
		{"3-byte", '日'},
		{"4-byte", '😀'},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := overCap(tc.r, 1)
			got := NewCycleInputs(CycleInputsInit{Carryover: in}).Carryover()
			assertBounded(t, tc.name, in, got)
			idx := strings.Index(got, TruncationMarker)
			if idx < 0 {
				return
			}
			for _, rr := range got[:idx] {
				if rr != tc.r {
					t.Fatalf("retained prefix contains rune %U, want only %U — the cap split a rune", rr, tc.r)
				}
			}
		})
	}
}

func TestCapField_IsIdempotent(t *testing.T) {
	for _, in := range []string{"", "short", overCap('i', 1), overCap('😀', 3)} {
		once := CapField(in)
		if twice := CapField(once); twice != once {
			t.Errorf("CapField not idempotent for len=%d: once=%d bytes twice=%d bytes", len(in), len(once), len(twice))
		}
	}
}
