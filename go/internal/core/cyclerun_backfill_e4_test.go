package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestTriageErrorDiagnostics_ReadsOnlyTheLatestTriageTiming(t *testing.T) {
	t.Parallel()
	errDiag := Diagnostic{Severity: cyclestate.SeverityError, Message: "inbox unreadable"}
	warnDiag := Diagnostic{Severity: "warn", Message: "slow"}
	older := Diagnostic{Severity: cyclestate.SeverityError, Message: "stale triage error"}
	cases := []struct {
		name    string
		timings []phaseTimingEntry
		want    []string
	}{
		{"no timings", nil, nil},
		{"no triage timing", []phaseTimingEntry{{Phase: string(PhaseScout), Diagnostics: []Diagnostic{errDiag}}}, nil},
		{"triage before a later phase", []phaseTimingEntry{
			{Phase: string(PhaseTriage), Diagnostics: []Diagnostic{older}},
			{Phase: string(PhaseTriage), Diagnostics: []Diagnostic{errDiag, warnDiag}},
			{Phase: string(PhaseBuild), Diagnostics: []Diagnostic{errDiag}},
		}, []string{"inbox unreadable"}},
		{"a clean latest triage hides an older error", []phaseTimingEntry{
			{Phase: string(PhaseTriage), Diagnostics: []Diagnostic{older}},
			{Phase: string(PhaseTriage), Diagnostics: []Diagnostic{warnDiag}},
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := triageErrorDiagnostics(tc.timings)
			var msgs []string
			for _, d := range got {
				msgs = append(msgs, d.Message)
			}
			if strings.Join(msgs, "|") != strings.Join(tc.want, "|") || (tc.want == nil) != (got == nil) {
				t.Fatalf("diagnostics = %+v, want messages %v", got, tc.want)
			}
		})
	}
}

func TestNewCycleRun_AFleetLanePinsItsOwnCycleStateFileUntilCleanup(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "")
	storage := &fakeUpdaterStorage{}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
	root := t.TempDir()

	init, cleanup, err := o.newCycleRun(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("newCycleRun: %v", err)
	}
	want := filepath.Join(init.cs.WorkspacePath, CycleStateFile)
	if got := os.Getenv(ipcenv.CycleStateFileKey); got != want {
		t.Fatalf("%s = %q during the cycle, want the lane's own %q", ipcenv.CycleStateFileKey, got, want)
	}
	cleanup(false, true)
	if _, set := os.LookupEnv(ipcenv.CycleStateFileKey); set {
		t.Fatalf("%s must be unset after cleanup so a reused process never carries it", ipcenv.CycleStateFileKey)
	}
}

func TestNewCycleRun_AFailedFleetIsolationAbortsAndReleasesEverything(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "")
	orig := fleetIsolationSetenv
	fleetIsolationSetenv = func(string, string) error { return errors.New("setenv refused") }
	t.Cleanup(func() { fleetIsolationSetenv = orig })
	storage := &fakeStorage{}
	worktree := &fakeWorktree{path: t.TempDir()}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(worktree))

	_, cleanup, err := o.newCycleRun(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})

	if err == nil || err.Error() != "fleet cycle-state isolation: set "+ipcenv.CycleStateFileKey+": setenv refused" {
		t.Fatalf("err = %v, want the isolation failure named", err)
	}
	if cleanup != nil {
		t.Fatal("an aborted start must not hand back a cleanup")
	}
	if storage.lockHeld {
		t.Fatal("the project lock must be released on the abort")
	}
	if id, _ := o.currentRunID.Load().(string); id != "" {
		t.Fatalf("run id %q survived the abort", id)
	}
	if len(worktree.createdCycles) != 0 {
		t.Fatalf("a worktree was provisioned after the abort: %v", worktree.createdCycles)
	}
}

func TestPlanCycle_LedgerStampFailuresWarnAndThePlanStillCarriesTheDirectives(t *testing.T) {
	t.Parallel()

	ledger := &fakeLedger{failOnAppend: true}
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil),
		WithCatalogRefresher(func(context.Context) error { return nil }),
		WithDirectivesProvider(func(context.Context, int) directives.Set {
			return directives.Set{Merged: "## Operator Directives\n- be terse\n", Version: "v-77"}
		}))
	ws := t.TempDir()

	var plan cyclePlan
	var sink injectedStderr
	o.stderr = &sink
	plan = o.planCycle(context.Background(), CycleRequest{ProjectRoot: ws}, State{}, CycleState{WorkspacePath: ws}, 77)
	log := sink.String()

	for _, want := range []string{"WARN catalog-refresh ledger stamp failed: forced ledger append fail", "WARN operator-directives ledger stamp failed: forced ledger append fail"} {
		if !strings.Contains(log, want) {
			t.Errorf("stderr lacks %q:\n%s", want, log)
		}
	}
	if plan.directivesSet.Version != "v-77" || !strings.Contains(plan.directivesSet.Merged, "be terse") {
		t.Fatalf("a failed stamp dropped the directives: %+v", plan.directivesSet)
	}
}

