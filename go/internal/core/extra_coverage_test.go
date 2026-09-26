package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func coverNow() time.Time { return time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC) }

func TestEnforceNext(t *testing.T) {
	t.Parallel()
	// Catalog carries one optional user phase "extra-check" so the candidatePhase
	// catalog-lookup branch (a router-proposed USER phase) is exercised, not just
	// the built-in phaseFromRouter path.
	userCat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "extra-check", Optional: true}})
	o := &Orchestrator{
		sm:      NewStateMachine(),
		cfg:     config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}, Order: []string{"scout", "extra-check", "triage", "tdd", "build", "audit", "ship"}},
		catalog: userCat,
	}
	sig := router.RoutingSignals{} // scout absent → build's spine gate fails
	cases := []struct {
		name        string
		next        string
		shipPlanned bool
		wantPhase   Phase
		wantOK      bool
	}{
		{"empty-proposal", "", true, PhaseTriage, false},
		{"equals-static", "triage", true, PhaseTriage, false},
		{"illegal-edge", "ship", true, PhaseTriage, false}, // scout↛ship
		{"spine-gated", "build", true, PhaseTriage, false}, // legal edge, scout artifact absent
		{"accepted", "tdd", true, PhaseTDD, true},          // legal, needs 0 anchors
		// Early-exit: advisor proposes end from scout. Allowed only for a no-ship
		// cycle; a ship-intended cycle is declined (must satisfy the floor).
		{"early-exit-noship", "end", false, PhaseEnd, true},
		{"early-exit-blocked-when-ship", "end", true, PhaseTriage, false},
		// User phase proposed by the advisor resolves via candidatePhase's catalog
		// branch (forward-progress legal: scout→extra-check in cfg.Order).
		{"user-phase-insert", "extra-check", true, Phase("extra-check"), true},
	}
	for _, tc := range cases {
		gotPhase, gotOK := o.enforceNext(PhaseScout, PhaseTriage, VerdictPASS, sig,
			router.RouterDecision{NextPhase: tc.next}, tc.shipPlanned)
		if gotPhase != tc.wantPhase || gotOK != tc.wantOK {
			t.Errorf("%s: enforceNext = (%s, %v), want (%s, %v)", tc.name, gotPhase, gotOK, tc.wantPhase, tc.wantOK)
		}
	}
}

func TestEnforceNext_EmptyOrderSkipAdvance(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{sm: NewStateMachine(), cfg: config.RoutingConfig{}}
	gotPhase, gotOK := o.enforceNext(PhaseScout, PhaseTriage, VerdictPASS, router.RoutingSignals{},
		router.RouterDecision{SkipPhases: []string{"triage"}}, true)
	if gotPhase != PhaseTriage || gotOK {
		t.Errorf("empty-order skip-advance: enforceNext = (%s, %v), want (%s, false) — skipped staticNext must survive, never become PhaseEnd", gotPhase, gotOK, PhaseTriage)
	}
}

func TestPlanRunsShip(t *testing.T) {
	t.Parallel()
	if !planRunsShip(nil) {
		t.Error("nil plan must be treated as ship-intended (no silent early-exit)")
	}
	shipRun := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "ship", Run: true}}}
	if !planRunsShip(shipRun) {
		t.Error("plan with ship run=true should report ship planned")
	}
	shipSkipped := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "ship", Run: false}}}
	if planRunsShip(shipSkipped) {
		t.Error("plan with ship run=false is a no-ship cycle")
	}
	noShip := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
	if planRunsShip(noShip) {
		t.Error("plan without a ship entry is a no-ship cycle")
	}
}

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

func TestRecordRoutingDecision_LedgerFailIsSwallowed(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{ledger: &fakeLedger{failOnAppend: true}, now: coverNow}
	// Must not panic even though every Append errors.
	o.recordRoutingDecision(context.Background(), 5, CycleState{WorkspacePath: t.TempDir()}, 1,
		router.RouterDecision{NextPhase: "audit", SkipPhases: []string{"tester"}})
}

func TestRecordPhasePlan_HappyAndClamps(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := &Orchestrator{ledger: led, now: coverNow}
	ws := t.TempDir()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	clamps := []router.Clamp{
		{Rule: "ship-requires-build", Proposed: "build=skip", Forced: "build=run"},
		{Rule: "ship-requires-audit", Proposed: "audit=skip", Forced: "audit=run"},
	}
	o.recordPhasePlan(context.Background(), 5, CycleState{WorkspacePath: ws}, plan, clamps)

	if _, err := os.Stat(filepath.Join(ws, "phase-plan.json")); err != nil {
		t.Errorf("phase-plan artifact missing: %v", err)
	}
	if len(led.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1 (phase_plan)", len(led.entries))
	}
	if led.entries[0].Kind != "phase_plan" || led.entries[0].ArtifactSHA256 == "" {
		t.Errorf("entry = %+v, want phase_plan with SHA", led.entries[0])
	}
}

func TestRecordPhasePlan_ArtifactFailIsSwallowed(t *testing.T) {
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
				if err := os.MkdirAll(filepath.Join(ws, "phase-plan.json"), 0o755); err != nil {
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
			plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "build", Run: true}}}
			o.recordPhasePlan(context.Background(), 5, CycleState{WorkspacePath: tc.ws(t)}, plan, nil)
			if len(led.entries) != 1 {
				t.Fatalf("ledger entries = %d, want 1", len(led.entries))
			}
			if led.entries[0].ArtifactPath != "" {
				t.Errorf("artifact failure should blank artifactPath, got %q", led.entries[0].ArtifactPath)
			}
		})
	}
}

func TestRecordPhasePlan_LedgerFailIsSwallowed(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{ledger: &fakeLedger{failOnAppend: true}, now: coverNow}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "build", Run: true}}}
	o.recordPhasePlan(context.Background(), 5, CycleState{WorkspacePath: t.TempDir()}, plan,
		[]router.Clamp{{Rule: "ship-requires-build", Proposed: "build=skip", Forced: "build=run"}})
}

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

func TestDecideAfterRetro(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{now: coverNow}

	next, env, reason, _ := o.decideAfterRetro(CycleState{}, VerdictPASS, nil)
	if next == PhaseShip {
		t.Errorf("PASS arm = (%s, %q), must not ship a cycle the auditor rejected", next, reason)
	}
	if !strings.Contains(reason, "proceed:") {
		t.Errorf("PASS arm reason = %q, want the same proceed: disposition as a retro FAIL", reason)
	}
	if env != nil {
		t.Errorf("PASS arm should set no extra env, got %v", env)
	}

	next, _, reason, _ = o.decideAfterRetro(CycleState{}, VerdictFAIL, nil)
	if next != PhaseEnd || !strings.Contains(reason, "proceed") {
		t.Errorf("non-strict FAIL arm = (%s, %q), want end/proceed", next, reason)
	}
}

func TestPhaseAdvisorOptions(t *testing.T) {
	t.Parallel()
	p := NewPhaseAdvisor(nil, WithProposerCLI("codex-tmux"), WithProposerModel("opus"))
	if p.identity.CLI != "codex-tmux" {
		t.Errorf("cli = %q, want codex-tmux", p.identity.CLI)
	}
	if p.identity.Model != "opus" {
		t.Errorf("model = %q, want opus", p.identity.Model)
	}
	d := NewPhaseAdvisor(nil, WithProposerCLI(""), WithProposerModel(""))
	if d.identity.CLI != "claude-tmux" || d.identity.Model != "opus" {
		t.Errorf("empty opts overrode defaults: cli=%q model=%q", d.identity.CLI, d.identity.Model)
	}
}

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
