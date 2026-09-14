package main

// cmd_loop_unit13_golden_test.go — ADR-0103 unit 13 fold 0: the characterization
// goldens captured on 8e8f080f BEFORE the wave engine (cmd_loop_wave.go) and
// the chain engine (cmd_loop_chain.go) moved into internal/loopwave and
// internal/loopchain. Every test here drives the package-main spellings the
// unit keeps as facades, so the same fixtures replay through the leaves. The
// goldens live beside the leaves (internal/<leaf>/testdata) so the leaf tests
// read the identical bytes; temp paths are templated {ROOT} / {EVOLVE_DIR},
// RFC3339 stamps {TS}.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	u13WaveGoldens  = "../../internal/loopwave/testdata"
	u13ChainGoldens = "../../internal/loopchain/testdata"
)

var u13RFC3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)
var u13ReadDirOp = regexp.MustCompile(`\b[a-z_]+ (\S+): not a directory`)

func u13Golden(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return string(raw)
}

// u13Template replaces the fixture's temp paths and stamps with placeholders.
func u13Template(s, root, evolveDir string) string {
	s = strings.ReplaceAll(s, evolveDir, "{EVOLVE_DIR}")
	s = strings.ReplaceAll(s, root, "{ROOT}")
	// The os.ReadDir fault's PathError Op is Go-version- and OS-dependent
	// (`open` / `fdopendir` / `readdirent`); the goldens keep `open` (see
	// loopchain's template).
	s = u13ReadDirOp.ReplaceAllString(s, "open $1: not a directory")
	return u13RFC3339.ReplaceAllString(s, "{TS}")
}

