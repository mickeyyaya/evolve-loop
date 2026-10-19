package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReauditState_RefusesUnusableState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		write func(t *testing.T, path string)
		want  string
	}{
		{"missing file", func(*testing.T, string) {}, "no cycle state"},
		{"malformed json", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "parse cycle state"},
		{"no worktree", func(t *testing.T, path string) {
			raw, _ := json.Marshal(CycleState{CycleID: 7, WorkspacePath: "w"})
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				t.Fatal(err)
			}
		}, "no worktree or workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tt.write(t, ResolveCycleStatePath(dir))

			_, err := loadReauditState(dir, 7)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestReauditCycle_RebaseWithoutConflictIsInfra(t *testing.T) {
	t.Parallel()
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	fx.worktree = fx.root.Dir
	fx.writeCycleState(reauditCycle)
	runner := fx.runner(VerdictPASS)

	_, err := ReauditCycle(context.Background(), fx.input(runner, &fakeLedger{}))

	requireReauditKind(t, err, ReauditInfra)
	if runner.calls != 0 {
		t.Fatalf("audit ran %d times after a failed rebase", runner.calls)
	}
}

func TestReauditCycle_LedgerAppendFailureIsInfra(t *testing.T) {
	t.Parallel()
	fx := newReauditFixture(t, "lane.txt", "peer.txt")

	_, err := ReauditCycle(context.Background(), fx.input(fx.runner(VerdictPASS), &fakeLedger{failOnAppend: true}))

	re := requireReauditKind(t, err, ReauditInfra)
	if !strings.Contains(re.Error(), "record the audit binding") {
		t.Fatalf("Error() = %q, want the binding step named", re.Error())
	}
}

func TestReauditCycle_MissingAuditReportIsInfra(t *testing.T) {
	t.Parallel()
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	silent := &silentAuditRunner{}

	_, err := ReauditCycle(context.Background(), fx.input(silent, &fakeLedger{}))

	requireReauditKind(t, err, ReauditInfra)
}

type silentAuditRunner struct{}

func (silentAuditRunner) Name() string { return string(PhaseAudit) }

func (silentAuditRunner) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS}, nil
}

func TestReauditCycle_UnresolvableMainIsInfraAndAuditsNothing(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	fx.worktree = filepath.Join(t.TempDir(), "gone")
	fx.writeCycleState(reauditCycle)
	real := reauditRebase
	reauditRebase = func(context.Context, string, *CycleState) (bool, bool) { return true, false }
	t.Cleanup(func() { reauditRebase = real })
	runner := fx.runner(VerdictPASS)

	var res ReauditResult
	res, err := ReauditCycle(context.Background(), fx.input(runner, &fakeLedger{}))

	re := requireReauditKind(t, err, ReauditInfra)
	if !strings.Contains(re.Error(), "resolve main") {
		t.Fatalf("Error() = %q, want the resolve-main step named", re.Error())
	}
	if res != (ReauditResult{}) || runner.calls != 0 {
		t.Fatalf("res = %+v, audit calls = %d, want an empty result and no audit", res, runner.calls)
	}
}
