package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func coverNow() time.Time { return time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC) }

// --- enforceNext: all five decision branches -------------------------------

// TestEnforceNext covers the router-vs-kernel gate: empty/equal proposal, an
// illegal edge (CanTransition false), a legal-but-spine-gated edge
// (SpineSatisfiedUpTo false), and a legal differing edge that survives.
func TestEnforceNext(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{
		sm:  NewStateMachine(),
		cfg: config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}},
	}
	sig := router.RoutingSignals{} // scout absent → build's spine gate fails
	cases := []struct {
		name      string
		next      string
		wantPhase Phase
		wantOK    bool
	}{
		{"empty-proposal", "", PhaseTriage, false},
		{"equals-static", "triage", PhaseTriage, false},
		{"illegal-edge", "ship", PhaseTriage, false},   // scout↛ship
		{"spine-gated", "build", PhaseTriage, false},   // legal edge, scout artifact absent
		{"accepted", "tdd", PhaseTDD, true},            // legal, needs 0 anchors
	}
	for _, tc := range cases {
		gotPhase, gotOK := o.enforceNext(PhaseScout, PhaseTriage, sig,
			router.RouterDecision{NextPhase: tc.next})
		if gotPhase != tc.wantPhase || gotOK != tc.wantOK {
			t.Errorf("%s: enforceNext = (%s, %v), want (%s, %v)", tc.name, gotPhase, gotOK, tc.wantPhase, tc.wantOK)
		}
	}
}

// --- recordRoutingDecision: happy + skip-phases + error tolerance ----------

// TestRecordRoutingDecision_HappyAndSkips covers the artifact write, SHA, and
// the per-skip-phase ledger loop.
func TestRecordRoutingDecision_HappyAndSkips(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := &Orchestrator{ledger: led, now: coverNow}
	ws := t.TempDir()
	dec := router.RouterDecision{NextPhase: "audit", SkipPhases: []string{"plan-review", "tester"}}
	o.recordRoutingDecision(context.Background(), 5, CycleState{WorkspacePath: ws}, 1, dec)

	if _, err := os.Stat(filepath.Join(ws, "routing-decision-1.json")); err != nil {
		t.Errorf("routing-decision artifact missing: %v", err)
	}
	// 1 routing_decision + 2 phase_skipped.
	if len(led.entries) != 3 {
		t.Fatalf("ledger entries = %d, want 3", len(led.entries))
	}
	if led.entries[0].Kind != "routing_decision" || led.entries[0].ArtifactSHA256 == "" {
		t.Errorf("first entry = %+v, want routing_decision with SHA", led.entries[0])
	}
}

// TestRecordRoutingDecision_ArtifactFailIsSwallowed covers two branches where
// the artifact cannot be written — both produce exactly 1 ledger entry with a
// blank ArtifactPath (forensics must never abort a cycle):
//   - mkdir-fail:  workspace is under a regular file → MkdirAll errors
//   - write-fail:  mkdir succeeds but the artifact path is a directory → WriteFile errors
func TestRecordRoutingDecision_ArtifactFailIsSwallowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ws   func(t *testing.T) string
	}{
		{
			// workspace under a regular file → MkdirAll fails before WriteFile.
			name: "mkdir-fail",
			ws: func(t *testing.T) string {
				t.Helper()
				blocker := filepath.Join(t.TempDir(), "blocker")
				if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(blocker, "ws")
			},
		},
		{
			// artifact path is a directory → mkdir succeeds, WriteFile fails.
			name: "write-fail",
			ws: func(t *testing.T) string {
				t.Helper()
				ws := t.TempDir()
				if err := os.MkdirAll(filepath.Join(ws, "routing-decision-1.json"), 0o755); err != nil {
					t.Fatal(err)
				}
				return ws
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			led := &fakeLedger{}
			o := &Orchestrator{ledger: led, now: coverNow}
			o.recordRoutingDecision(context.Background(), 5,
				CycleState{WorkspacePath: tc.ws(t)}, 1,
				router.RouterDecision{NextPhase: "audit"})
			if len(led.entries) != 1 {
				t.Fatalf("ledger entries = %d, want 1", len(led.entries))
			}
			if led.entries[0].ArtifactPath != "" {
				t.Errorf("artifact failure should blank artifactPath, got %q", led.entries[0].ArtifactPath)
			}
		})
	}
}