func u13Compare(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s drifted from the golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func u13WriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// u13Launcher records every Run and answers one clean result per spec.
type u13Launcher struct{ calls [][]fleet.CycleSpec }

func (l *u13Launcher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	l.calls = append(l.calls, specs)
	out := make([]fleet.Result, len(specs))
	for i := range specs {
		out[i] = fleet.Result{Index: i}
	}
	return out
}

func u13FloorsPlan(context.Context, int) ([]byte, []string, error) {
	return []byte(`{"committed_floors":["core"]}`), nil, nil
}

// --- 1. the three step-error strings (dispatchIteration ≡ forceOneLaneDispatch) ---

func u13StepErrorLines(t *testing.T) string {
	t.Helper()
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage"}
	ok := func() error { return nil }
	cases := []struct {
		step      string
		preflight func() error
		plan      wavePlanFn
	}{
		{"preflight", func() error { return errors.New("dirty control plane") }, u13FloorsPlan},
		{"plan", ok, func(context.Context, int) ([]byte, []string, error) { return nil, nil, errors.New("plan exploded") }},
		{"adapt", ok, func(context.Context, int) ([]byte, []string, error) { return []byte("{not json"), nil, nil }},
	}
	var b strings.Builder
	for _, c := range cases {
		wave, one := &u13Launcher{}, &u13Launcher{}
		ran, _, _, err := dispatchIteration(context.Background(), fc, c.preflight, c.plan, wave, nil, 3)
		ran1, _, _, err1 := forceOneLaneDispatch(context.Background(), c.preflight, c.plan, one, nil, 3)
		if ran || ran1 || err == nil || err1 == nil || len(wave.calls)+len(one.calls) != 0 {
			t.Fatalf("%s: a step failure never launches (ran=%v/%v err=%v/%v)", c.step, ran, ran1, err, err1)
		}
		if err.Error() != err1.Error() {
			t.Errorf("%s: the two dispatchers render one text: %q vs %q", c.step, err, err1)
		}
		fmt.Fprintf(&b, "%s\t%s\n", c.step, err.Error())
	}
	return b.String()
}

func TestWaveDispatch_StepErrorsAreByteIdenticalToTheGolden(t *testing.T) {
	u13Compare(t, "step errors", u13StepErrorLines(t), u13Golden(t, u13WaveGoldens, "wave_step_errors.golden.txt"))
	refusal := errors.New("dirty control plane")
	_, _, _, err := dispatchIteration(context.Background(), policy.FleetConfig{Count: 2, PlanSource: "triage"},
		func() error { return refusal }, u13FloorsPlan, &u13Launcher{}, nil, 1)
	if !errors.Is(err, refusal) {
		t.Errorf("the cause stays errors.Is-matchable: %v", err)
	}
}

// --- 2. the decision bytes: prune keeps every key, widen re-marshals top_n only ---

// u13PruneFixture seeds the prior cycle's decision (cycle 7) with one pending
// and one consumed id and drives the production plan source: the prune runs
// before the widen, and with alpha the only pending item the widen has
// nothing to add, so the plan's bytes ARE the prune's.
func u13PruneFixture(t *testing.T, decision string) (plan func(stderr io.Writer) []byte, root string) {
	t.Helper()
	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "alpha.json"), map[string]any{"id": "alpha", "weight": 0.9, "files": []string{"a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "processed", "gamma.json"), map[string]any{"id": "gamma", "weight": 0.7, "files": []string{"g.go"}})
	if err := os.MkdirAll(cycleWorkspace(root, 7), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleWorkspace(root, 7), "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}
	return func(stderr io.Writer) []byte {
		data, _, err := productionWavePlanFn(cfg, &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}, 2, stderr)(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}, root
}

func u13WidenFixture(t *testing.T) (data []byte, evolveDir string) {
	t.Helper()
	evolveDir = filepath.Join(t.TempDir(), ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "alpha.json"), map[string]any{"id": "alpha", "weight": 0.9, "files": []string{"a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "beta.json"), map[string]any{"id": "beta", "weight": 0.8, "files": []string{"b.go"}})
	return []byte(`{"note":"dropped","top_n":[{"id":"alpha","files":["a.go"]}]}`), evolveDir
}

func TestWavePlan_PruneAndWidenBytesAreByteIdenticalToTheGolden(t *testing.T) {
	plan, _ := u13PruneFixture(t, `{"note":"keep","top_n":[{"id":"alpha","files":["a.go"]},{"id":"gamma","files":["g.go"]}]}`)
	var stderr bytes.Buffer
	u13Compare(t, "prune", string(plan(&stderr)), u13Golden(t, u13WaveGoldens, "decision_prune.golden.json"))
	if !strings.Contains(stderr.String(), `pruned consumed top_n id "gamma"`) {
		t.Errorf("the prune names the dropped id: %q", stderr.String())
	}
	untouched := `{"note":"keep","top_n":[{"id":"alpha","files":["a.go"]}]}`
	plan, _ = u13PruneFixture(t, untouched)
	if got := plan(io.Discard); string(got) != untouched {
		t.Errorf("nothing consumed returns the original bytes: %s", got)
	}

	narrow, evolveDir := u13WidenFixture(t)
	u13Compare(t, "widen", string(widenNarrowDecision(narrow, evolveDir, 2)), u13Golden(t, u13WaveGoldens, "decision_widen.golden.json"))
	floors := []byte(`{"committed_floors":["core"],"top_n":[]}`)
	if got := widenNarrowDecision(floors, evolveDir, 2); !bytes.Equal(got, floors) {
		t.Errorf("committed_floors pass through: %s", got)
	}
	if got := widenNarrowDecision(narrow, evolveDir, 1); !bytes.Equal(got, narrow) {
		t.Errorf("count<2 passes through: %s", got)
	}
}

// --- 3. the inbox seed: bytes and the exact refusal text ---

func u13SeedFixture(t *testing.T, todos int) string {
	t.Helper()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	for i := 0; i < todos; i++ {
		id := fmt.Sprintf("todo-%c", 'a'+i)
		u13WriteJSON(t, filepath.Join(evolveDir, "inbox", id+".json"),
			map[string]any{"id": id, "weight": 0.9 - float64(i)/10, "files": []string{fmt.Sprintf("pkg/%s/%s.go", id, id)}})
	}
	return evolveDir
}

func TestWaveSeed_JSONAndErrorAreByteIdenticalToTheGolden(t *testing.T) {
	data, err := seedWavePlanFromInbox(u13SeedFixture(t, 3), 1)
	if err != nil {
		t.Fatal(err)
	}
	u13Compare(t, "seed", string(data), u13Golden(t, u13WaveGoldens, "decision_seed.golden.json"))
	_, err = seedWavePlanFromInbox(u13SeedFixture(t, 1), 2)
	if err == nil || err.Error() != "inbox seed: 1 disjoint lane(s) — need >= 2 file-disjoint inbox todos to fill a wave" {
		t.Errorf("the refusal text is pinned: %v", err)
	}
}

// --- 4. every stderr line the wave file writes, labeled per case ---

func u13WaveStderrSections(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	section := func(name string, fn func(w io.Writer)) {
		var buf bytes.Buffer
		fn(&buf)
		fmt.Fprintf(&b, "== %s\n%s", name, buf.String())
	}
	base := policy.FleetConfig{Count: 3, MinLanes: 2, PlanSource: "triage", Scheduling: "wave"}
	section("reload_changed", func(w io.Writer) {
		dir := t.TempDir()
		u13WritePolicy(t, dir, `{"fleet":{"count":5,"min_lanes":2,"plan_source":"weird"}}`)
		reloadFleetConfigAtWaveBoundary(dir, base, w)
	})
	section("reload_unchanged", func(w io.Writer) {
		dir := t.TempDir()
		u13WritePolicy(t, dir, `{"fleet":{"count":3,"min_lanes":2,"plan_source":"triage"}}`)
		reloadFleetConfigAtWaveBoundary(dir, base, w)
	})
	section("reload_unreadable", func(w io.Writer) {
		dir := t.TempDir()
		u13WritePolicy(t, dir, `{not json`)
		out := u13Template(u13CaptureWriter(func(x io.Writer) { reloadFleetConfigAtWaveBoundary(dir, base, x) }), dir, dir)
		io.WriteString(w, out)
	})
	section("reload_concurrency_only", func(w io.Writer) {
		dir := t.TempDir()
		u13WritePolicy(t, dir, `{"fleet":{"count":3,"min_lanes":2,"concurrency":9,"plan_source":"triage"}}`)
		reloadFleetConfigAtWaveBoundary(dir, base, w)
	})
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	section("size_nil_budget", func(w io.Writer) {
		quotaAwareWaveConfig(policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1}, t.TempDir(), w, tightQuota(now), fastPace(), now)
	})
	section("size_shadow", func(w io.Writer) {
		fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: &policy.FleetBudgetConfig{Stage: "shadow", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
		quotaAwareWaveConfig(fc, t.TempDir(), w, tightQuota(now), tokenFastPace(1110), now)
	})
	section("size_enforce", func(w io.Writer) {
		fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
		quotaAwareWaveConfig(fc, t.TempDir(), w, tightQuota(now), fastPace(), now)
	})
	section("routed_refused", func(w io.Writer) {
		root := t.TempDir()
		u13WriteJSON(t, filepath.Join(root, ".evolve", "inbox", "console.json"),
			map[string]any{"id": "console-item", "weight": 0.9, "files": []string{"go/internal/guards/integrity_surface.go"}})
		consoleRoutedResolver(root, w)("console-item")
	})
	section("prune_dropped", func(w io.Writer) {
		plan, _ := u13PruneFixture(t, `{"note":"keep","top_n":[{"id":"alpha","files":["a.go"]},{"id":"gamma","files":["g.go"]}]}`)
		plan(w)
	})
	section("launcher_all_stale", func(w io.Writer) {
		root := t.TempDir()
		for _, id := range []string{"a", "b"} {
			// The promoter nests processed/ by cycle (lifecycle.promoteDestPath); the
			// console names the cycle dir, so the fixture writes the real layout.
			u13WriteJSON(t, filepath.Join(root, ".evolve", "inbox", "processed", "cycle-9", id+".json"), map[string]any{"id": id, "weight": 0.5, "files": []string{id + ".go"}})
		}
		productionWaveLauncher(policy.FleetConfig{Concurrency: 1}, "", root, "", "", io.Discard, w).Run(context.Background(), []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"b"}}})
	})
	return b.String()
}

// u13MinWidthSections drives the four minWidthRepair branches through the
// production sink topology (testRootSignals) — the console they render on.
func u13MinWidthSections(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	run := func(name string, fleetCfg, waveCfg policy.FleetConfig, preflight func() error, plan wavePlanFn) {
		var console bytes.Buffer
		handled := minWidthRepair(context.Background(), fleetCfg, waveCfg, preflight, plan, &u13Launcher{}, nil, 4, &console, testRootSignals(t, &console))
		fmt.Fprintf(&b, "== %s handled=%v\n%s", name, handled, console.String())
	}
	ok := func() error { return nil }
	run("minwidth_guard_not_met", policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1}, ok, u13FloorsPlan)
	run("minwidth_dispatched", policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 1}, ok, u13FloorsPlan)
	run("minwidth_empty_backlog", policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1}, ok,
		func(context.Context, int) ([]byte, []string, error) {
			return []byte(`{"committed_floors":[]}`), nil, nil
		})
	run("minwidth_failed", policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1}, func() error { return errors.New("dirty control plane") }, u13FloorsPlan)
	return b.String()
}

