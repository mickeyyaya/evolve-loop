package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// goldenBridgeResponse is returned on the error arms too, because the reconcile arms read its cost, tokens and boot time.
var goldenBridgeResponse = core.BridgeResponse{ExitCode: 81, CostUSD: 1.5, Tokens: core.TokenUsage{Input: 42}, BootMS: 7}

type goldenBridge struct {
	err         error
	fileContent string
	stdout      string
}

func (b *goldenBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if b.fileContent != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(b.fileContent), 0o644)
	}
	resp := goldenBridgeResponse
	resp.Stdout = b.stdout
	return resp, b.err
}

func (b *goldenBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func transientErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 85, core.ErrTransientBridgeFailure)
}

type verifyKind int

const (
	verifyOK         verifyKind = iota // contracted + well-formed (bytes read from disk)
	verifyNotOK                        // contracted + malformed/absent, codes carried
	verifyErr                          // no contract for the phase (the pane stays the source)
	verifyEmptyNotOK                   // not-OK with NO bytes (an infra read fault) — the late-read arm
	verifySettlesOn                    // not-OK until the settleOn-th probe, then OK
)

type verifySpec struct {
	kind     verifyKind
	codes    []string
	settleOn int
}

type probeCounts struct {
	Verify int `json:"verify"`
	Sleep  int `json:"sleep"`
}

func (v verifySpec) fn(counts *probeCounts) func(string, phasecontract.Roots) (deliverable.Result, error) {
	violations := make([]deliverable.Violation, 0, len(v.codes))
	for _, c := range v.codes {
		violations = append(violations, deliverable.Violation{Code: c, Message: "violation " + c})
	}
	return func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		counts.Verify++
		switch v.kind {
		case verifyErr:
			return deliverable.Result{}, errors.New("no deliverable contract for phase " + phase)
		case verifyEmptyNotOK:
			return deliverable.Result{OK: false, Violations: violations}, nil
		case verifyNotOK:
			return verifiedFrom(deliverable.Result{OK: false, Violations: violations}, phase, roots), nil
		case verifySettlesOn:
			if counts.Verify < v.settleOn {
				return verifiedFrom(deliverable.Result{OK: false, Violations: violations}, phase, roots), nil
			}
		}
		return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
	}
}

type verdictScenario struct {
	name      string
	phase     string
	agent     string
	optional  bool
	bridgeErr error
	file      string
	stdout    string
	verify    verifySpec
	verdict   string
	nextPhase string
	diags     []core.Diagnostic
	stale     bool   // seed a byte-identical pre-dispatch leftover
	acs       string // "" | PASS | FAIL — stage the ACS-floor workspace
	wantErr   bool
	wantErrIs error
}

const (
	// unverifiedReport outgrows the 160-byte forensic tail, so the tail is a real suffix; a wider tail changes the golden.
	unverifiedReport = "# audit\n(partial — no verdict sentinel, no challenge token)\n" +
		"## Findings\n- one\n- two\n- three\n- four\n- five\n- six\n- seven\n- eight\n- nine\n- ten\n- eleven\n- twelve\n- thirteen\n- fourteen\n- fifteen\n- sixteen\n- seventeen\n- eighteen\n- nineteen\n- twenty\n"
	panePASS = "pane says PASS\n"
)

