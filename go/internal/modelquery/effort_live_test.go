//go:build integration

package modelquery

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestLive_HelpEffortLadders(t *testing.T) {
	for _, tc := range []struct {
		cli      string
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

func TestLive_DiscoveredLadderReachesTheCatalog(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skipf("claude not installed here: %v", err)
	}
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:          []string{"claude"},
		Lister:        fakeLister{ids: map[string][]string{"claude": {"haiku", "sonnet", "opus"}}},
		Classifier:    fakeClassifier{},
		Now:           fixedNow,
		EffortListers: DefaultEffortListers(),
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
