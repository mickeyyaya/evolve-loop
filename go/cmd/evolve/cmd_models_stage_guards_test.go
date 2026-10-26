package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func detectOnly(fetchedAt time.Time, cli, deepModel string) modelcatalog.Catalog {
	cat := cannedCatalog(fetchedAt, cli, deepModel)
	e := cat.CLIs[cli]
	e.Source = modelcatalog.SourceDetect
	cat.CLIs[cli] = e
	return cat
}

func TestRunStagedCatalogRefresh_ZeroLiveClassificationsWriteNeitherCatalog(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		dir := t.TempDir()
		if err := modelcatalog.Write(dir, cannedCatalog(stageNow().Add(-48*time.Hour), "agy", "gemini-3.8-flash")); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
		if err != nil {
			t.Fatal(err)
		}
		var log bytes.Buffer

		rerr := runStagedCatalogRefresh(context.Background(), dir, stage, func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
			return detectOnly(stageNow(), "agy", "gemini-3.5-flash"), nil
		}, stageNow, &log)

		if rerr != nil {
			t.Fatalf("%s: %v", stage, rerr)
		}
		after, err := os.ReadFile(filepath.Join(dir, modelcatalog.FileName))
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("%s: a refresh with no live classification changed the live catalog", stage)
		}
		if shadow, _ := modelcatalog.ReadShadow(dir); !shadow.FetchedAt.IsZero() {
			t.Errorf("%s: a refresh with no live classification wrote the shadow catalog", stage)
		}
		if !strings.Contains(log.String(), "no ready CLI was classified live") {
			t.Errorf("%s: log = %q, want the WARN line", stage, log.String())
		}
	}
}

func TestRunStagedCatalogRefresh_EnforceKeepsThePriorLivePickOfAFamilyWhoseListFailed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prior := cannedCatalog(stageNow().Add(-48*time.Hour), "agy", "gemini-3.8-flash")
	prior.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.4"}, Source: modelcatalog.SourceLive}
	if err := modelcatalog.Write(dir, prior); err != nil {
		t.Fatal(err)
	}
	fresh := detectOnly(stageNow(), "agy", "gemini-3.5-flash")
	fresh.CLIs["claude"] = modelcatalog.CLIEntry{TierModels: map[string]string{"deep": "opus-5.5"}, Source: modelcatalog.SourceLive}
	var log bytes.Buffer

	err := runStagedCatalogRefresh(context.Background(), dir, "enforce", func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
		return fresh, nil
	}, stageNow, &log)

	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	live, err := modelcatalog.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := live.CLIs["agy"]; got.Source != modelcatalog.SourceLive || got.TierModels["deep"] != "gemini-3.8-flash" {
		t.Errorf("agy = %+v, want the prior live pick gemini-3.8-flash", got)
	}
	if got := live.CLIs["claude"].TierModels["deep"]; got != "opus-5.5" {
		t.Errorf("claude deep = %q, want the new live pick opus-5.5", got)
	}
	if !strings.Contains(log.String(), "catalog_refresh_degraded agy") {
		t.Errorf("log = %q, want the degraded line for agy", log.String())
	}
}

func TestRunStagedCatalogRefresh_AnUnreadableCatalogIsRefreshedAndARefreshErrorIsReturned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage, file, warn string
	}{
		{"enforce", modelcatalog.FileName, "unreadable catalog"},
		{"shadow", modelcatalog.ShadowFileName, "unreadable shadow catalog"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, tc.file), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		var log bytes.Buffer
		boom := errors.New("list failed")

		err := runStagedCatalogRefresh(context.Background(), dir, tc.stage, func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error) {
			return modelcatalog.Catalog{}, boom
		}, stageNow, &log)

		if !errors.Is(err, boom) {
			t.Errorf("%s: err = %v, want the refresh error", tc.stage, err)
		}
		if !strings.Contains(log.String(), tc.warn) {
			t.Errorf("%s: log = %q, want %q", tc.stage, log.String(), tc.warn)
		}
	}
}
