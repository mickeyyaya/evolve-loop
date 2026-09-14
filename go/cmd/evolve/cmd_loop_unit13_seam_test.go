package main

// cmd_loop_unit13_seam_test.go — ADR-0103 unit 13: the host-side pins. The
// origin guard (fold 0, red on 8e8f080f: "runWaveIteration" names no
// function) and the seam tests of fold 4.

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// loopProducerOrigins collects every origin literal handed to the three loop
// producers in file, keyed by the producer's name.
func loopProducerOrigins(t *testing.T, fset *token.FileSet, file string) []string {
	t.Helper()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var origins []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || (id.Name != "emitLoopHalt" && id.Name != "emitLoopWave" && id.Name != "emitLoopEscalation") || len(call.Args) < 3 {
			return true
		}
		lit, ok := call.Args[2].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Errorf("%s: %s's origin must be a string literal, got %T", file, id.Name, call.Args[2])
			return true
		}
		origin, _ := strconv.Unquote(lit.Value)
		origins = append(origins, origin)
		return true
	})
	return origins
}

// declaredFuncs lists the package's non-test FuncDecls as "name" or
// "Type.method".
func declaredFuncs(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Recv == nil || len(fn.Recv.List) == 0 {
				out[fn.Name.Name] = true
				continue
			}
			typ := fn.Recv.List[0].Type
			if star, ok := typ.(*ast.StarExpr); ok {
				typ = star.X
			}
			if id, ok := typ.(*ast.Ident); ok {
				out[id.Name+"."+fn.Name.Name] = true
			}
		}
	}
	return out
}

// Test 8 — every loop producer's origin names a declared function or
// Type.method (the schema's origin contract); a renamed function must take
// its origin literal with it.
func TestLoopSignalOrigins_NameDeclaredFunctions(t *testing.T) {
	fset := token.NewFileSet()
	declared := declaredFuncs(t, fset)
	for _, file := range []string{"cmd_loop_window.go", "cmd_loop_escalate.go", "cmd_loop_systemfailure_halt.go", "cmd_loop_blockerbreaker.go"} {
		for _, origin := range loopProducerOrigins(t, fset, file) {
			if !declared[origin] {
				t.Errorf("%s: origin %q names no declared function or Type.method in package main", file, origin)
			}
		}
	}
}

// nonTestSourcesMentioningU13 lists the module's non-test Go files outside
// the two unit-13 leaves and the one allowed site whose source contains
// needle (the carryover_lifecycle_test.go idiom, moduleRoot = go/).
func nonTestSourcesMentioningU13(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/loopwave/") || strings.HasPrefix(rel, "internal/loopchain/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// Test 56 — ONE wired wave engine: loopwave.New( lives in the seam file
// only; the five test-only facades have no production caller (one would drop
// the unit's WARNs through the Null Center); the producer's tokens are not
// re-spelled in main.
func TestWaveEngine_OneConstructionSite(t *testing.T) {
	const seam = "cmd/evolve/cmd_loop_wave.go"
	if offenders := nonTestSourcesMentioningU13(t, "loopwave.New(", seam); len(offenders) > 0 {
		t.Errorf("loopwave.New( belongs to %s alone; also in %v", seam, offenders)
	}
	for _, facade := range []string{"dispatchIteration(", "forceOneLaneDispatch(", "minWidthRepair(", "productionWaveLauncher(", "reloadFleetConfigAtWaveBoundary("} {
		if offenders := nonTestSourcesMentioningU13(t, facade, seam); len(offenders) > 0 {
			t.Errorf("%s is a test-only facade (a production caller would run on a Null Center): %v", facade, offenders)
		}
	}
	for _, token := range []string{`stamped["wave"]`, `"LOOP_MIN_WIDTH_REPAIR"`} {
		if offenders := nonTestSourcesMentioningU13(t, token, ""); len(offenders) > 0 {
			t.Errorf("%s is the leaf's alone (single source): %v", token, offenders)
		}
	}
}

// Test 57 — ONE wired Refresher and Driver: the constructions live in the
// chain seam; the 3-arg maybeRefreshChainBoundary (a test facade over a
// throwaway root Center) has no production caller.
func TestChainEngines_OneConstructionSite(t *testing.T) {
	const seam = "cmd/evolve/cmd_loop_chain.go"
	for _, needle := range []string{"loopchain.NewRefresher(", "loopchain.NewDriver("} {
		if offenders := nonTestSourcesMentioningU13(t, needle, seam); len(offenders) > 0 {
			t.Errorf("%s belongs to %s alone; also in %v", needle, seam, offenders)
		}
	}
	src, err := os.ReadFile("cmd_loop_window.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "maybeRefreshChainBoundaryWithSignals(b.cfg, iteration+1, b.stderr, b.deps.Signals)") {
		t.Error("the coordinator passes the batch Center to the refresh (never the 3-arg facade)")
	}
	if offenders := nonTestSourcesMentioningU13(t, "maybeRefreshChainBoundary(", seam); len(offenders) > 0 {
		t.Errorf("the 3-arg facade has production callers: %v", offenders)
	}
}

// Test 58 — the coordinator's lazy engine is cached and reads the batch
// Center live.
func TestLoopBatchCoordinator_WaveEngineIsCachedAndReadsTheCenterLive(t *testing.T) {
	root := t.TempDir()
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}, stderr: io.Discard}
	first := b.wave()
	if first != b.wave() {
		t.Error("wave() caches the engine")
	}
	if first.SignalsWired() {
		t.Error("no Center yet")
	}
	b.deps.Signals = signalcenter.New()
	if !first.SignalsWired() {
		t.Error("the engine reads deps.Signals live, never a snapshot")
	}
}

