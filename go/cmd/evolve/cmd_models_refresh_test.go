package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

const codexLaunchReason = "classify models: every classifier CLI failed (codex): bridge: launch exit=1"

func refreshResult(entries map[string]modelcatalog.CLIEntry) modelcatalog.Catalog {
	return modelcatalog.Catalog{FetchedAt: stageNow(), CLIs: entries}
}

func detectFallback(model, reason string) modelcatalog.CLIEntry {
	return modelcatalog.CLIEntry{TierModels: map[string]string{"balanced": model}, Source: modelcatalog.SourceDetect, FallbackReason: reason}
}

func seedPriorCatalog(t *testing.T) (string, time.Time) {
	t.Helper()
	dir := t.TempDir()
	fetched := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	prior := modelcatalog.Catalog{FetchedAt: fetched, CLIs: map[string]modelcatalog.CLIEntry{
		"agy": {TierModels: map[string]string{"balanced": "Gemini 3.7 Flash (High)"}, Source: modelcatalog.SourceLive},
	}}
	if err := modelcatalog.Write(dir, prior); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return dir, fetched
}

func TestFinishLiveRefresh_NoLiveCLIExitsNonZeroAndKeepsThePriorCatalog(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		dir, priorFetched := seedPriorCatalog(t)
		cat := refreshResult(map[string]modelcatalog.CLIEntry{
			"agy":   detectFallback("Gemini 3.7 Flash (High)", codexLaunchReason),
			"codex": detectFallback("gpt-5.6-terra", "list models: picker did not open"),
		})
		var out, errb bytes.Buffer

		code := finishLiveRefresh(modelsOpts{EvolveDir: dir, AsJSON: asJSON}, cat, &out, &errb)

		if code == 0 {
			t.Fatalf("json=%v: exit 0 when no ready CLI was classified live", asJSON)
		}
		if !strings.Contains(errb.String(), "no ready CLI was classified live") {
			t.Errorf("json=%v: stderr does not say why it failed: %q", asJSON, errb.String())
		}
		for _, want := range []string{"exit=1", "picker did not open"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("json=%v: report missing the per-CLI reason %q: %q", asJSON, want, out.String())
			}
		}
		onDisk, err := modelcatalog.Read(dir)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if !onDisk.FetchedAt.Equal(priorFetched) || onDisk.CLIs["agy"].Source != modelcatalog.SourceLive {
			t.Errorf("json=%v: a refresh with no live CLI overwrote the prior catalog: %+v", asJSON, onDisk)
		}
	}
}

func TestFinishLiveRefresh_PartialFallbackCommitsAndNamesEachFallback(t *testing.T) {
	dir, _ := seedPriorCatalog(t)
	cat := refreshResult(map[string]modelcatalog.CLIEntry{
		"agy":   {TierModels: map[string]string{"balanced": "Gemini 3.8 Flash (High)"}, Source: modelcatalog.SourceLive},
		"codex": detectFallback("gpt-5.6-terra", "list models: picker did not open"),
	})
	var out, errb bytes.Buffer

	code := finishLiveRefresh(modelsOpts{EvolveDir: dir}, cat, &out, &errb)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q, want 0 when at least one CLI classified live", code, errb.String())
	}
	if !strings.Contains(out.String(), "1 live, 1 detect fallback") {
		t.Errorf("header must count live vs fallback entries: %q", out.String())
	}
	if strings.Contains(out.String(), "live /model (detect fallback)") {
		t.Errorf("header still reads as a plain success with a generic fallback label: %q", out.String())
	}
	if !strings.Contains(out.String(), "source: detect fallback") || !strings.Contains(out.String(), "picker did not open") {
		t.Errorf("codex row must say it is a detect fallback and why: %q", out.String())
	}
	if !strings.Contains(out.String(), "source: live") {
		t.Errorf("agy row must say it is live: %q", out.String())
	}
	onDisk, err := modelcatalog.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := onDisk.CLIs["agy"].TierModels["balanced"]; got != "Gemini 3.8 Flash (High)" {
		t.Errorf("committed agy balanced = %q, want the live 3.8 pick", got)
	}
}

func TestFinishLiveRefresh_JSONCarriesTheFallbackReason(t *testing.T) {
	dir, _ := seedPriorCatalog(t)
	cat := refreshResult(map[string]modelcatalog.CLIEntry{
		"agy":   {TierModels: map[string]string{"balanced": "Gemini 3.8 Flash (High)"}, Source: modelcatalog.SourceLive},
		"codex": detectFallback("gpt-5.6-terra", "list models: picker did not open"),
	})
	var out, errb bytes.Buffer

	code := finishLiveRefresh(modelsOpts{EvolveDir: dir, AsJSON: true}, cat, &out, &errb)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), `"fallback_reason": "list models: picker did not open"`) {
		t.Errorf("--json must carry each fallback's reason: %q", out.String())
	}
}
