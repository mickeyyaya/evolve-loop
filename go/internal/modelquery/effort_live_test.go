//go:build integration

package modelquery

// effort_live_test.go — the guard that would have caught the original defect.
//
// Every unit test in effort_test.go asserts against help text CAPTURED at a
// point in time. That is exactly the weakness that let two live bugs sit
// undetected for weeks: a fixture proves the parser handles what the CLI USED
// to print, never what it prints today. The agy incident is the canonical
// case — three tests pinned "Gemini Flash 3.7 (High)" and stayed green for two
// weeks while agy rejected that very string at every launch.
//
// So this test asks the REAL binary. It runs under -tags integration and skips
// cleanly wherever the CLI is not installed, because "not installed" and
// "installed but no longer publishes its ladder" must never look alike.
//
// KNOW WHAT THIS DOES NOT COVER. CI provisions no agent CLIs, so on the
// runners every case here SKIPS — it reports SKIP, never PASS, and a ladder
// that changed upstream would not be caught there. These are an OPERATOR/dev
// guard, run where the CLIs actually live; treating a green CI as evidence
// they passed is the same "cited for more than it checks" error the rest of
// this work is about. The honest reading of a CI run is: unit tests covered
// the parser, nothing checked it against reality.

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestLive_HelpEffortLadders(t *testing.T) {
	for _, tc := range []struct {
		cli string
		// mustHave are rungs the CLI is known to publish. A rung DISAPPEARING
		// is as much a signal as one appearing: it means the dial we set is no
		// longer accepted, which realizeScalar would swallow.
		mustHave []string
	}{
		{"claude", []string{"low", "medium", "high", "xhigh", "max"}},
		{"agy", []string{"low", "medium", "high"}},
	} {
		t.Run(tc.cli, func(t *testing.T) {
			if _, err := exec.LookPath(tc.cli); err != nil {
				t.Skipf("%s not installed here: %v", tc.cli, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			got, err := (HelpEffortLister{}).ListEfforts(ctx, tc.cli)
			if err != nil {
				t.Fatalf("%s: ListEfforts against the real binary: %v", tc.cli, err)
			}
			if len(got) == 0 {
				t.Fatalf("%s: discovered an EMPTY ladder from live --help. Either the flag was renamed or its enum moved — both mean the effort dial we set is no longer verified", tc.cli)
			}
			for _, want := range tc.mustHave {
				if !slices.Contains(got, want) {
					t.Errorf("%s: live ladder %v is missing %q — a rung we rely on is gone; realizeScalar would drop it silently on a values-mapped manifest", tc.cli, got, want)
				}
			}
		})
	}
}

// codex publishes NO reasoning-effort enum in --help (verified live on
// 0.147.0; its rungs are reachable only through the /model picker's "More
// reasoning..." submenu). If that ever changes, help discovery becomes
// available for codex and DefaultEffortListers should gain it — this test
// fails to tell us so, rather than leaving codex on manifest-declared values
// forever out of habit.
func TestLive_CodexStillHidesItsLadderFromHelp(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skipf("codex not installed here: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := (HelpEffortLister{}).ListEfforts(ctx, "codex")
	if err != nil {
		t.Fatalf("codex --help: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("codex --help now publishes an effort enum %v — help discovery is available for it, so register it in DefaultEffortListers instead of relying on manifest-declared rungs", got)
	}
}

// End-to-end with the REAL registry and REAL CLIs, but a fake model lister so
// no classifier/LLM call and no quota is spent: `--help` is free.
//
// This is the wiring proof that matters. The unit tests fake the lister, and a
// fake can be perfect while production never calls it — which is precisely how
// the agy defect shipped (correct parser, wrong route). Here DefaultEffortListers
// is the real registry, HelpEffortLister shells the real binary, and the
// assertion is made on the CATALOG that Refresh returns.
func TestLive_DiscoveredLadderReachesTheCatalog(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skipf("claude not installed here: %v", err)
	}
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:          []string{"claude"},
		Lister:        fakeLister{ids: map[string][]string{"claude": {"haiku", "sonnet", "opus"}}},
		Classifier:    fakeClassifier{},
		Now:           fixedNow,
		EffortListers: DefaultEffortListers(), // the REAL registry
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	got := cat.CLIs["claude"].Efforts
	if len(got) == 0 {
		t.Fatal("the catalog recorded NO effort ladder for claude even though the live CLI publishes one — discovery is not reaching the catalog")
	}
	if !slices.Contains(got, "max") {
		t.Errorf("catalog ladder %v does not contain \"max\", which live `claude --help` advertises", got)
	}
}