// TestRecordRoutingDecision_LedgerFailIsSwallowed covers the swallowed
// ledger-append-error path (forensics must never abort a cycle).
func TestRecordRoutingDecision_LedgerFailIsSwallowed(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{ledger: &fakeLedger{failOnAppend: true}, now: coverNow}
	// Must not panic even though every Append errors.
	o.recordRoutingDecision(context.Background(), 5, CycleState{WorkspacePath: t.TempDir()}, 1,
		router.RouterDecision{NextPhase: "audit", SkipPhases: []string{"tester"}})
}

// --- archivePollutedWorkspace: stat/non-dir/readdir error branches ----------

func TestArchivePollutedWorkspace_Branches(t *testing.T) {
	t.Parallel()

	// Non-directory workspace → returns nil (nothing to archive).
	t.Run("not-a-dir", func(t *testing.T) {
		t.Parallel()
		f := filepath.Join(t.TempDir(), "ws-file")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := archivePollutedWorkspace(f, coverNow); err != nil {
			t.Errorf("non-dir workspace should be nil, got %v", err)
		}
	})

	// Stat returns a non-IsNotExist error (path component is a file → ENOTDIR).
	t.Run("stat-error", func(t *testing.T) {
		t.Parallel()
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := archivePollutedWorkspace(filepath.Join(blocker, "ws"), coverNow); err == nil {
			t.Error("expected stat error for path under a file")
		}
	})

	// ReadDir error: a directory that exists but cannot be listed.
	t.Run("readdir-error", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := filepath.Join(t.TempDir(), "ws")
		if err := os.MkdirAll(dir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		if err := archivePollutedWorkspace(dir, coverNow); err == nil {
			t.Error("expected readdir error for unreadable workspace")
		}
	})
}

// --- decideAfterRetro: the two reachable arms ------------------------------

// TestDecideAfterRetro covers the retro-PASS→ship arm and the non-strict
// PROCEED→end arm.
//
// The RETRY/BLOCK switch arms are unreachable FROM THIS CALL SITE: decideAfterRetro
// calls failureadapter.Decide with Options{Now:...} (Strict=false), and every
// non-Proceed Decision in failureadapter is gated behind `if opts.Strict`
// (failureadapter.go:158/175/188/201/215/227). So with the current call site,
// Decide can only return ActionProceed → the default arm. If a future change
// passes Strict=true here, those arms become reachable and must be tested then.
func TestDecideAfterRetro(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{now: coverNow}

	next, env, reason := o.decideAfterRetro(VerdictPASS, nil)
	if next != PhaseShip || !strings.Contains(reason, "retro-recovered") {
		t.Errorf("PASS arm = (%s, %q), want ship/retro-recovered", next, reason)
	}
	if env != nil {
		t.Errorf("PASS arm should set no extra env, got %v", env)
	}

	next, _, reason = o.decideAfterRetro(VerdictFAIL, nil)
	if next != PhaseEnd || !strings.Contains(reason, "proceed") {
		t.Errorf("non-strict FAIL arm = (%s, %q), want end/proceed", next, reason)
	}
}

// --- router_proposer pure functions ----------------------------------------

// TestRoutingProposerOptions covers WithProposerCLI/WithProposerModel (both the
// override and the empty-ignored guard) and the NewRoutingProposer opts loop.
func TestRoutingProposerOptions(t *testing.T) {
	t.Parallel()
	p := NewRoutingProposer(nil, WithProposerCLI("codex-tmux"), WithProposerModel("opus"))
	if p.cli != "codex-tmux" {
		t.Errorf("cli = %q, want codex-tmux", p.cli)
	}
	if p.model != "opus" {
		t.Errorf("model = %q, want opus", p.model)
	}
	// Empty values must NOT override the defaults.
	d := NewRoutingProposer(nil, WithProposerCLI(""), WithProposerModel(""))
	if d.cli != "claude-tmux" || d.model != "haiku" {
		t.Errorf("empty opts overrode defaults: cli=%q model=%q", d.cli, d.model)
	}
}

