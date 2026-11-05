package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func noEmit(signalcenter.Event) {}

func collectEmit() (signalcenter.Listener, *[]signalcenter.Event) {
	var got []signalcenter.Event
	return func(e signalcenter.Event) { got = append(got, e) }, &got
}

func degradedFamilies(events []signalcenter.Event) []string {
	var families []string
	for _, e := range events {
		if e.Code == modelcatalog.CodeRefreshDegraded {
			families = append(families, e.Fields["cli"])
		}
	}
	slices.Sort(families)
	return families
}

func priorWithThreeLiveFamilies() modelcatalog.Catalog {
	prior := cannedCatalog(stageNow().Add(-48*time.Hour), "agy", "gemini-3.8-flash")
	prior.CLIs["codex"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "gpt-5.5"}, Source: modelcatalog.SourceLive}
	prior.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.4"}, Source: modelcatalog.SourceLive}
	return prior
}

func TestCatalogDegraded_EnforceEmitsOneEventPerFamilyWhoseListFailed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := modelcatalog.Write(dir, priorWithThreeLiveFamilies()); err != nil {
		t.Fatal(err)
	}
	fresh := detectOnly(stageNow(), "agy", "gemini-3.5-flash")
	codexDetect := modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "gpt-5.1"}, Source: modelcatalog.SourceDetect, FallbackReason: "codex list timed out"}
	fresh.CLIs["codex"] = codexDetect
	fresh.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.5"}, Source: modelcatalog.SourceLive}
	emit, events := collectEmit()
	var log bytes.Buffer

	err := runStagedCatalogRefresh(context.Background(), dir, "enforce", func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		return fresh, nil
	}, stageNow, &log, emit)

	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if got := degradedFamilies(*events); !slices.Equal(got, []string{"agy", "codex"}) {
		t.Fatalf("degraded events name %v, want exactly agy and codex (claude listed live): %+v", got, *events)
	}
	for _, e := range *events {
		if e.Kind != signalcenter.KindConfigWarning || e.Severity != signalcenter.SeverityWarn || e.Origin != "commitKeepingLivePicks" {
			t.Errorf("event = %+v, want a config.warning WARN from commitKeepingLivePicks", e)
		}
		if e.Fields["cli"] == "codex" && e.Fields["fallback_reason"] != "codex list timed out" {
			t.Errorf("codex event fallback_reason = %q, want the reason the fresh entry carried", e.Fields["fallback_reason"])
		}
	}
	if !strings.Contains(log.String(), "catalog_refresh_degraded agy") || !strings.Contains(log.String(), "catalog_refresh_degraded codex") {
		t.Errorf("log = %q, want the WARN line to stay beside the event", log.String())
	}
}

func TestCatalogDegraded_NoEventWhenEveryFamilyListedLive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := modelcatalog.Write(dir, cannedCatalog(stageNow().Add(-48*time.Hour), "agy", "gemini-3.8-flash")); err != nil {
		t.Fatal(err)
	}
	emit, events := collectEmit()

	err := runStagedCatalogRefresh(context.Background(), dir, "enforce", func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		return cannedCatalog(stageNow(), "agy", "gemini-3.9-flash"), nil
	}, stageNow, &bytes.Buffer{}, emit)

	if err != nil || len(*events) != 0 {
		t.Fatalf("err=%v events=%+v, want a clean commit with no degraded event", err, *events)
	}
}

func TestCatalogDegraded_NoEventForAFamilyThatNeverHadALivePick(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prior := detectOnly(stageNow().Add(-48*time.Hour), "agy", "gemini-3.5-flash")
	prior.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.4"}, Source: modelcatalog.SourceLive}
	if err := modelcatalog.Write(dir, prior); err != nil {
		t.Fatal(err)
	}
	fresh := detectOnly(stageNow(), "agy", "gemini-3.5-flash")
	fresh.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.5"}, Source: modelcatalog.SourceLive}
	emit, events := collectEmit()

	err := runStagedCatalogRefresh(context.Background(), dir, "enforce", func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		return fresh, nil
	}, stageNow, &bytes.Buffer{}, emit)

	if err != nil || len(*events) != 0 {
		t.Fatalf("err=%v events=%+v, want no event: agy had no live pick to keep, so nothing degraded", err, *events)
	}
}

