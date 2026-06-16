//go:build integration

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const DefaultBootBudget (looppreflight.go) — the per-driver REPL boot
//     deadline; asserted as the default resolve() applies when BootBudget<=0.
//   - var DefaultSpinePhases (looppreflight.go) — the always-dispatched spine
//     phases; asserted via its role as the fallback resolve() uses for an empty
//     SpinePhases, plus its documented membership.
//   - method CheckResult.MarshalJSON (result_json.go) — MUST be invoked by name
//     (json.Marshal alone names it only implicitly); we call c.MarshalJSON().
//   - method Result.MarshalJSON (result_json.go) — likewise invoked as
//     r.MarshalJSON() directly.
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
package looppreflight

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestDefaultBootBudget_IsResolveDefault pins DefaultBootBudget both as its
// documented 90s value and as the value resolve() substitutes when the caller
// leaves BootBudget unset (<=0).
func TestDefaultBootBudget_IsResolveDefault(t *testing.T) {
	if DefaultBootBudget != 90*time.Second {
		t.Fatalf("DefaultBootBudget = %v, want 90s", DefaultBootBudget)
	}
	o, err := resolve(Options{ProjectRoot: t.TempDir()}) // BootBudget unset → default.
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if o.bootBudget != DefaultBootBudget {
		t.Fatalf("resolve() bootBudget = %v, want DefaultBootBudget (%v)", o.bootBudget, DefaultBootBudget)
	}
}

// TestDefaultSpinePhases_FallbackAndMembership pins DefaultSpinePhases as the
// fallback resolve() uses for an empty SpinePhases, and asserts it contains the
// load-bearing spine phases a real cycle always dispatches.
func TestDefaultSpinePhases_FallbackAndMembership(t *testing.T) {
	for _, want := range []string{"build", "scout", "tdd", "audit", "intent", "triage"} {
		found := false
		for _, p := range DefaultSpinePhases {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("DefaultSpinePhases missing %q; got %v", want, DefaultSpinePhases)
		}
	}
	// resolve() with no SpinePhases must fall back to DefaultSpinePhases.
	o, err := resolve(Options{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(o.spinePhases) != len(DefaultSpinePhases) {
		t.Fatalf("resolve() spinePhases = %v, want DefaultSpinePhases (%v)", o.spinePhases, DefaultSpinePhases)
	}
	for i := range DefaultSpinePhases {
		if o.spinePhases[i] != DefaultSpinePhases[i] {
			t.Fatalf("resolve() spinePhases[%d] = %q, want %q", i, o.spinePhases[i], DefaultSpinePhases[i])
		}
	}
}

// TestCheckResultMarshalJSON_LevelAsString invokes CheckResult.MarshalJSON
// directly (by name) and asserts the int Level serializes as its lowercase
// string token and that an empty Detail is omitted.
func TestCheckResultMarshalJSON_LevelAsString(t *testing.T) {
	c := CheckResult{Name: "bridge-boot", Level: LevelHalt, Message: "1 driver failed"}
	b, err := c.MarshalJSON()
	if err != nil {
		t.Fatalf("CheckResult.MarshalJSON: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"level":"halt"`) {
		t.Fatalf("level must serialize as the string \"halt\"; got %s", s)
	}
	if !strings.Contains(s, `"name":"bridge-boot"`) {
		t.Fatalf("name missing; got %s", s)
	}
	if strings.Contains(s, "detail") {
		t.Fatalf("empty detail must be omitted; got %s", s)
	}
	// Round-trips through encoding/json identically to the direct call.
	via, _ := json.Marshal(c)
	if string(via) != s {
		t.Fatalf("json.Marshal(%v) = %s, want identical to MarshalJSON() = %s", c, via, s)
	}
}

// TestResultMarshalJSON_OverallLevelAsString invokes Result.MarshalJSON
// directly (by name) and asserts OverallLevel serializes as a string token and
// the wrapped CheckResults marshal via their own MarshalJSON (level as string).
func TestResultMarshalJSON_OverallLevelAsString(t *testing.T) {
	r := Result{
		Checks:       []CheckResult{{Name: "pipeline-structure", Level: LevelPass, Message: "ok"}},
		ChecksPassed: 1,
		ChecksTotal:  1,
		OverallLevel: LevelWarn,
		GeneratedAt:  "2026-05-23T12:00:00Z",
	}
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("Result.MarshalJSON: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"overall_level":"warn"`) {
		t.Fatalf("overall_level must serialize as the string \"warn\"; got %s", s)
	}
	if strings.Contains(s, `"overall_level":1`) {
		t.Fatalf("overall_level must not serialize as a raw int; got %s", s)
	}
	if !strings.Contains(s, `"level":"pass"`) {
		t.Fatalf("nested CheckResult level must serialize as a string via its MarshalJSON; got %s", s)
	}
}