// TestBuildRoutingPrompt_FullSignalsAndTriggers covers writeSignals' four
// present-branches and buildRoutingPrompt's optional-triggers block.
func TestBuildRoutingPrompt_FullSignalsAndTriggers(t *testing.T) {
	t.Parallel()
	in := router.RouteInput{
		Current:         "build",
		Verdict:         VerdictPASS,
		Cycle:           9,
		Completed:       []string{"scout", "build"},
		BudgetRemaining: 12.5,
		Signals: router.RoutingSignals{
			Scout:  router.ScoutSignals{Present: true, CycleSizeEstimate: "medium", ItemCount: 3, CarryoverCount: 1},
			Triage: router.TriageSignals{Present: true, CycleSize: "small", PhaseSkip: []string{"plan-review"}},
			Build:  router.BuildSignals{Present: true, Verdict: "PASS", ACSGreen: 5, ACSRed: 1, FilesTouched: 4},
			Audit:  router.AuditSignals{Present: true, Verdict: "PASS", Confidence: 0.9, RedCount: 0},
		},
		Cfg: config.RoutingConfig{
			Mandatory:     []string{"scout", "build", "audit", "ship"},
			MaxInsertions: 4,
			Triggers:      map[string]config.RoutingBlock{"tester": {}, "plan-review": {}},
		},
	}
	got := buildRoutingPrompt(in)
	for _, want := range []string{
		"scout: cycle_size_estimate=medium",
		"triage: cycle_size=small",
		"build: verdict=PASS",
		"audit: verdict=PASS",
		"- plan-review",
		"- tester",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\n---\n%s", want, got)
		}
	}
}

// TestParseProposal_MalformedJSON covers the json.Unmarshal error branch: a
// JSON object that is syntactically broken between the braces.
func TestParseProposal_MalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := parseProposal(`prefix {"next_phase": } suffix`); err == nil {
		t.Error("expected unmarshal error for malformed JSON object")
	}
}

// --- statemachine pure spine-floor functions --------------------------------

// TestPrecedingAnchorBound covers every case of the non-anchor → anchor-bound map.
func TestPrecedingAnchorBound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		target Phase
		want   int
	}{
		{PhaseStart, 0},
		{PhaseIntent, 0},
		{PhaseScout, 0},
		{PhaseTriage, 0},
		{PhaseTDD, 0},
		{PhaseBuildPlanner, 0},
		{PhaseBuild, 1},
		{PhaseRetro, 3},
		{PhaseEnd, 3},
		{PhaseShip, 0}, // default
	}
	for _, tc := range cases {
		if got := precedingAnchorBound(tc.target); got != tc.want {
			t.Errorf("precedingAnchorBound(%s) = %d, want %d", tc.target, got, tc.want)
		}
	}
}

// TestAnchorArtifactPresent covers each anchor branch including the audit
// verdict gate (only PASS/WARN count) and the default "no pre-artifact" return.
func TestAnchorArtifactPresent(t *testing.T) {
	t.Parallel()
	full := router.RoutingSignals{
		Scout: router.ScoutSignals{Present: true},
		Build: router.BuildSignals{Present: true},
		Audit: router.AuditSignals{Present: true, Verdict: VerdictPASS},
	}
	cases := []struct {
		name   string
		anchor Phase
		sig    router.RoutingSignals
		want   bool
	}{
		{"scout-present", PhaseScout, full, true},
		{"scout-absent", PhaseScout, router.RoutingSignals{}, false},
		{"build-present", PhaseBuild, full, true},
		{"build-absent", PhaseBuild, router.RoutingSignals{}, false},
		{"audit-pass", PhaseAudit, full, true},
		{"audit-warn", PhaseAudit, router.RoutingSignals{Audit: router.AuditSignals{Present: true, Verdict: VerdictWARN}}, true},
		{"audit-fail-rejected", PhaseAudit, router.RoutingSignals{Audit: router.AuditSignals{Present: true, Verdict: "FAIL"}}, false},
		{"audit-absent", PhaseAudit, router.RoutingSignals{}, false},
		{"ship-no-preartifact", PhaseShip, router.RoutingSignals{}, true},
		{"default-phase", PhaseIntent, router.RoutingSignals{}, true},
	}
	for _, tc := range cases {
		if got := anchorArtifactPresent(tc.anchor, tc.sig); got != tc.want {
			t.Errorf("%s: anchorArtifactPresent(%s) = %v, want %v", tc.name, tc.anchor, got, tc.want)
		}
	}
}