// Test 59 — the minWidthRepair facade through the production sink: the four
// pinned substrings stay on the console and the ndjson carries the codes.
func TestMinWidthRepairFacade_ConsoleKeepsTheFourPinnedSubstringsAndNdjsonCarriesTheCodes(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	var console bytes.Buffer
	signals := newRootSignalCenter(root, evolveDir, &console)
	plan := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	ok := func() error { return nil }
	minWidthRepair(context.Background(), policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1}, ok, plan, &u13Launcher{}, nil, 0, &console, signals)
	minWidthRepair(context.Background(), policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 1}, ok, plan, &u13Launcher{}, nil, 1, &console, signals)
	minWidthRepair(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1}, ok,
		func(context.Context, int) ([]byte, []string, error) {
			return []byte(`{"committed_floors":[]}`), nil, nil
		}, &u13Launcher{}, nil, 2, &console, signals)
	minWidthRepair(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1}, func() error { return errors.New("dirty control plane") }, plan, &u13Launcher{}, nil, 3, &console, signals)
	signals.Flush()
	out := console.String()
	for _, want := range []string{"empty triage plan", "min-width repair dispatched", "empty backlog", "min-width repair failed", "dirty control plane",
		"LOOP_WAVE_EMPTY_PLAN", "LOOP_MIN_WIDTH_REPAIR", "LOOP_WAVE_DISPATCH_FAILED", "origin=Engine.RepairMinWidth"} {
		if !strings.Contains(out, want) {
			t.Errorf("console lacks %q:\n%s", want, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{`"code":"LOOP_WAVE_EMPTY_PLAN"`, `"code":"LOOP_MIN_WIDTH_REPAIR"`, `"code":"LOOP_WAVE_DISPATCH_FAILED"`} {
		if !strings.Contains(string(data), code) {
			t.Errorf("ndjson lacks %s", code)
		}
	}
}

// Test 60 — the coordinator's wave path renders ONE LOOP_WAVE_DISPATCH_FAILED
// on a preflight refusal (a non-git root) and no hand-written line.
func TestDispatchFleetIteration_PreflightRefusalRendersDispatchFailedOnce(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = &fixtures.FakeStorage{}
	var starvation fleet.StarvationTracker
	d := b.dispatchFleetIteration(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"}, "/nonexistent/evolve", &starvation)
	b.deps.Signals.Flush()
	out := console.String()
	if d.flow != batchProceed {
		t.Errorf("a refused wave falls through to sequential: %+v", d)
	}
	for _, want := range []string{"[loop] loop.wave WARN LOOP_WAVE_DISPATCH_FAILED", "step=preflight", "wave 0 dispatch failed, falling back to sequential"} {
		if !strings.Contains(out, want) {
			t.Errorf("console lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[loop] WARN: fleet: wave") || strings.Count(out, "dispatch failed, falling back to sequential") != 1 {
		t.Errorf("the fact renders ONCE, coded:\n%s", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"LOOP_WAVE_DISPATCH_FAILED"`) {
		t.Errorf("the durable stream carries the code: %v %s", err, data)
	}
}

// Test 61 — the host keeps no hand-written twin of the coded line, and the
// ACS tokens hold by code.
func TestCmdLoopWindow_NoHandWrittenWaveDispatchFailedLine(t *testing.T) {
	src, err := os.ReadFile("cmd_loop_window.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "wave %d dispatch failed, falling back to sequential") {
		t.Error("cmd_loop_window.go re-added the hand-written line the engine now codes")
	}
}

func TestCmdLoopWave_ACSTokensHoldByCode(t *testing.T) {
	src, err := os.ReadFile("cmd_loop_wave.go")
	if err != nil {
		t.Fatal(err)
	}
	coded := false
	for _, line := range strings.Split(string(src), "\n") {
		code, _, _ := strings.Cut(strings.TrimSpace(line), "//") // the code part, never a trailing comment
		if strings.Contains(code, "fleet.QuotaAwareCount") {
			coded = true
		}
	}
	if !coded {
		t.Error("fleet.QuotaAwareCount must be wired in CODE (acs/cycle467), not a comment")
	}
	if strings.Contains(string(src), "context.Background()") {
		t.Error("no context.Background() mint in the seam (acs/cycle467)")
	}
}

// Test 62 — the refresh seam reads the package vars at call time (a cached
// Refresher would ignore a swap between calls).
func TestMaybeRefreshChainBoundary_ReadsTheSeamVarsAtCallTime(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	var stderr bytes.Buffer
	if !maybeRefreshChainBoundary(cfg, 1, &stderr) {
		t.Fatalf("first: %s", stderr.String())
	}
	chainRebuildFn = func(string) error { return errors.New("swapped after the first call") }
	chainRunningCommitFn = func() string { return "moved0000001" }
	if maybeRefreshChainBoundary(cfg, 2, &stderr) || !strings.Contains(stderr.String(), "swapped after the first call") {
		t.Errorf("the second call reads the swapped rebuild: %s", stderr.String())
	}
}

// Test 63 — the coordinator hands the batch Center to the refresh: a rebuild
// failure renders as the coded WARN on the batch console.
func TestPrepareIteration_PassesTheBatchCenterToTheRefresh(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	chainRebuildFn = func(string) error { return errors.New("build failed: syntax error") }
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = &fixtures.FakeStorage{}
	fc := policy.FleetConfig{Count: 1}
	bin := ""
	d := b.prepareIteration(0, &fc, &bin, 0)
	b.deps.Signals.Flush()
	if d.flow != batchProceed {
		t.Errorf("a degraded refresh continues: %+v", d)
	}
	if out := console.String(); !strings.Contains(out, "LOOP_BOUNDARY_REFRESH_SKIPPED") || !strings.Contains(out, "step=rebuild") || !strings.Contains(out, "build failed: syntax error") {
		t.Errorf("the batch console renders the refresh degrade: %s", out)
	}
}

// Test 64 — runLoopChain builds a batch-level Center: its warnings render on
// the console and land in <evolveDir>/signals.ndjson.
func TestRunLoopChain_ConstructsABatchLevelCenterAndRendersItsWarnings(t *testing.T) {
	root, evolveDir := u13ChainEnv(t, 3)
	boundary := 0
	u13StubRefresh(t, func() bool { boundary++; return boundary == 2 })
	chainRebuildFn = func(string) error { return errors.New("build failed: syntax error") }
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { return 0 }
	var stdout, stderr bytes.Buffer
	runLoopChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 2}, nil, &stdout, &stderr)
	if out := stderr.String(); !strings.Contains(out, "[loop] loop.warning WARN LOOP_BOUNDARY_REFRESH_SKIPPED") || !strings.Contains(out, "step=rebuild") {
		t.Errorf("the chain console renders the coded degrade: %s", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"LOOP_BOUNDARY_REFRESH_SKIPPED"`) {
		t.Errorf("the durable batch-level stream carries it: %v %s", err, data)
	}
}

// Test 65 — one loop.wave producer, one registrar for the moved code.
func TestEmitLoopWave_ProjectsToTheLeafProducerWithOneRegistrar(t *testing.T) {
	if CodeLoopMinWidthRepair != loopwave.CodeMinWidthRepair {
		t.Error("CodeLoopMinWidthRepair projects the leaf's code")
	}
	moduleRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	registrars := 0
	_ = filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(path)
		for _, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "RegisterCode(") && strings.Contains(line, "CodeMinWidthRepair") {
				registrars++
			}
		}
		return nil
	})
	if registrars != 1 {
		t.Errorf("LOOP_MIN_WIDTH_REPAIR has %d registrars, want exactly 1 (the leaf)", registrars)
	}
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("the registry is clean: %v", conflicts)
	}
}

// Review fold R1 — the chain summary's stdout schema has ONE home: chainResult
// IS loopchain.Result (an alias, so the by-name tests keep their spelling) and
// no wire tag is re-spelled in package main.
func TestChainResult_IsTheLeafResultWithNoReSpelledSchema(t *testing.T) {
	if reflect.TypeOf(chainResult{}) != reflect.TypeOf(loopchain.Result{}) {
		t.Error("chainResult must alias loopchain.Result — two declarations of one wire schema drift silently")
	}
	for _, tag := range []string{`json:"chain_mode"`, `json:"max_batches"`, `json:"chain_stop_reason"`} {
		if offenders := nonTestSourcesMentioningU13(t, tag, ""); len(offenders) > 0 {
			t.Errorf("%s is the leaf's alone: %v", tag, offenders)
		}
	}
}

// Review fold R2 — the chain root drains its own Center before EVERY batch:
// a chained batch can re-exec at a wave boundary, and that refresh flushes
// only the BATCH Center (D-7), so whatever the chain root emitted at the
// boundary must already be delivered when the batch starts. The held-drain
// idiom of signalcenter/flush_test.go proves the wait: an event queued behind
// a running drain keeps the batch from starting until the drain ends.
func TestWiredChain_FlushesTheChainCenterBeforeEveryBatch(t *testing.T) {
	root, evolveDir := u13ChainEnv(t, 0)
	u13StubRefresh(t, func() bool { return false })
	c := signalcenter.New()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	c.Subscribe(func(e signalcenter.Event) {
		if e.Reason == "slow" {
			entered <- struct{}{}
			<-release
		}
	})
	event := func(reason string) signalcenter.Event {
		return signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "test", Kind: signalcenter.KindLoopWarning, Severity: signalcenter.SeverityInfo, Reason: reason}
	}
	go c.Emit(event("slow")) // this goroutine becomes the drainer and blocks inside the listener
	<-entered
	c.Emit(event("queued behind the held drain")) // enqueues and returns
	batchReached, done := make(chan struct{}), make(chan struct{})
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { close(batchReached); return 0 }
	go func() {
		wiredChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 1}, nil, io.Discard, io.Discard, c).Run()
		close(done)
	}()
	select {
	case <-batchReached:
		t.Fatal("the batch started while a chain-root event was still queued behind a running drain: wiredChain's Batch dep must Flush the chain Center first")
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	for _, ch := range []chan struct{}{batchReached, done} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("the chain did not run its batch and stop after the drain finished")
		}
	}
}

