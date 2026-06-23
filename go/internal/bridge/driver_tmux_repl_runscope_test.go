// driver_tmux_repl_runscope_test.go — CB.5 contract (concurrency campaign
// W4), bridge half: tmux sessions are RUN-SCOPED.
//
//  1. Names carry the run token: evolve-bridge-r<runid8>-c<cycle>-<agent>-…
//     so observers/watchers can ASSERT ownership (CB.6) and a human reading
//     `tmux ls` during an M-run fleet sees which run owns what. RunID=""
//     keeps the legacy name byte-identical (single-driver mode unchanged).
//  2. Every ephemeral session is RECORDED in the per-run registry
//     (<workspace>/tmux-sessions.jsonl) at creation, so run teardown reaps
//     exactly what this run created — by registry, never by glob.
package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

func TestResolveSessionRunScopedPrefix(t *testing.T) {
	t.Parallel()
	deps := Deps{Now: time.Now}.withDefaults()

	withRun := &Config{Cycle: 12, Agent: "build", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	got, named := resolveSession(withRun, deps, "evolve-bridge-")
	if named {
		t.Fatal("ephemeral session misreported as named")
	}
	if !strings.HasPrefix(got, "evolve-bridge-r01ARZ3ND-c12-build-") {
		t.Errorf("session=%q, want prefix evolve-bridge-r01ARZ3ND-c12-build- — the run token must namespace the session", got)
	}

	legacy := &Config{Cycle: 12, Agent: "build"}
	got, _ = resolveSession(legacy, deps, "evolve-bridge-")
	if !strings.HasPrefix(got, "evolve-bridge-c12-build-") {
		t.Errorf("legacy session=%q, want prefix evolve-bridge-c12-build- — RunID=\"\" must stay byte-identical", got)
	}
}

// TestLaunchRecordsSessionInRunRegistry: the driver appends a record for the
// session it creates, into the run's own registry file, before the launch
// completes — so even a crash mid-phase leaves the session reapable.
func TestLaunchRecordsSessionInRunRegistry(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.RunID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "runscope-record", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil", code, err)
	}

	recs, err := sessionrecord.ReadAll(sessionrecord.PathIn(cfg.Workspace))
	if err != nil {
		t.Fatalf("read run session registry: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("registry records=%d, want 1; recs=%+v", len(recs), recs)
	}
	r := recs[0]
	if r.Session != "runscope-record" || r.RunID != cfg.RunID || r.Agent != "build" {
		t.Errorf("record=%+v, want session=runscope-record run_id=%s agent=build", r, cfg.RunID)
	}
}

// TestRecipeEnsureSessionRecordsInRunRegistry: the recipe path is a second
// session-creation surface (review HIGH on this slice) — a crash between
// EnsureSession and the caller's deferred KillSession would leak an
// unrecorded session, the exact pre-CB.5 state.
func TestRecipeEnsureSessionRecordsInRunRegistry(t *testing.T) {
	ws := t.TempDir()
	tm := &fakeTmux{paneSeq: []string{"❯"}}
	d := &recipeSessionDriver{
		cfg:        &Config{Workspace: ws, Agent: "recipe", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		deps:       recipeDeps(tm),
		session:    "evolve-bridge-recipe-record-pin",
		launchCmd:  "claude",
		marker:     "❯",
		scrollback: recipeBootScrollback,
	}
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	recs, err := sessionrecord.ReadAll(sessionrecord.PathIn(ws))
	if err != nil || len(recs) != 1 {
		t.Fatalf("registry recs=%v err=%v, want exactly the recipe session recorded", recs, err)
	}
	if recs[0].Session != "evolve-bridge-recipe-record-pin" || recs[0].RunID != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Errorf("record=%+v, want the recipe session + run id", recs[0])
	}
}

// runIDPinDriver records the Config the engine's full args-build → parse
// pipeline delivered (the middle hop no other test exercises end-to-end).
type runIDPinDriver struct{ got *string }

func (runIDPinDriver) Name() string { return "cb5-runid-pin" }
func (d runIDPinDriver) Launch(_ context.Context, cfg *Config, _ Deps) (int, error) {
	*d.got = cfg.RunID
	return ExitOK, nil
}

// TestEngineLaunchThreadsRunID: BridgeRequest.RunID → --run-id arg →
// parseLaunchArgs → Config.RunID, through the real Engine.Launch pipeline.
// Not parallel: mutates the global driver registry.
func TestEngineLaunchThreadsRunID(t *testing.T) {
	var got string
	Register(runIDPinDriver{got: &got})
	defer func() { ResetDriversForTesting(); registerBuiltins() }()

	ws := t.TempDir()
	prof := writeJSON(t, filepath.Join(ws, "p.json"), `{"name":"n","model":"haiku"}`)
	eng := NewEngine(Deps{LookupEnv: mapLookup(nil)})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "cb5-runid-pin", Profile: prof, Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"),
		RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
	}); err != nil {
		t.Fatalf("Engine.Launch: %v", err)
	}
	if got != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Errorf("driver saw Config.RunID=%q, want the request's run id — the engine hop dropped --run-id", got)
	}
}

func TestParseLaunchArgsRunID(t *testing.T) {
	t.Parallel()
	raw, err := parseLaunchArgs([]string{"--run-id=01ARZ3NDEKTSV4RRFFQ69G5FAV"}, nil)
	if err != nil {
		t.Fatalf("parseLaunchArgs: %v", err)
	}
	if raw.runID != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Errorf("runID=%q, want the flag value", raw.runID)
	}
}
