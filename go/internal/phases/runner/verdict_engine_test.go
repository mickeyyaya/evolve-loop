package runner

// verdict_engine_test.go — the unit-11 seam (ADR-0103 §6 tests 35-41): the
// ONE construction of the verdict engine, the lazy accessor for a literal
// runner, the ONE projection onto the engine's input, the Center derivation
// (explicit → the bridge's → Null Object), the stream per scenario, and the
// consumer pins on the two beliefs preparation.go now reads from the leaf.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 35 — verdict.New( is spelled in exactly one non-test file of the module.
func TestVerdictEngine_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/phases/runner/verdict_engine.go"
	if offenders := nonTestSourcesMentioning(t, "verdict.New(", onlySite); len(offenders) > 0 {
		t.Errorf("verdict.New( belongs to ONE non-test file (%s); these non-test files construct it too: %v", onlySite, offenders)
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// verdict leaf and the one allowed site whose source contains needle
// (core/carryover_lifecycle_test.go idiom; the module root is three levels up).
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/phases/runner/verdict/") {
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

// Test 36 — the engine has ONE moment of truth: New builds it eagerly and no
// other non-test line of the package assigns it (no lazy accessor — a
// BaseRunner has no production literal, so a lazy branch would be dead code
// with an unsynchronised write; review fold). Kills `a second construction`,
// `New leaves judge nil`.
func TestNew_BuildsTheEngineOnceEagerly(t *testing.T) {
	eager := New(Options{Hooks: &fakeHooks{phase: "audit"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-auditor", "x")})
	if eager.judge == nil {
		t.Fatal("New constructs the engine eagerly")
	}
	assignments := map[string]int{}
	for _, name := range packageNonTestSources(t) {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(src), ".judge = "); n > 0 {
			assignments[name] = n
		}
	}
	if len(assignments) != 1 || assignments["runner.go"] != 1 {
		t.Errorf("the engine is assigned exactly once, in New (runner.go): %v", assignments)
	}
}

// Test 37 — dispatchOf carries every engine input from its ONE source.
func TestDispatchOf_IsTheOneProjection(t *testing.T) {
	req := core.PhaseRequest{Cycle: 9, RunID: "r9", Workspace: "ws", Worktree: "wt", ProjectRoot: "root", ExplanationDocumentationVersion: 2}
	snap, ok := verdict.StatSnapshot(writeTemp(t, "body"))
	if !ok {
		t.Fatal("snapshot fixture")
	}
	prep := phasePreparation{phase: "audit", artifactPath: "ws/audit-report.md", preDispatch: snap, hadPreDispatch: true}
	plan := phaseDispatchPlan{modelSource: "advisor"}
	res := phaseDispatchResult{bridgeResponse: goldenBridgeResponse, bridgeErr: errors.New("e"), resolvedModel: "sonnet", durationMS: 321, fenceDiagnostics: []core.Diagnostic{{Severity: "warning", Message: "fence"}}}
	d := dispatchOf(req, prep, plan, res)
	if d.Cycle != 9 || d.RunID != "r9" || d.Phase != "audit" || d.Workspace != "ws" || d.Worktree != "wt" || d.ProjectRoot != "root" || d.ExplanationDocumentationVersion != 2 ||
		d.ArtifactPath != "ws/audit-report.md" || d.PreDispatch != snap || !d.HadPreDispatch || d.Bridge != goldenBridgeResponse || d.BridgeErr != res.bridgeErr ||
		d.DurationMS != 321 || d.ResolvedModel != "sonnet" || d.ModelSource != "advisor" || len(d.FenceDiagnostics) != 1 || d.FenceDiagnostics[0].Message != "fence" {
		t.Errorf("dispatchOf dropped a field: %+v", d)
	}
}

// packageNonTestSources lists this package's non-test Go files (the leaf's
// nonTestSources idiom) for the source-scan pins.
func packageNonTestSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			out = append(out, name)
		}
	}
	return out
}

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// signalBridge is a bridge double that carries a Center, like the production
// Adapter (Signals() beside SignalsWired()).
type signalBridge struct {
	fakeBridge
	center *signalcenter.Center
}

func (b *signalBridge) Signals() *signalcenter.Center { return b.center }

func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

