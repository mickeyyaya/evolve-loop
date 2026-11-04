//go:build acs

package cycle2147

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
	return map[string]string{"fast": ids[0], "balanced": ids[len(ids)/2], "deep": ids[len(ids)-1]}, nil
}

func stamp() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }

func priorCodex(source string, tiers map[string]string, available []string) modelcatalog.Catalog {
	return modelcatalog.Catalog{
		FetchedAt: stamp(),
		CLIs: map[string]modelcatalog.CLIEntry{"codex": {
			TierModels: tiers, Available: available, Source: source,
			CandidatesHash: modelquery.Fingerprint(modelquery.FingerprintInput{
				CLI: "codex", Candidates: available, Tiers: modelcatalog.CanonicalTiers,
			}),
		}},
	}
}

func livePickBeforeBump() map[string]string {
	return map[string]string{"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5", "top": "gpt-5.5"}
}

func refreshCodex(t *testing.T, ids []string, prior modelcatalog.Catalog) (modelcatalog.Catalog, *callRecorder) {
	t.Helper()
	rec := &callRecorder{}
	cat, err := modelquery.Refresh(context.Background(), modelquery.RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     offering{"codex": ids},
		Classifier: rec,
		Prior:      prior,
		Now:        stamp,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat, rec
}

func TestC2147_001_VersionBumpWithinLineageMakesNoClassifierCall(t *testing.T) {
	prior := priorCodex(modelcatalog.SourceLive, livePickBeforeBump(), []string{"gpt-5.5-mini", "gpt-5.5"})
	cat, rec := refreshCodex(t, []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}, prior)
	if len(rec.clis) != 0 {
		t.Fatalf("classifier called %d times on a version bump inside one lineage, want 0", len(rec.clis))
	}
	entry := cat.CLIs["codex"]
	for tier, want := range map[string]string{"fast": "gpt-5.5-mini", "balanced": "gpt-5.6", "deep": "gpt-5.6", "top": "gpt-5.6"} {
		if entry.TierModels[tier] != want {
			t.Errorf("tier %s = %q, want %q", tier, entry.TierModels[tier], want)
		}
	}
	if entry.Source != modelcatalog.SourceLive {
		t.Errorf("Source = %q, want live", entry.Source)
	}
	_, again := refreshCodex(t, []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}, cat)
	if len(again.clis) != 0 {
		t.Errorf("refresh after the bump called the classifier %d times, want 0", len(again.clis))
	}
}

func TestC2147_002_ReplacedVersionPromotesItsSuccessorWithoutClassifier(t *testing.T) {
	prior := priorCodex(modelcatalog.SourceLive, livePickBeforeBump(), []string{"gpt-5.5-mini", "gpt-5.5"})
	cat, rec := refreshCodex(t, []string{"gpt-5.5-mini", "gpt-5.6"}, prior)
	if len(rec.clis) != 0 {
		t.Fatalf("classifier called %d times when gpt-5.6 replaced gpt-5.5, want 0", len(rec.clis))
	}
	if got := cat.CLIs["codex"].TierModels["deep"]; got != "gpt-5.6" {
		t.Errorf("deep = %q, want the successor gpt-5.6", got)
	}
}