func u13WritePolicy(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func u13CaptureWriter(fn func(io.Writer)) string {
	var buf bytes.Buffer
	fn(&buf)
	return buf.String()
}

// The `.golden.txt` files are the base capture (8e8f080f) — the leaf tests
// derive every replaced line's reason from them. The `.rendered.golden.txt`
// files are the fold-4 console: byte-identical for every KEPT line, and the
// declared replacements (D-1 the three min-width lines, D-2 the dispatch
// failure, D-4 the all-stale gate) rendered by the root sink as
// `[loop] loop.wave WARN <CODE> … — <the old sentence> k=v`. The all-stale
// row disappears from the test-only launcher facade's console (a Null
// Center); the leaf asserts the signal.
func TestWaveStderr_EveryLineIsByteIdentical(t *testing.T) {
	u13Compare(t, "wave stderr", u13WaveStderrSections(t), u13Golden(t, u13WaveGoldens, "stderr_wave.rendered.golden.txt"))
	u13Compare(t, "min-width console", u13MinWidthSections(t), u13Golden(t, u13WaveGoldens, "stderr_minwidth.rendered.golden.txt"))
	// Every kept line of the base capture survives verbatim.
	base := u13Golden(t, u13WaveGoldens, "stderr_wave.golden.txt")
	for _, line := range strings.Split(base, "\n") {
		if strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[fleet] freshness gate: all") && !strings.Contains(u13WaveStderrSections(t), line) {
			t.Errorf("kept line missing from the fold-4 console: %q", line)
		}
	}
}