// Test 38 — the Center derivation: Options.Signals wins, else the bridge's
// when it exposes one, else the Null Object; read live, never snapshotted;
// the event carries the request's identity. Names SignalsWired for the host
// apicover row.
func TestNew_SignalsResolveExplicitThenBridgeThenNull(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	failing := func(string, string) error { return errors.New("filter blowup") }
	if r := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-auditor", "x")}); r.SignalsWired() {
		t.Fatal("a bridge without Signals() ⇒ not wired")
	}
	c, got := recordingCenter()
	viaBridge := New(Options{Hooks: hooks, Bridge: &signalBridge{fakeBridge: fakeBridge{writeArtifact: verifiedPASS}, center: c}, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: alwaysOKVerify, StdoutFilter: failing})
	if !viaBridge.SignalsWired() {
		t.Fatal("the injected bridge's Center is adopted")
	}
	if _, err := viaBridge.Run(context.Background(), core.PhaseRequest{Cycle: 5, RunID: "r5", ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0].Code != verdict.CodeStdoutFilterFailed || (*got)[0].Cycle != 5 || (*got)[0].RunID != "r5" || (*got)[0].Phase != "audit" {
		t.Fatalf("the provoked WARN lands in the bridge's Center with the request's identity: %+v", *got)
	}
	explicit, gotExplicit := recordingCenter()
	both := New(Options{Hooks: hooks, Bridge: &signalBridge{fakeBridge: fakeBridge{writeArtifact: verifiedPASS}, center: c}, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: alwaysOKVerify, StdoutFilter: failing, Signals: func() *signalcenter.Center { return explicit }})
	if _, err := both.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if len(*gotExplicit) != 1 || len(*got) != 1 {
		t.Errorf("Options.Signals wins over the bridge's Center: explicit=%d bridge=%d", len(*gotExplicit), len(*got))
	}
	late := &signalBridge{}
	lateRunner := New(Options{Hooks: hooks, Bridge: late, Prompts: fakePromptsFS("evolve-auditor", "x")})
	if lateRunner.SignalsWired() {
		t.Fatal("a bridge whose Signals() returns nil ⇒ not wired")
	}
	late.center = c
	if !lateRunner.SignalsWired() {
		t.Fatal("the accessor reads the bridge's Center live, never a snapshot taken at New")
	}
}

// Test 39 — no Center anywhere: a provoked fault runs silently, no panic.
func TestNew_NoCenterIsTheNullObject(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &fakeBridge{writeArtifact: verifiedPASS}, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: alwaysOKVerify, StdoutFilter: func(string, string) error { return errors.New("filter blowup") }})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("Null Object: %+v %v", resp, err)
	}
}

