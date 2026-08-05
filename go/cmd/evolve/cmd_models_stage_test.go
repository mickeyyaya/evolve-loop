package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func stageNow() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }

func cannedCatalog(fetchedAt time.Time, cli, deepModel string) modelcatalog.Catalog {
	return modelcatalog.Catalog{
		FetchedAt: fetchedAt,
		CLIs: map[string]modelcatalog.CLIEntry{
			cli: {TierModels: map[string]string{"deep": deepModel}, Source: modelcatalog.SourceLive},
		},
	}
}

// TestRunStagedCatalogRefresh_OffIsNoOp: stage off never invokes the pipeline
// and touches no file.
func TestRunStagedCatalogRefresh_OffIsNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	calls := 0
	err := runStagedCatalogRefresh(context.Background(), dir, "off", func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		calls++
		return modelcatalog.Catalog{}, nil
	}, stageNow, os.Stderr)
	if err != nil || calls != 0 {
		t.Fatalf("off: err=%v calls=%d, want nil/0", err, calls)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("off wrote files: %v", entries)
	}
}

// TestRunStagedCatalogRefresh_ShadowWritesShadowOnly: the shadow stage runs
// the full pipeline but lands ONLY in model-catalog.shadow.json, logs the
// per-tier would-change diff against the live catalog, and never touches the
// live file — dispatch stays byte-identical to off.
func TestRunStagedCatalogRefresh_ShadowWritesShadowOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := modelcatalog.Write(dir, cannedCatalog(stageNow().Add(-time.Hour), "codex", "gpt-5.5")); err != nil {
		t.Fatal(err)
	}
	liveRaw, err := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
	if err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	rerr := runStagedCatalogRefresh(context.Background(), dir, "shadow", func(_ context.Context, prior modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		if !prior.Empty() {
			t.Errorf("first shadow run should see an EMPTY prior (no shadow file yet), got %#v", prior)
		}
		return cannedCatalog(stageNow(), "codex", "gpt-5.6"), nil
	}, stageNow, &log)
	if rerr != nil {
		t.Fatalf("shadow refresh: %v", rerr)
	}
	shadow, err := modelcatalog.ReadShadow(dir)
	if err != nil || shadow.CLIs["codex"].TierModels["deep"] != "gpt-5.6" {
		t.Fatalf("shadow catalog = %#v (err=%v), want the refreshed entry", shadow, err)
	}
	after, err := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
	if err != nil || !bytes.Equal(liveRaw, after) {
		t.Error("shadow stage modified the LIVE catalog file")
	}
	if !strings.Contains(log.String(), "would-change codex.deep: gpt-5.5 -> gpt-5.6") {
		t.Errorf("missing would-change line; log:\n%s", log.String())
	}
}

// TestRunStagedCatalogRefresh_ShadowTTLGatesOnShadowFile: a fresh shadow file
// suppresses the run even when the live catalog is stale (gating on the live
// file would drive the expensive probe every cycle), and a prior shadow file
// is what the pipeline receives as Prior (that is where its reuse
// fingerprints live).
func TestRunStagedCatalogRefresh_ShadowTTLGatesOnShadowFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Live catalog long stale; shadow catalog fresh.
	if err := modelcatalog.Write(dir, cannedCatalog(stageNow().Add(-72*time.Hour), "codex", "gpt-5.5")); err != nil {
		t.Fatal(err)
	}
	if err := modelcatalog.WriteShadow(dir, cannedCatalog(stageNow().Add(-time.Hour), "codex", "gpt-5.6")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := runStagedCatalogRefresh(context.Background(), dir, "shadow", func(_ context.Context, prior modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		calls++
		return prior, nil
	}, stageNow, os.Stderr)
	if err != nil || calls != 0 {
		t.Fatalf("fresh shadow should gate the run: err=%v calls=%d", err, calls)
	}
	// Stale shadow → runs, and Prior IS the shadow catalog.
	if err := modelcatalog.WriteShadow(dir, cannedCatalog(stageNow().Add(-48*time.Hour), "codex", "gpt-5.6")); err != nil {
		t.Fatal(err)
	}
	err = runStagedCatalogRefresh(context.Background(), dir, "shadow", func(_ context.Context, prior modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		calls++
		if prior.CLIs["codex"].TierModels["deep"] != "gpt-5.6" {
			t.Errorf("Prior should be the SHADOW catalog, got %#v", prior)
		}
		return cannedCatalog(stageNow(), "codex", "gpt-5.6"), nil
	}, stageNow, os.Stderr)
	if err != nil || calls != 1 {
		t.Fatalf("stale shadow should run once: err=%v calls=%d", err, calls)
	}
}

// TestRunStagedCatalogRefresh_EnforceCommitsLive: enforce lands in the live
// catalog through the Commit seam (operator tier_fallbacks carried forward —
// the property Commit's own tests pin).
func TestRunStagedCatalogRefresh_EnforceCommitsLive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := runStagedCatalogRefresh(context.Background(), dir, "enforce", func(_ context.Context, prior modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		return cannedCatalog(stageNow(), "codex", "gpt-5.6"), nil
	}, stageNow, os.Stderr)
	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	live, err := modelcatalog.Read(dir)
	if err != nil || live.CLIs["codex"].TierModels["deep"] != "gpt-5.6" {
		t.Fatalf("live catalog = %#v (err=%v), want the committed entry", live, err)
	}
}

// TestShadowDiffLines_DeterministicAndComplete: sorted CLI × canonical-tier
// order, absent sides render "(none)", unchanged tiers are silent.
func TestShadowDiffLines_DeterministicAndComplete(t *testing.T) {
	t.Parallel()
	live := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {TierModels: map[string]string{"deep": "gpt-5.5", "fast": "gpt-5.5-mini"}},
	}}
	next := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {TierModels: map[string]string{"deep": "gpt-5.6", "fast": "gpt-5.5-mini"}},
		"agy":   {TierModels: map[string]string{"deep": "Gemini 3.5 Pro (High)"}},
	}}
	got := shadowDiffLines(live, next)
	want := []string{
		"would-change agy.deep: (none) -> Gemini 3.5 Pro (High)",
		"would-change codex.deep: gpt-5.5 -> gpt-5.6",
	}
	if len(got) != len(want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFreshnessFromManifests_ManifestDataNotGoConditionals: claude's alias
// policy comes from its manifest block; enumerating CLIs get no entry (zero
// policy); a CLI with no manifest is skipped silently.
func TestFreshnessFromManifests_ManifestDataNotGoConditionals(t *testing.T) {
	t.Parallel()
	got := freshnessFromManifests([]string{"claude", "codex", "ollama"})
	claude, ok := got["claude"]
	if !ok || !claude.PreferAlias || len(claude.AliasIDs) == 0 {
		t.Errorf("claude policy = %#v (ok=%v), want PreferAlias with alias ids", claude, ok)
	}
	if _, ok := got["codex"]; ok {
		t.Error("codex should carry no freshness entry (zero policy)")
	}
	if _, ok := got["ollama"]; ok {
		t.Error("ollama (no manifest) should carry no freshness entry")
	}
	var zero modelquery.FreshnessPolicy
	if zero.PreferAlias {
		t.Error("zero policy must be the enumerating default")
	}
}