// --- 5. the chain summary JSON per stop reason ---

// u13ChainEnv seeds a chain project: `items` pending todos, a rebuilt binary
// and a stale pin (so a scripted refresh can fire).
func u13ChainEnv(t *testing.T, items int) (root, evolveDir string) {
	t.Helper()
	root, evolveDir, _ = brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	for i := 0; i < items; i++ {
		u13WriteJSON(t, filepath.Join(evolveDir, "inbox", fmt.Sprintf("item-%d.json", i)), map[string]any{"id": fmt.Sprintf("item-%d", i)})
	}
	return root, evolveDir
}

// u13StubRefresh scripts every refresh seam so a boundary refresh fires
// deterministically (rebuild/re-exec are recorded, never real).
func u13StubRefresh(t *testing.T, ahead func() bool) {
	t.Helper()
	prevAhead, prevCommit, prevProv := chainBoundaryAheadFn, chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	prevRebuild, prevReExec, prevArgv := chainRebuildFn, chainReExecFn, chainReExecArgvFn
	t.Cleanup(func() {
		chainBoundaryAheadFn, chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevAhead, prevCommit, prevProv
		chainRebuildFn, chainReExecFn, chainReExecArgvFn = prevRebuild, prevReExec, prevArgv
	})
	chainBoundaryAheadFn = func(string, string) (bool, error) { return ahead(), nil }
	chainRunningCommitFn = func() string { return "cafebabe1234deadbeef" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234deadbeef", func(c string) bool { return c == "cafebabe1234deadbeef" }
	}
	chainRebuildFn = func(string) error { return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }
	chainReExecArgvFn = func() []string { return []string{"evolve", "loop", "--until-inbox-empty"} }
}