// Review fold R3 — the Roots Parameter Object is never half populated at a
// facade: every nullWaveEngine( construction in the seam passes either
// loopwave.RootsOf(<projectRoot>) (both halves derived the production way) or
// the zero loopwave.Roots{} (the rootless engine of the pure dispatch
// facades); a bare "" half would resolve CWD-relative the day a method reads
// it. The boundary reload constructs nothing: it is a package function.
func TestNullWaveEngine_EveryConstructionCarriesBothRootsOrNone(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "cmd_loop_wave.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "nullWaveEngine" {
			return true
		}
		calls++
		if len(call.Args) != 2 {
			t.Errorf("%s: nullWaveEngine takes (roots, warn), got %d args", fset.Position(call.Pos()), len(call.Args))
			return true
		}
		switch arg := call.Args[0].(type) {
		case *ast.CallExpr:
			if sel, ok := arg.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "RootsOf" {
				t.Errorf("%s: the roots argument must be loopwave.RootsOf(projectRoot)", fset.Position(call.Pos()))
			}
		case *ast.CompositeLit:
			if sel, ok := arg.Type.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Roots" || len(arg.Elts) != 0 {
				t.Errorf("%s: a literal roots argument must be the zero loopwave.Roots{} (rootless), never a half", fset.Position(call.Pos()))
			}
		default:
			t.Errorf("%s: the roots argument is a %T, want loopwave.RootsOf(…) or loopwave.Roots{}", fset.Position(call.Pos()), arg)
		}
		return true
	})
	if calls < 5 {
		t.Errorf("expected the five Null-Center constructions in the seam, found %d", calls)
	}
	src, err := os.ReadFile("cmd_loop_wave.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "return loopwave.ReloadFleetConfig(evolveDir, prev, warn)") {
		t.Error("the boundary reload facade projects onto the package function — no engine, no half-populated roots")
	}
}

// Test 66 — failedLaneCount is a one-line projection onto loopwave.FailedLanes.
func TestFailedLaneCount_ProjectsToTheLeaf(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "cmd_loop_window.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "failedLaneCount" {
			continue
		}
		if len(fn.Body.List) != 1 {
			t.Fatalf("failedLaneCount has %d statements, want one return", len(fn.Body.List))
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			t.Fatal("not a single return")
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			t.Fatal("not a call")
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "FailedLanes" {
			t.Errorf("failedLaneCount must return loopwave.FailedLanes(results)")
		}
		return
	}
	t.Fatal("failedLaneCount not found")
}
