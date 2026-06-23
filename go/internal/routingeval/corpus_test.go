package routingeval

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// WS4-S1 (ADR-0052): the corpus loads + validates. A malformed case file is a
// LOUD error, never a skipped case.
func TestGoldenRoutingCorpus_LoadsAndValidates(t *testing.T) {
	t.Parallel()
	cases, err := LoadCorpus("testdata")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(cases) < 6 {
		t.Fatalf("want >=6 seed cases, got %d", len(cases))
	}
	// Name-sorted for deterministic subtest order.
	if !sort.SliceIsSorted(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name }) {
		t.Error("LoadCorpus must return name-sorted cases")
	}
	// Name the Case type directly (public-API coverage) and assert the realized
	// fields are populated.
	var first Case = cases[0]
	if first.Name == "" || first.RawResponse == "" {
		t.Errorf("first realized Case is malformed: %+v", first)
	}
	for _, c := range cases {
		if c.Name == "" {
			t.Error("case with empty name")
		}
		if c.RawResponse == "" {
			t.Errorf("case %q: empty raw response", c.Name)
		}
		// A positive case must assert something (run-set or forbidden); a
		// negative case must declare the parse error. Never a no-op case.
		if !c.ExpectParseError && len(c.ExpectRunSet) == 0 && len(c.ForbiddenPhases) == 0 {
			t.Errorf("case %q asserts nothing — a no-op corpus case locks nothing", c.Name)
		}
	}

	// Malformed loader inputs must be loud errors.
	t.Run("bad json is loud", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cd := filepath.Join(dir, "broken")
		mustMkdir(t, cd)
		mustWrite(t, filepath.Join(cd, "response.txt"), "[]")
		mustWrite(t, filepath.Join(cd, "case.json"), "{ not json")
		if _, err := LoadCorpus(dir); err == nil {
			t.Error("malformed case.json must be a loud error")
		}
	})
	t.Run("unknown key is loud", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cd := filepath.Join(dir, "typo")
		mustMkdir(t, cd)
		mustWrite(t, filepath.Join(cd, "response.txt"), "[]")
		mustWrite(t, filepath.Join(cd, "case.json"), `{"expect_runset":["x"]}`) // typo'd key
		if _, err := LoadCorpus(dir); err == nil {
			t.Error("a typo'd expectation key must be a loud error (DisallowUnknownFields)")
		}
	})
	t.Run("missing response is loud", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cd := filepath.Join(dir, "noresp")
		mustMkdir(t, cd)
		mustWrite(t, filepath.Join(cd, "case.json"), `{"expect_parse_error":true}`)
		if _, err := LoadCorpus(dir); err == nil {
			t.Error("a case missing response.txt must be a loud error")
		}
	})
	t.Run("empty corpus is loud", func(t *testing.T) {
		t.Parallel()
		if _, err := LoadCorpus(t.TempDir()); err == nil {
			t.Error("an empty corpus must be a loud error (a no-op corpus locks nothing)")
		}
	})
}

// WS4-S2 (ADR-0052): replay each captured response through the REAL parse +
// integrity-floor clamp (core.ReplayPlanFromResponse — the WS3-S5 entry point)
// and assert the run-set, the clamps that fire, the phases that must never run,
// and the universal ship⇒audit safety invariant. This locks the parse + floor:
// a regression that weakened either would change a run-set and break a case.
func TestGoldenCorpus_ReplayProducesExpectedRunSet(t *testing.T) {
	t.Parallel()
	cases, err := LoadCorpus("testdata")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			floor := c.Floor
			if len(floor) == 0 {
				floor = router.DefaultShipFloor()
			}
			plan, clamps, err := core.ReplayPlanFromResponse(c.RawResponse, c.Input, floor)
			if c.ExpectParseError {
				if err == nil {
					t.Fatalf("expected a parse error, got plan %+v", plan)
				}
				return
			}
			if err != nil {
				t.Fatalf("replay: %v", err)
			}

			if got := runSetOf(plan); !equalStrings(got, c.ExpectRunSet) {
				t.Errorf("run-set = %v, want %v", got, c.ExpectRunSet)
			}
			fired := clampRules(clamps)
			for _, want := range c.ExpectClamps {
				if !contains(fired, want) {
					t.Errorf("expected clamp %q to fire; clamps fired = %v", want, fired)
				}
			}
			for _, forbidden := range c.ForbiddenPhases {
				if phaseRuns(plan, forbidden) {
					t.Errorf("forbidden phase %q must NOT run; plan = %+v", forbidden, plan.Entries)
				}
			}
			// Universal safety invariant (intent, not characterization): a plan
			// that reaches ship MUST run audit — the floor's whole purpose.
			if phaseRuns(plan, "ship") && !phaseRuns(plan, "audit") {
				t.Errorf("ship without audit survived the clamp — floor violated; plan = %+v", plan.Entries)
			}
		})
	}
}

func runSetOf(plan *router.PhasePlan) []string {
	var out []string
	for _, e := range plan.Entries {
		if e.Run {
			out = append(out, e.Phase)
		}
	}
	sort.Strings(out)
	return out
}

func phaseRuns(plan *router.PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

func clampRules(clamps []router.Clamp) []string {
	out := make([]string, 0, len(clamps))
	for _, c := range clamps {
		out = append(out, c.Rule)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