func TestPlanCycle_ScopePathsSkipBlankIDsAndRefuseUnencodablePairs(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	var asked []string
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithScopePathResolver(func(_, id string) string {
			asked = append(asked, id)
			switch id {
			case "spaced-path":
				return "/inbox/has space.json"
			case "no-record":
				return ""
			}
			return "/inbox/" + id + ".json"
		}))
	req := CycleRequest{ProjectRoot: ws, Env: map[string]string{ipcenv.FleetScopeKey: "alpha, ,k=v,spaced-path,,no-record,ünï"}}

	var plan cyclePlan
	var sink injectedStderr
	o.stderr = &sink
	plan = o.planCycle(context.Background(), req, State{}, CycleState{WorkspacePath: ws}, 5)
	log := sink.String()

	if got := plan.ctxSnap["fleet_scope_paths"]; got != "alpha=/inbox/alpha.json ünï=/inbox/ünï.json" {
		t.Fatalf("fleet_scope_paths = %q", got)
	}
	if strings.Join(asked, ",") != "alpha,k=v,spaced-path,no-record,ünï" {
		t.Fatalf("resolver asked for %v; blank ids must never be resolved", asked)
	}
	if !strings.Contains(log, `cannot encode id "k=v"`) || !strings.Contains(log, `path "/inbox/has space.json"`) {
		t.Fatalf("stderr must refuse each unencodable pair loudly:\n%s", log)
	}
}

func TestPlanCycle_AnUnwritableWorkspaceKeepsTheContextChallengeToken(t *testing.T) {
	t.Parallel()

	ws := filepath.Join(t.TempDir(), "workspace-is-a-file")
	writeFile(t, ws, "x")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	var plan cyclePlan
	var sink injectedStderr
	o.stderr = &sink
	plan = o.planCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, State{}, CycleState{WorkspacePath: ws}, 6)
	log := sink.String()

	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(plan.ctxSnap["challengeToken"]) {
		t.Fatalf("challengeToken = %q, want 8 hex-encoded bytes", plan.ctxSnap["challengeToken"])
	}
	if !strings.Contains(log, "WARN challenge-token.txt write failed") {
		t.Fatalf("stderr must warn about the lost fallback file:\n%s", log)
	}
}

func TestPlanCycle_AFailedTokenMintLeavesNoTokenAnywhere(t *testing.T) {
	orig := challengeTokenRead
	challengeTokenRead = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	t.Cleanup(func() { challengeTokenRead = orig })
	ws := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	var plan cyclePlan
	var sink injectedStderr
	o.stderr = &sink
	plan = o.planCycle(context.Background(), CycleRequest{ProjectRoot: ws}, State{}, CycleState{WorkspacePath: ws}, 8)
	log := sink.String()

	if tok, ok := plan.ctxSnap["challengeToken"]; ok {
		t.Fatalf("a failed mint set challengeToken %q", tok)
	}
	if _, err := os.Stat(filepath.Join(ws, "challenge-token.txt")); !os.IsNotExist(err) {
		t.Fatalf("a failed mint wrote challenge-token.txt: %v", err)
	}
	if !strings.Contains(log, "WARN challenge token mint failed: entropy unavailable") {
		t.Fatalf("stderr must surface the mint failure:\n%s", log)
	}
}

func TestPlanCycle_ACallerTokenIsNeverReminted(t *testing.T) {
	orig := challengeTokenRead
	challengeTokenRead = func([]byte) (int, error) {
		t.Fatal("a caller-supplied token must not be reminted")
		return 0, nil
	}
	t.Cleanup(func() { challengeTokenRead = orig })
	ws := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	plan := o.planCycle(context.Background(), CycleRequest{ProjectRoot: ws, Context: map[string]string{"challengeToken": "caller-tok"}}, State{}, CycleState{WorkspacePath: ws}, 9)

	if plan.ctxSnap["challengeToken"] != "caller-tok" {
		t.Fatalf("challengeToken = %q, want the caller's", plan.ctxSnap["challengeToken"])
	}
}
