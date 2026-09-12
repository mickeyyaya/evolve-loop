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
	var c CycleInputs // zero value must be safe and empty
	if c.Goal() != "" || c.Strategy() != "" || c.CommitMessage() != "" || c.FleetScope() != "" || c.ChallengeToken() != "" || c.PreviousVerdict() != "" || c.Carryover() != "" {
		t.Fatalf("zero CycleInputs not empty: %+v", c)
	}
}

// ---------------------------------------------------------------------------
// Cycle 1632 RED contract (task tokenopt-handoff-digests, retry of the
// cycle-1593 attempt): the two unbounded free-text fields that cross from the
// legacy Context map into the typed handoff (Carryover / PreviousVerdict) get
// ONE single-sourced bounded representation at DTO construction:
//
//	phaseio.MaxFieldBytes    — exported positive cap (bytes)
//	phaseio.TruncationMarker — exported non-empty visible marker
//	phaseio.CapField(s)      — s unchanged when len(s) <= MaxFieldBytes, else a
//	                           rune-aligned prefix of s + TruncationMarker with
//	                           len(result) <= MaxFieldBytes
//
// NewCycleInputs applies CapField to Carryover and PreviousVerdict. These tests
// are frozen (doNotModifyTests) — Builder implements, never edits.
// ---------------------------------------------------------------------------

// overCap returns a valid-UTF-8 string of exactly MaxFieldBytes+extra bytes
// built from the given rune, so the cap boundary lands mid-rune for multibyte
// input.
func overCap(r rune, extra int) string {
	n := (MaxFieldBytes + extra + len(string(r)) - 1) / len(string(r))
	return strings.Repeat(string(r), n)
}

// assertBounded is the shared over-cap contract: bounded, marked, valid UTF-8,
// and the retained text is a genuine prefix of the input (not a rewrite).
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
	bigVerdict := overCap('v', 218480-MaxFieldBytes) // the cycle-1593 measured size
	c := NewCycleInputs(CycleInputsInit{Carryover: bigCarry, PreviousVerdict: bigVerdict})

	assertBounded(t, "Carryover", bigCarry, c.Carryover())
	assertBounded(t, "PreviousVerdict", bigVerdict, c.PreviousVerdict())
	// Single source: the DTO stores exactly what the exported helper produces,
	// so a comparator that runs the legacy value through CapField agrees by
	// construction (the cycle-1593 round-2 H1 false-mismatch class).
	if c.Carryover() != CapField(bigCarry) {
		t.Errorf("Carryover() != CapField(input): DTO and helper diverged")
	}
	if c.PreviousVerdict() != CapField(bigVerdict) {
		t.Errorf("PreviousVerdict() != CapField(input): DTO and helper diverged")
	}
	// Negative: a silent prefix cut is NOT an acceptable cap — the reader must
	// be able to see that text was dropped.
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
		{"multibyte-exact", strings.Repeat("é", MaxFieldBytes/2)}, // 2-byte rune, lands exactly on the cap
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
			// One extra byte so the naive byte cut at MaxFieldBytes splits a rune.
			in := overCap(tc.r, 1)
			got := NewCycleInputs(CycleInputsInit{Carryover: in}).Carryover()
			assertBounded(t, tc.name, in, got)
			idx := strings.Index(got, TruncationMarker)
			if idx < 0 {
				return // already reported by assertBounded
			}
			// The retained prefix must be rune-aligned: every rune before the
			// marker decodes as the input rune, never a replacement char.
			for _, rr := range got[:idx] {
				if rr != tc.r {
					t.Fatalf("retained prefix contains rune %U, want only %U — the cap split a rune", rr, tc.r)
				}
			}
		})
	}
}

func TestCapField_IsIdempotent(t *testing.T) {
	// Idempotence is what lets the shadow comparator cap the legacy value and
	// compare it to the already-capped typed value without a second marker or
	// a shorter re-cut.
	for _, in := range []string{"", "short", overCap('i', 1), overCap('😀', 3)} {
		once := CapField(in)
		if twice := CapField(once); twice != once {
			t.Errorf("CapField not idempotent for len=%d: once=%d bytes twice=%d bytes", len(in), len(once), len(twice))
		}
	}
}