func TestCatalogDegraded_ShadowAndZeroLiveEmitNothingAndWriteNoLiveCatalog(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage string
		fresh modelcatalog.Catalog
	}{
		{"shadow", detectOnly(stageNow(), "agy", "gemini-3.5-flash")},
		{"enforce", detectOnly(stageNow(), "agy", "gemini-3.5-flash")},
	} {
		dir := t.TempDir()
		if err := modelcatalog.Write(dir, cannedCatalog(stageNow().Add(-48*time.Hour), "agy", "gemini-3.8-flash")); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
		if err != nil {
			t.Fatal(err)
		}
		emit, events := collectEmit()

		rerr := runStagedCatalogRefresh(context.Background(), dir, tc.stage, func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
			return tc.fresh, nil
		}, stageNow, &bytes.Buffer{}, emit)

		after, _ := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
		if rerr != nil || len(*events) != 0 || !bytes.Equal(before, after) {
			t.Errorf("%s: err=%v events=%+v live catalog unchanged=%v, want no event and an untouched live catalog", tc.stage, rerr, *events, bytes.Equal(before, after))
		}
	}
}

func TestCatalogDegraded_ZeroLiveKeepsFetchedAtSoTheNextCycleStartRetries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	staleAt := stageNow().Add(-48 * time.Hour)
	if err := modelcatalog.Write(dir, cannedCatalog(staleAt, "agy", "gemini-3.8-flash")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	refresh := func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		calls++
		return detectOnly(stageNow(), "agy", "gemini-3.5-flash"), nil
	}

	for i := 0; i < 2; i++ {
		if err := runStagedCatalogRefresh(context.Background(), dir, "enforce", refresh, stageNow, &bytes.Buffer{}, noEmit); err != nil {
			t.Fatal(err)
		}
	}

	got, err := modelcatalog.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.FetchedAt.Equal(staleAt) || calls != 2 {
		t.Errorf("FetchedAt = %v (want %v), refresh calls = %d (want 2): a refresh with no live classification must stay stale so the next start tries again", got.FetchedAt, staleAt, calls)
	}
}

func TestCatalogDegraded_AFreshCatalogIsNotRefreshedAtEitherStage(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		dir := t.TempDir()
		fresh := cannedCatalog(stageNow().Add(-time.Hour), "agy", "gemini-3.8-flash")
		if err := modelcatalog.Write(dir, fresh); err != nil {
			t.Fatal(err)
		}
		if err := modelcatalog.WriteShadow(dir, fresh); err != nil {
			t.Fatal(err)
		}
		calls := 0

		err := runStagedCatalogRefresh(context.Background(), dir, stage, func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
			calls++
			return modelcatalog.Catalog{}, nil
		}, stageNow, &bytes.Buffer{}, noEmit)

		if err != nil || calls != 0 {
			t.Errorf("%s: err=%v calls=%d, want the TTL to stop a refresh of a one-hour-old catalog", stage, err, calls)
		}
	}
}

func TestCatalogDegraded_CountLiveAndFallbackReadsThePredicateOfModelcatalog(t *testing.T) {
	t.Parallel()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"agy":    {Source: modelcatalog.SourceLive},
		"claude": {Source: modelcatalog.SourceDetect},
		"codex":  {Source: modelcatalog.SourceLive},
	}}

	live, fallback := countLiveAndFallback(cat)

	if live != cat.LiveCount() || live != 2 || fallback != 1 {
		t.Errorf("live=%d fallback=%d, want 2 and 1 from the one modelcatalog predicate", live, fallback)
	}
}

func TestMakeCatalogRefresher_AStaleCatalogWithNoReadyCLIReturnsTheDetectErrorAndLeavesTheCatalog(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staleAt := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
	if err := modelcatalog.Write(evolveDir, cannedCatalog(staleAt, "agy", "gemini-3.8-flash")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	emit, events := collectEmit()

	err := makeCatalogRefresher(root, evolveDir, "enforce", emit)(context.Background())

	if err == nil || !strings.Contains(err.Error(), "no ready CLIs") {
		t.Fatalf("err = %v, want the no-ready-CLI refusal of liveRefresh", err)
	}
	got, rerr := modelcatalog.Read(evolveDir)
	if rerr != nil || !got.FetchedAt.Equal(staleAt) || got.CLIs["agy"].TierModels["deep"] != "gemini-3.8-flash" || len(*events) != 0 {
		t.Errorf("catalog after a failed refresh = %+v (err %v), events = %+v, want it untouched", got, rerr, *events)
	}
}
