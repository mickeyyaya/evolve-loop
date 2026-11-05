package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRefreshCatalogAtLoopBoot_RunsTheRefresherOnce(t *testing.T) {
	t.Parallel()
	calls := 0

	refreshCatalogAtLoopBoot(context.Background(), func(context.Context) error {
		calls++
		return nil
	}, io.Discard)

	if calls != 1 {
		t.Errorf("refresher calls = %d, want 1", calls)
	}
}

func TestRefreshCatalogAtLoopBoot_AnErrorWarnsAndDoesNotAbortTheBoot(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	refreshCatalogAtLoopBoot(context.Background(), func(context.Context) error {
		return errors.New("no ready CLIs to query")
	}, &stderr)

	out := stderr.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "catalog") || !strings.Contains(out, "no ready CLIs to query") {
		t.Errorf("stderr = %q, want one WARN line that names the catalog refresh and the error", out)
	}
}

func TestRefreshCatalogAtLoopBoot_ANilRefresherIsSkipped(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	refreshCatalogAtLoopBoot(context.Background(), nil, &stderr)

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence when no refresher is wired", stderr.String())
	}
}

func TestRunLoopBatch_RefreshesTheCatalogOnceBeforeThePreflight(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var order []string
	prevDeps, prevPf := wireOrchestratorDepsFn, runLoopPreflightFn
	t.Cleanup(func() { wireOrchestratorDepsFn, runLoopPreflightFn = prevDeps, prevPf })
	c, _ := recordedCenter()
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{
			Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger(), Signals: c,
			CatalogRefresher: func(context.Context) error {
				order = append(order, "refresh")
				return errors.New("refresher failed")
			},
		}
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		order = append(order, "preflight")
		return forcedHalt()
	}
	cfg, rc := parseLoopArgs(haltedLoopArgs(projectRoot, evolveDir), io.Discard)
	if rc != 0 {
		t.Fatalf("parseLoopArgs rc = %d", rc)
	}
	cfg.announce = silentLoop

	exit := runLoopBatch(cfg, nil, io.Discard, io.Discard)

	if exit != 2 {
		t.Errorf("exit = %d, want the preflight halt (2): a refresher error must not change the boot outcome", exit)
	}
	if !slices.Equal(order, []string{"refresh", "preflight"}) {
		t.Errorf("order = %v, want the catalog refresh once, then the preflight", order)
	}
}

func TestWireOrchestratorDeps_TheCatalogRefresherIsWiredAndTheTTLStopsItOnAFreshCatalog(t *testing.T) {
	root, evolveDir := wiringRoot(t, `{"catalog":{"auto_refresh":true,"refresh_stage":"enforce"}}`)
	freshAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	if err := modelcatalog.Write(evolveDir, cannedCatalog(freshAt, "agy", "gemini-3.8-flash")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(evolveDir, modelcatalog.FileName))
	if err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})

	if d.RoutingErr != nil || d.CatalogRefresher == nil {
		t.Fatalf("RoutingErr = %v, CatalogRefresher nil = %v, want the composition root to wire the refresher the loop boot calls", d.RoutingErr, d.CatalogRefresher == nil)
	}
	t.Setenv("PATH", t.TempDir())
	if err := d.CatalogRefresher(context.Background()); err != nil {
		t.Fatalf("a one-hour-old catalog is inside the TTL, so the refresher must return nil, got %v", err)
	}
	after, err := os.ReadFile(filepath.Join(evolveDir, modelcatalog.FileName))
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("the live catalog changed on a TTL-gated refresh (read err %v)", err)
	}
}