func verdictScenarios() []verdictScenario {
	audit := func(sc verdictScenario) verdictScenario {
		sc.phase, sc.agent = "audit", "evolve-auditor"
		return sc
	}
	return []verdictScenario{
		audit(verdictScenario{name: "timeout_wellformed_pass", bridgeErr: artifactTimeoutErr(), file: verifiedPASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS, nextPhase: "ship"}),
		audit(verdictScenario{name: "timeout_sentinel_fail", bridgeErr: artifactTimeoutErr(), file: "# audit\nFAIL\n", verify: verifySpec{kind: verifyOK}, verdict: core.VerdictFAIL, nextPhase: "retro"}),
		audit(verdictScenario{name: "timeout_malformed_mandatory", bridgeErr: artifactTimeoutErr(), verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS, wantErr: true, wantErrIs: core.ErrArtifactTimeout}),
		audit(verdictScenario{name: "transient_malformed_mandatory", bridgeErr: transientErr(), file: unverifiedReport, verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}}, verdict: core.VerdictPASS, wantErr: true, wantErrIs: core.ErrTransientBridgeFailure}),
		{name: "timeout_malformed_optional", phase: "build-planner", agent: "evolve-build-planner", optional: true, bridgeErr: artifactTimeoutErr(), verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS},
		audit(verdictScenario{name: "timeout_stale_mandatory", bridgeErr: artifactTimeoutErr(), stale: true, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS, wantErr: true, wantErrIs: core.ErrArtifactTimeout}),
		{name: "timeout_stale_optional", phase: "smell-scan", agent: "evolve-smell-scan", optional: true, bridgeErr: artifactTimeoutErr(), stale: true, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS},
		audit(verdictScenario{name: "teardown_acs_floor", bridgeErr: artifactTimeoutErr(), file: reportWithToken, acs: "PASS", verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeStrayInWorktree}}, verdict: core.VerdictPASS, nextPhase: "ship"}),
		audit(verdictScenario{name: "substantive_error", bridgeErr: errors.New("bridge: launch exit=2"), file: verifiedPASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS, wantErr: true}),
		audit(verdictScenario{name: "clean_contracted_ok", file: verifiedPASS, stdout: panePASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS, nextPhase: "ship", diags: []core.Diagnostic{{Severity: "warning", Message: "classify note"}}}),
		audit(verdictScenario{name: "clean_contracted_unverified", file: unverifiedReport, stdout: panePASS, verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}}, verdict: core.VerdictPASS, nextPhase: "ship"}),
		{name: "clean_uncontracted_pane", phase: "widget-scan", agent: "evolve-widget-scan", stdout: panePASS, verify: verifySpec{kind: verifyErr}, verdict: core.VerdictPASS, nextPhase: "audit"},
	}
}

type scenarioRun struct {
	ws, wt, root string
	hooks        *fakeHooks
	counts       probeCounts
	resp         core.PhaseResponse
	err          error
}

func runVerdictScenario(t *testing.T, sc verdictScenario, extra ...func(*Options)) *scenarioRun {
	t.Helper()
	run := &scenarioRun{ws: t.TempDir(), wt: t.TempDir(), root: t.TempDir()}
	if sc.stale {
		seedStaleReport(t, run.ws, sc.phase)
	}
	if sc.acs != "" {
		writeACSFloorWorkspace(t, run.ws, sc.acs)
	}
	run.hooks = &fakeHooks{phase: sc.phase, agent: sc.agent, model: "opus", prompt: "x", verdict: sc.verdict, nextPhase: sc.nextPhase, diagnostics: sc.diags}
	opts := Options{
		Hooks:    run.hooks,
		Bridge:   &goldenBridge{err: sc.bridgeErr, fileContent: sc.file, stdout: sc.stdout},
		Prompts:  fakePromptsFS(sc.agent, "x"),
		NowFn:    fixtures.FixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond),
		Optional: sc.optional,
		VerifyFn: sc.verify.fn(&run.counts),
		SleepFn:  func(time.Duration) { run.counts.Sleep++ },
	}
	for _, apply := range extra {
		apply(&opts)
	}
	run.resp, run.err = New(opts).Run(context.Background(), core.PhaseRequest{Cycle: 7, RunID: "run-11", ProjectRoot: run.root, Workspace: run.ws, Worktree: run.wt})
	return run
}

// templatePaths replaces the temp roots with stable tokens, so a golden captured on one machine compares on another.
func (r *scenarioRun) templatePaths(s string) string {
	repl := []struct{ path, token string }{{r.ws, "{ws}"}, {r.wt, "{wt}"}, {r.root, "{root}"}}
	sort.Slice(repl, func(i, j int) bool { return len(repl[i].path) > len(repl[j].path) })
	for _, p := range repl {
		s = strings.ReplaceAll(s, p.path, p.token)
	}
	return s
}

func (r *scenarioRun) responseJSON(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(r.resp)
	if err != nil {
		t.Fatal(err)
	}
	return r.templatePaths(string(raw))
}

func goldenPath(name string) string { return filepath.Join("verdict", "testdata", name) }

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("golden %s: %v (capture it on the pre-extraction code first)", name, err)
	}
	return string(b)
}