func u13ChainRun(t *testing.T, cfg loopConfig, cc policy.ChainConfig, batch func(n int) int) (rc int, stdout, stderr string) {
	t.Helper()
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	n := 0
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { n++; return batch(n) }
	var out, errb bytes.Buffer
	rc = runLoopChain(cfg, cc, nil, &out, &errb)
	return rc, u13Template(out.String(), cfg.ProjectRoot, cfg.EvolveDir), u13Template(errb.String(), cfg.ProjectRoot, cfg.EvolveDir)
}

type u13ChainCase struct {
	reason string
	exit   int
	items  int
	cap    int
	batch  func(n int) int
	setup  func(t *testing.T, root, evolveDir string)
}

func u13ChainCases() []u13ChainCase {
	zero := func(int) int { return 0 }
	return []u13ChainCase{
		{"brake", 0, 2, 5, zero, func(t *testing.T, _, evolveDir string) {
			if err := os.WriteFile(filepath.Join(evolveDir, chainBrakeFile), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"inbox_empty", 0, 0, 5, zero, nil},
		{"max_batches", 0, 3, 2, zero, nil},
		{"quota_defer", 5, 3, 5, func(int) int { return 5 }, func(t *testing.T, _, evolveDir string) {
			u13WriteJSON(t, filepath.Join(evolveDir, "cycle-state.json"), map[string]any{"cycle_id": 7,
				"checkpoint": map[string]any{"enabled": true, "reason": "quota-likely", "quotaResetAt": "2026-09-14T18:00:00Z", "quotaResetSource": "probe"}})
		}},
		{"batch_error", 2, 3, 5, func(int) int { return 2 }, nil},
		{"inbox_unreadable", 2, 0, 5, zero, func(t *testing.T, _, evolveDir string) {
			if err := os.RemoveAll(filepath.Join(evolveDir, "inbox")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("not a dir"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"boundary_refresh_reexec", 0, 3, 5, zero, nil},
	}
}

func u13ChainStdout(t *testing.T, c u13ChainCase) (rc int, stdout, stderr string) {
	t.Helper()
	root, evolveDir := u13ChainEnv(t, c.items)
	if c.setup != nil {
		c.setup(t, root, evolveDir)
	}
	boundary := 0
	u13StubRefresh(t, func() bool { boundary++; return c.reason == "boundary_refresh_reexec" && boundary == 2 })
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	return u13ChainRun(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: c.cap}, c.batch)
}

func TestChainResult_StdoutIsByteIdenticalPerStopReason(t *testing.T) {
	for _, c := range u13ChainCases() {
		t.Run(c.reason, func(t *testing.T) {
			rc, stdout, _ := u13ChainStdout(t, c)
			if rc != c.exit {
				t.Errorf("exit = %d, want %d", rc, c.exit)
			}
			u13Compare(t, "chain_result_"+c.reason, stdout, u13Golden(t, u13ChainGoldens, "chain_result_"+c.reason+".golden.json"))
		})
	}
}

// --- 6. the attempt marker and the JSONL audit entry ---

func u13MarkerAndLog(t *testing.T) (marker, logLine string) {
	t.Helper()
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	var stderr bytes.Buffer
	if !maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 7, &stderr) {
		t.Fatalf("the refresh fires: %s", stderr.String())
	}
	m, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile))
	if err != nil {
		t.Fatal(err)
	}
	l, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshLogFile))
	if err != nil {
		t.Fatal(err)
	}
	return u13Template(string(m), root, evolveDir), u13Template(string(l), root, evolveDir)
}

