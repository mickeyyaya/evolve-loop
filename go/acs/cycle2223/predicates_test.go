//go:build acs

package cycle2223

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type offering map[string][]string

func (o offering) List(_ context.Context, cli string) ([]string, error) { return o[cli], nil }

type callRecorder struct{ clis []string }

func (c *callRecorder) Classify(_ context.Context, cli string, ids []string) (map[string]string, error) {
	c.clis = append(c.clis, cli)
	return map[string]string{"fast": ids[0], "balanced": ids[len(ids)/2], "deep": ids[len(ids)-1], "top": ids[len(ids)-1]}, nil
}

func stamp() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }

func refresh(t *testing.T, clis []string, live offering, prior modelcatalog.Catalog) (modelcatalog.Catalog, *callRecorder) {
	t.Helper()
	rec := &callRecorder{}
	cat, err := modelquery.Refresh(context.Background(), modelquery.RefreshDeps{
		CLIs: clis, Lister: live, Classifier: rec, Prior: prior, Now: stamp,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat, rec
}

func liveEntry(cli string, tiers map[string]string, available []string) modelcatalog.CLIEntry {
	return modelcatalog.CLIEntry{
		TierModels: tiers, Available: available, Source: modelcatalog.SourceLive,
		CandidatesHash: modelquery.Fingerprint(modelquery.FingerprintInput{
			CLI: cli, Candidates: available, Tiers: modelcatalog.CanonicalTiers,
		}),
	}
}

func codexPick(model string) map[string]string {
	return map[string]string{"fast": "gpt-5.5-mini", "balanced": model, "deep": model, "top": model}
}

func TestC2223_001_ConsecutiveVersionBumpsInOneLineageMakeNoClassifierCall(t *testing.T) {
	offers := [][]string{
		{"gpt-5.5-mini", "gpt-5.5"},
		{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"},
		{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6", "gpt-5.7"},
	}
	prior := modelcatalog.Catalog{FetchedAt: stamp(), CLIs: map[string]modelcatalog.CLIEntry{
		"codex": liveEntry("codex", codexPick("gpt-5.5"), offers[0]),
	}}
	for step, ids := range offers[1:] {
		newest := ids[len(ids)-1]
		cat, rec := refresh(t, []string{"codex"}, offering{"codex": ids}, prior)
		if len(rec.clis) != 0 {
			t.Fatalf("bump step %d: classifier called %d times, want 0", step+1, len(rec.clis))
		}
		got := cat.CLIs["codex"].TierModels
		for _, tier := range []string{"balanced", "deep", "top"} {
			if got[tier] != newest {
				t.Errorf("bump step %d: tier %s = %q, want newest %q", step+1, tier, got[tier], newest)
			}
		}
		if got["fast"] != "gpt-5.5-mini" {
			t.Errorf("bump step %d: fast = %q, want the untouched gpt-5.5-mini", step+1, got["fast"])
		}
		prior = cat
	}
}

func TestC2223_002_NewLineageCallsClassifierExactlyOnceAndKeepsItsVerdict(t *testing.T) {
	before := []string{"gpt-5.5-mini", "gpt-5.5"}
	prior := modelcatalog.Catalog{FetchedAt: stamp(), CLIs: map[string]modelcatalog.CLIEntry{
		"codex": liveEntry("codex", codexPick("gpt-5.5"), before),
	}}
	withNewLineage := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6", "o4-mini"}
	cat, rec := refresh(t, []string{"codex"}, offering{"codex": withNewLineage}, prior)
	if len(rec.clis) != 1 || rec.clis[0] != "codex" {
		t.Fatalf("classifier calls = %v, want exactly one for codex", rec.clis)
	}
	if got := cat.CLIs["codex"].CandidatesHash; got == prior.CLIs["codex"].CandidatesHash {
		t.Errorf("CandidatesHash unchanged after a new lineage; the classification was not restamped")
	}
}

func TestC2223_003_OnlyTheCLIWithANewLineageIsClassified(t *testing.T) {
	codexBefore := []string{"gpt-5.5-mini", "gpt-5.5"}
	geminiBefore := []string{"gemini-3-flash", "gemini-3-pro"}
	prior := modelcatalog.Catalog{FetchedAt: stamp(), CLIs: map[string]modelcatalog.CLIEntry{
		"codex": liveEntry("codex", codexPick("gpt-5.5"), codexBefore),
		"gemini": liveEntry("gemini", map[string]string{
			"fast": "gemini-3-flash", "balanced": "gemini-3-pro", "deep": "gemini-3-pro", "top": "gemini-3-pro",
		}, geminiBefore),
	}}
	live := offering{
		"codex":  {"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"},
		"gemini": {"gemini-3-flash", "gemini-3-pro", "gemini-embed"},
	}
	_, rec := refresh(t, []string{"codex", "gemini"}, live, prior)
	if len(rec.clis) != 1 || rec.clis[0] != "gemini" {
		t.Fatalf("classifier calls = %v, want only gemini (codex merely bumped a version)", rec.clis)
	}
}

func TestC2223_004_UnsoundOrMissingPriorNeverCarriesAcrossABump(t *testing.T) {
	before := []string{"gpt-5.5-mini", "gpt-5.5"}
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	detected := liveEntry("codex", codexPick("gpt-5.5"), before)
	detected.Source = modelcatalog.SourceDetect
	cases := map[string]modelcatalog.Catalog{
		"detect-sourced prior": {CLIs: map[string]modelcatalog.CLIEntry{"codex": detected}},
		"no prior at all":      {},
	}
	for name, prior := range cases {
		_, rec := refresh(t, []string{"codex"}, offering{"codex": bumped}, prior)
		if len(rec.clis) != 1 {
			t.Errorf("%s: classifier calls = %d, want 1", name, len(rec.clis))
		}
	}
}

func TestC2223_005_EveryFunctionOfQueryGoIsFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	modDir, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatalf("ModuleDir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, "./internal/modelquery")
	test.Dir = modDir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./internal/modelquery failed: %v\n%s", err, acsassert.OutputTail(string(out), 25))
	}
	cover := exec.Command("go", "tool", "cover", "-func="+profile)
	cover.Dir = modDir
	out, err := cover.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, acsassert.OutputTail(string(out), 25))
	}
	covered := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "modelquery/query.go:") {
			continue
		}
		covered[fields[1]] = true
		if pct := fields[len(fields)-1]; pct != "100.0%" {
			t.Errorf("%s coverage = %s, want 100.0%%", fields[1], pct)
		}
	}
	for _, fn := range []string{"liveTiers", "carryAcrossBump", "sameLineages", "priorDerivedFromItsOffering", "isSoundPrior"} {
		if !covered[fn] {
			t.Errorf("cover output lists no function %s of query.go", fn)
		}
	}
}