func TestC2147_003_NewLineageStillCallsClassifier(t *testing.T) {
	prior := priorCodex(modelcatalog.SourceLive, livePickBeforeBump(), []string{"gpt-5.5-mini", "gpt-5.5"})
	cases := []struct {
		name string
		ids  []string
	}{
		{"a new lineage appears beside a bump", []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6", "o4-mini"}},
		{"a lineage vanishes", []string{"gpt-5.5", "gpt-5.6"}},
	}
	for _, c := range cases {
		_, rec := refreshCodex(t, c.ids, prior)
		if len(rec.clis) != 1 {
			t.Errorf("%s: classifier calls = %d, want 1", c.name, len(rec.clis))
		}
	}
}

func TestC2147_004_UnsoundPriorIsNeverReusedAcrossABump(t *testing.T) {
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	before := []string{"gpt-5.5-mini", "gpt-5.5"}
	topless := map[string]string{"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5"}
	cases := []struct {
		name  string
		prior modelcatalog.Catalog
	}{
		{"detect-sourced prior", priorCodex(modelcatalog.SourceDetect, livePickBeforeBump(), before)},
		{"prior without top tier", priorCodex(modelcatalog.SourceLive, topless, before)},
		{"prior with no recorded offering", priorCodex(modelcatalog.SourceLive, livePickBeforeBump(), nil)},
	}
	for _, c := range cases {
		_, rec := refreshCodex(t, bumped, c.prior)
		if len(rec.clis) != 1 {
			t.Errorf("%s: classifier calls = %d, want 1", c.name, len(rec.clis))
		}
	}
}

func refreshCodexUnderPolicy(t *testing.T, ids []string, prior modelcatalog.Catalog, p modelquery.FreshnessPolicy) (modelcatalog.Catalog, *callRecorder) {
	t.Helper()
	rec := &callRecorder{}
	cat, err := modelquery.Refresh(context.Background(), modelquery.RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     offering{"codex": ids},
		Classifier: rec,
		Prior:      prior,
		Freshness:  map[string]modelquery.FreshnessPolicy{"codex": p},
		Now:        stamp,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat, rec
}

func priorCodexUnderPolicy(p modelquery.FreshnessPolicy, hashOverride string) modelcatalog.Catalog {
	available := []string{"gpt-5.5-mini", "gpt-5.5"}
	prior := priorCodex(modelcatalog.SourceLive, livePickBeforeBump(), available)
	entry := prior.CLIs["codex"]
	entry.CandidatesHash = hashOverride
	if hashOverride == "derive" {
		entry.CandidatesHash = modelquery.Fingerprint(modelquery.FingerprintInput{
			CLI: "codex", Candidates: available, Policy: p, Tiers: modelcatalog.CanonicalTiers,
		})
	}
	prior.CLIs["codex"] = entry
	return prior
}

func TestC2147_006_FreshnessPolicyChangeAcrossABumpStillCallsClassifier(t *testing.T) {
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	changed := modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"gpt-5.6"}}
	prior := priorCodexUnderPolicy(modelquery.FreshnessPolicy{}, "derive")
	cat, rec := refreshCodexUnderPolicy(t, bumped, prior, changed)
	if len(rec.clis) != 1 {
		t.Fatalf("classifier calls = %d, want 1: a policy change invalidates the prior decision", len(rec.clis))
	}
	wantHash := modelquery.Fingerprint(modelquery.FingerprintInput{
		CLI: "codex", Candidates: bumped, Policy: changed, Tiers: modelcatalog.CanonicalTiers,
	})
	if got := cat.CLIs["codex"].CandidatesHash; got != wantHash {
		t.Errorf("CandidatesHash = %q, want the fingerprint under the new policy", got)
	}
}

func TestC2147_007_PriorHashFromAnOlderDecisionVersionStillCallsClassifier(t *testing.T) {
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	for name, hash := range map[string]string{
		"hash written under an older decisionVersion": "sha256:hash-written-under-an-older-decision-version",
		"empty hash": "",
	} {
		_, rec := refreshCodexUnderPolicy(t, bumped, priorCodexUnderPolicy(modelquery.FreshnessPolicy{}, hash), modelquery.FreshnessPolicy{})
		if len(rec.clis) != 1 {
			t.Errorf("%s: classifier calls = %d, want 1", name, len(rec.clis))
		}
	}
}

func TestC2147_008_UnchangedNonDefaultPolicyStillCarriesAcrossABump(t *testing.T) {
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	policy := modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"gpt-5.6"}}
	cat, rec := refreshCodexUnderPolicy(t, bumped, priorCodexUnderPolicy(policy, "derive"), policy)
	if len(rec.clis) != 0 {
		t.Fatalf("classifier calls = %d, want 0 when only the candidates changed under an unchanged policy", len(rec.clis))
	}
	if got := cat.CLIs["codex"].TierModels["balanced"]; got != "gpt-5.6" {
		t.Errorf("balanced = %q, want gpt-5.6", got)
	}
}

func TestC2147_005_ChangedFunctionsOfQueryGoAreFullyCovered(t *testing.T) {
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
	checked := 0
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "modelquery/query.go:") {
			continue
		}
		checked++
		if pct := fields[len(fields)-1]; pct != "100.0%" {
			t.Errorf("%s coverage = %s, want 100.0%%", fields[1], pct)
		}
	}
	if checked == 0 {
		t.Fatalf("cover output lists no function of query.go:\n%s", acsassert.OutputTail(string(out), 10))
	}
	if pct, err := acsassert.FuncCoverage(string(out), "liveTiers"); err != nil || pct < 100 {
		t.Errorf("liveTiers coverage = %.1f (err %v), want 100", pct, err)
	}
}