func TestChainBoundaryRefresh_MarkerAndLogBytesAreByteIdentical(t *testing.T) {
	marker, logLine := u13MarkerAndLog(t)
	u13Compare(t, "marker", marker, u13Golden(t, u13ChainGoldens, "marker.golden.json"))
	u13Compare(t, "log entry", logLine, u13Golden(t, u13ChainGoldens, "log_entry.golden.jsonl"))
}

// --- 7. all twenty-three chain stderr lines ---

// u13RefreshSections faults each refresh seam in turn and records the stderr
// of maybeRefreshChainBoundary — the thirteen branches.
func u13RefreshSections(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	run := func(name string, prep func(root, evolveDir string)) {
		root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
		u13StubRefresh(t, func() bool { return true })
		// Every seam a section faults is restored right after it — the
		// sections share one process.
		prevLane := chainBoundaryFleetLaneFn
		defer func() { chainBoundaryFleetLaneFn = prevLane }()
		if prep != nil {
			prep(root, evolveDir)
		}
		var stderr bytes.Buffer
		refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 7, &stderr)
		fmt.Fprintf(&b, "== %s refreshed=%v\n%s", name, refreshed, u13Template(stderr.String(), root, evolveDir))
	}
	run("ahead_check_failed", func(string, string) {
		chainBoundaryAheadFn = func(string, string) (bool, error) { return false, errors.New("git fetch: network unreachable") }
	})
	run("lane_check_unverifiable", func(string, string) {
		chainBoundaryFleetLaneFn = func(loopConfig) (bool, error) {
			return false, errors.New("runlease: parse .lease: unexpected end of JSON input")
		}
	})
	run("lane_active", func(string, string) {
		chainBoundaryFleetLaneFn = func(loopConfig) (bool, error) { return true, nil }
	})
	run("breaker_refused", func(_, evolveDir string) {
		u13WriteJSON(t, filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), map[string]any{"running_commit": "cafebabe1234deadbeef", "batch": 6, "timestamp": "2026-09-14T10:00:00Z"})
	})
	run("rebuild_failed", func(string, string) {
		chainRebuildFn = func(string) error { return errors.New("make -C go build: exit status 2: build failed: syntax error") }
	})
	run("no_target", func(root, _ string) {
		if err := os.Remove(filepath.Join(root, "go", "bin", "evolve")); err != nil {
			t.Fatal(err)
		}
	})
	run("repin_refused", func(string, string) {
		chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
			return "cafebabe1234deadbeef", func(string) bool { return false }
		}
	})
	run("audit_log_failed", func(_, evolveDir string) {
		if err := os.Mkdir(filepath.Join(evolveDir, chainBoundaryRefreshLogFile), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	run("empty_argv", func(string, string) { chainReExecArgvFn = func() []string { return nil } })
	run("arm_failed", func(_, evolveDir string) {
		if err := os.Mkdir(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	run("reexec_failed", func(string, string) {
		chainReExecFn = func(string, []string, []string) error { return errors.New("exec format error") }
	})
	run("refreshed", nil)
	return b.String()
}

func u13DriverSections(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, c := range u13ChainCases() {
		_, _, stderr := u13ChainStdout(t, c)
		fmt.Fprintf(&b, "== %s\n%s", c.reason, stderr)
	}
	// The invalid inbox item and the quota defer without a checkpoint block.
	root, evolveDir := u13ChainEnv(t, 1)
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox", "typo-item.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	u13StubRefresh(t, func() bool { return false })
	_, _, stderr := u13ChainRun(t, loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 5}, func(int) int { return 5 })
	fmt.Fprintf(&b, "== invalid_item_then_quota_defer_no_block\n%s", stderr)
	return b.String()
}

// The chain's twenty-three lines: the base capture (`.golden.txt`) is the
// leaf's source of every replaced line's reason; the fold-4 console
// (`.rendered.golden.txt`) keeps the seven INFO-shaped report lines verbatim
// and renders the sixteen replaced ones as signals (D-6).
func TestChainStderr_AllTwentyThreeLinesAreByteIdentical(t *testing.T) {
	u13Compare(t, "refresh stderr", u13RefreshSections(t), u13Golden(t, u13ChainGoldens, "stderr_refresh.rendered.golden.txt"))
	u13Compare(t, "driver stderr", u13DriverSections(t), u13Golden(t, u13ChainGoldens, "stderr_driver.rendered.golden.txt"))
	rendered := u13RefreshSections(t) + u13DriverSections(t)
	for _, name := range []string{"stderr_refresh.golden.txt", "stderr_driver.golden.txt"} {
		for _, line := range strings.Split(u13Golden(t, u13ChainGoldens, name), "\n") {
			if !strings.HasPrefix(line, "[chain]") {
				continue
			}
			sentence := strings.TrimPrefix(line, "[chain] boundary-refresh: ")
			sentence = strings.TrimPrefix(sentence, "[chain] ")
			if !strings.Contains(rendered, sentence) {
				t.Errorf("the base sentence survives (kept verbatim or as a signal's reason): %q", line)
			}
		}
	}
}

// --- 9. the fault-free signal stream: a clean chain emits nothing ---

func TestLoop_FaultFreeStreamIsByteIdentical(t *testing.T) {
	root, evolveDir := u13ChainEnv(t, 1)
	u13StubRefresh(t, func() bool { return false })
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { return 0 }
	var out, errb bytes.Buffer
	if rc := runLoopChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 5}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d: %s", rc, errb.String())
	}
	// The chain's Signal Center records nothing on a clean run: the durable
	// batch-level stream is absent (8e8f080f: no chain Center exists) or empty.
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err == nil && strings.TrimSpace(string(data)) != "" {
		t.Errorf("a clean chain must add nothing to the signal stream:\n%s", data)
	}
	// A clean wave through the wave dispatcher: one INFO summary from the
	// coordinator is the only loop.wave event (the dispatcher itself is silent).
	c := signalcenter.New()
	var kinds []string
	c.Subscribe(func(e signalcenter.Event) {
		kinds = append(kinds, string(e.Module)+"/"+string(e.Kind)+"/"+string(e.Code))
	})
	launcher := &u13Launcher{}
	plan := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"top_n":[{"id":"a","files":["a.go"]},{"id":"b","files":["b.go"]}]}`), nil, nil
	}
	ran, _, results, err := dispatchIteration(context.Background(), policy.FleetConfig{Count: 2, PlanSource: "triage"}, func() error { return nil }, plan, launcher, nil, 1)
	if err != nil || !ran || len(results) != 2 {
		t.Fatalf("a clean wave ran two lanes: ran=%v results=%d err=%v", ran, len(results), err)
	}
	emitLoopWave(c, 1, "loopBatchCoordinator.dispatchFleetIteration", "", "wave 1: 2/2 lanes ok", nil)
	if got := strings.Join(kinds, ","); got != "loop/loop.wave/" {
		t.Errorf("the fault-free wave stream is exactly the coordinator's summary: %q", got)
	}
}