// Test 40 — the ordered {module, kind, code} sequence per scenario of the
// harness: empty on the happy, uncontracted, substantive and pass-through
// paths; exactly one code per fault path (RECONCILED after the filter's WARN
// when both fire — the declared fold D2).
func TestRun_StreamIsEmptyOnTheHappyPathAndCarriesOneCodePerFaultPath(t *testing.T) {
	want := map[string][]signalcenter.Code{
		"timeout_wellformed_pass":       {verdict.CodeReconciled},
		"timeout_sentinel_fail":         {verdict.CodeReconciled},
		"timeout_malformed_mandatory":   {verdict.CodeTeardownFail},
		"transient_malformed_mandatory": {verdict.CodeTeardownFail},
		"timeout_malformed_optional":    {verdict.CodeOptionalPhaseDegraded},
		"timeout_stale_mandatory":       {verdict.CodeTeardownFail},
		"timeout_stale_optional":        {verdict.CodeOptionalPhaseDegraded},
		"teardown_acs_floor":            {verdict.CodeReconciled},
		"substantive_error":             {},
		"clean_contracted_ok":           {},
		"clean_contracted_unverified":   {verdict.CodeDeliverableUnverified},
		"clean_uncontracted_pane":       {},
	}
	fields := map[string]map[string]string{
		"timeout_wellformed_pass":     {"via": "verify"},
		"teardown_acs_floor":          {"via": "acs_floor", "overridden_codes": deliverable.CodeStrayInWorktree},
		"timeout_malformed_mandatory": {"cause": "malformed"},
		"timeout_stale_mandatory":     {"cause": "stale_leftover"},
		"clean_contracted_unverified": {"downgraded": "true"},
	}
	for _, sc := range verdictScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			c, got := recordingCenter()
			runVerdictScenario(t, sc, func(o *Options) { o.Signals = func() *signalcenter.Center { return c } })
			var codes []signalcenter.Code
			for _, e := range *got {
				if e.Module != signalcenter.ModuleRunner || e.Kind != signalcenter.KindRunnerWarning || e.Severity != signalcenter.SeverityWarn || e.Cycle != 7 || e.RunID != "run-11" || e.Phase != sc.phase {
					t.Errorf("every runner event is a runner.warning WARN stamped with the request: %+v", e)
				}
				codes = append(codes, e.Code)
				for k, v := range fields[sc.name] {
					if e.Fields[k] != v {
						t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
					}
				}
			}
			w := want[sc.name]
			if len(codes) != len(w) {
				t.Fatalf("stream %v, want %v", codes, w)
			}
			for i := range w {
				if codes[i] != w[i] {
					t.Errorf("stream %v, want %v", codes, w)
				}
			}
		})
	}
	passThrough := verdictScenario{name: "skipped", phase: "intent", agent: "evolve-intent", file: "[intent-unchanged]", stdout: panePASS,
		verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeBadVerdict}}, verdict: core.VerdictSKIPPED}
	c, got := recordingCenter()
	run := runVerdictScenario(t, passThrough, func(o *Options) { o.Signals = func() *signalcenter.Center { return c } })
	if run.resp.Verdict != core.VerdictSKIPPED || len(*got) != 0 {
		t.Errorf("a WARN/SKIPPED pass-through on an unverified deliverable emits nothing: %q %+v", run.resp.Verdict, *got)
	}
	c, got = recordingCenter()
	run = runVerdictScenario(t, verdictScenarios()[0], func(o *Options) {
		o.Signals = func() *signalcenter.Center { return c }
		o.StdoutFilter = func(string, string) error { return errors.New("x") }
	})
	if codes := eventCodes(*got); len(codes) != 2 || codes[0] != verdict.CodeStdoutFilterFailed || codes[1] != verdict.CodeReconciled {
		t.Errorf("a failing filter on a reconciled teardown: [STDOUT_FILTER_FAILED, RECONCILED], got %v (resp=%+v)", codes, run.resp)
	}
}

func eventCodes(events []signalcenter.Event) []signalcenter.Code {
	out := make([]signalcenter.Code, 0, len(events))
	for _, e := range events {
		out = append(out, e.Code)
	}
	return out
}

// Test 41 — the consumer pins on the two beliefs preparation.go reads from
// outside the package: the prompt's challenge-token tail carries exactly
// phasecontract.ChallengeToken(ws) (whitespace-padded fixture; the reader the
// verdict engine's ACS floor shares — review fold F1), and the pre-dispatch
// snapshot follows verdict.StatSnapshot's rule (empty file ⇒ none). The
// package never spells the token file itself (source scan).
func TestPreparation_ChallengeTokenAndSnapshotProjectTheLeaf(t *testing.T) {
	for _, name := range packageNonTestSources(t) {
		if src, err := os.ReadFile(name); err != nil {
			t.Fatal(err)
		} else if strings.Contains(string(src), "challenge-token.txt") {
			t.Errorf("%s spells the challenge-token file — read it through phasecontract.ChallengeToken", name)
		}
	}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("  67dffdcb2fb3ab46 \n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tok, ok := phasecontract.ChallengeToken(ws)
	if !ok || tok != "67dffdcb2fb3ab46" {
		t.Fatalf("the contract's reader: %q %v", tok, ok)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &promptRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-builder", "body")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: ws}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(br.gotPrompt, "<!-- challenge-token: "+tok+" -->") || strings.Contains(br.gotPrompt, "  67dffdcb2fb3ab46 ") {
		t.Errorf("the prompt carries the TRIMMED token the contract's reader read: %q", br.gotPrompt[max(0, len(br.gotPrompt)-300):])
	}
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()}
	empty := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-builder", "body")})
	if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	prep, _, err := empty.preparePhaseExecution(req)
	if err != nil || prep.hadPreDispatch {
		t.Errorf("an empty pre-dispatch file does not snapshot (the leaf's rule): %v %+v", err, prep.hadPreDispatch)
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte("prior attempt"), 0o644); err != nil {
		t.Fatal(err)
	}
	prep, _, err = empty.preparePhaseExecution(req)
	if snap, ok := verdict.StatSnapshot(prep.artifactPath); err != nil || !prep.hadPreDispatch || !ok || prep.preDispatch != snap {
		t.Errorf("a non-empty pre-dispatch file snapshots through the leaf: %v %+v", err, prep)
	}
}
