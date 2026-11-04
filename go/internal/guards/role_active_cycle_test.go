package guards

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	selfHealTarget  = "/repo/boundary/self-heal-report.md"
	protectedTarget = "/repo/go/internal/guards/role.go"
)

var activeCycleNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func leaseGuard(t *testing.T, cs core.CycleState, alive func(int) bool) *Role {
	t.Helper()
	g := newRoleWithHome(cycleStateOnly{cs: cs}, false, fixtureHome)
	g.now = func() time.Time { return activeCycleNow }
	g.alive = alive
	return g
}

func writeLease(t *testing.T, dir string, l runlease.Lease, at time.Time) {
	t.Helper()
	if err := runlease.Write(dir, l, at); err != nil {
		t.Fatal(err)
	}
}

func decideOutside(g *Role, path string) core.GuardDecision {
	return g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": path},
	})
}

func TestRole_CycleActive(t *testing.T) {
	t.Parallel()
	fresh := activeCycleNow.Add(-time.Minute)
	stale := activeCycleNow.Add(-time.Hour)
	live := func(int) bool { return true }
	dead := func(int) bool { return false }
	cases := []struct {
		name  string
		phase string
		id    int
		lease func(t *testing.T, ws string)
		alive func(int) bool
		want  bool
	}{
		{"no cycle", "build", 0, nil, live, false},
		{"live phase needs no lease", "build", 7, nil, dead, true},
		{"aborted without lease", "aborted", 7, nil, live, false},
		{"end without lease", string(core.PhaseEnd), 7, nil, live, false},
		{"aborted with dead owner", "aborted", 7, func(t *testing.T, ws string) {
			writeLease(t, ws, runlease.Lease{OwnerPID: 99}, fresh)
		}, dead, false},
		{"aborted with stale lease", "aborted", 7, func(t *testing.T, ws string) {
			writeLease(t, ws, runlease.Lease{OwnerPID: 99}, stale)
		}, live, false},
		{"aborted with live owner", "aborted", 7, func(t *testing.T, ws string) {
			writeLease(t, ws, runlease.Lease{OwnerPID: 99}, fresh)
		}, live, true},
		{"aborted with unreadable lease", "aborted", 7, func(t *testing.T, ws string) {
			if err := os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, dead, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			if tc.lease != nil {
				tc.lease(t, ws)
			}
			cs := core.CycleState{CycleID: tc.id, Phase: tc.phase, WorkspacePath: ws}
			got, err := leaseGuard(t, cs, tc.alive).cycleActive(cs)
			if got != tc.want {
				t.Errorf("cycleActive=%v, want %v", got, tc.want)
			}
			if wantErr := tc.name == "aborted with unreadable lease"; (err != nil) != wantErr {
				t.Errorf("cycleActive err=%v, want error=%v", err, wantErr)
			}
		})
	}
}

func TestRole_CycleActive_TerminalPhaseWithoutWorkspaceIsNotActive(t *testing.T) {
	t.Parallel()
	cs := core.CycleState{CycleID: 7, Phase: "aborted"}
	if active, _ := leaseGuard(t, cs, func(int) bool { return true }).cycleActive(cs); active {
		t.Error("a terminal phase with no workspace has no lease and must not be active")
	}
}

func TestRole_AbortedCycleWithDeadOwnerAllowsTheSelfHealWrite(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	g := leaseGuard(t, cs, func(int) bool { return false })
	out := selfHealTarget
	if dec := decideOutside(g, out); !dec.Allow {
		t.Errorf("aborted cycle, no lease: deny %q, want allow", dec.Reason)
	}
}

func TestRole_AbortedCycleWithLiveOwnerStillRefuses(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLease(t, ws, runlease.Lease{OwnerPID: 99}, activeCycleNow.Add(-time.Minute))
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	g := leaseGuard(t, cs, func(int) bool { return true })
	if dec := decideOutside(g, selfHealTarget); dec.Allow {
		t.Error("a live owner on a terminal phase must still be refused")
	}
}

func TestRole_AbortedCycleWithUnreadableLeaseStillRefuses(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	g := leaseGuard(t, cs, func(int) bool { return false })
	if dec := decideOutside(g, selfHealTarget); dec.Allow {
		t.Error("an unreadable lease must keep the refusal")
	}
}

func TestRole_ProtectedSurfaceHoldsForEachActiveCycle(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLease(t, ws, runlease.Lease{OwnerPID: 99}, activeCycleNow.Add(-time.Minute))
	for _, phase := range []string{"build", "aborted", string(core.PhaseEnd)} {
		cs := core.CycleState{CycleID: 7, Phase: phase, WorkspacePath: ws}
		g := leaseGuard(t, cs, func(int) bool { return true })
		dec := decideOutside(g, protectedTarget)
		if dec.Allow || !dec.Alarm || !strings.Contains(dec.Reason, "INTEGRITY VIOLATION") {
			t.Errorf("phase %s: allow=%v alarm=%v reason=%q, want an alarmed integrity deny", phase, dec.Allow, dec.Alarm, dec.Reason)
		}
	}
}

func TestRole_DeadOwnerOnATerminalCycleAllowsAProtectedWrite(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLease(t, ws, runlease.Lease{OwnerPID: 99}, activeCycleNow.Add(-time.Minute))
	for _, phase := range []string{"aborted", string(core.PhaseEnd)} {
		cs := core.CycleState{CycleID: 7, Phase: phase, WorkspacePath: ws}
		dec := decideOutside(leaseGuard(t, cs, func(int) bool { return false }), protectedTarget)
		if !dec.Allow || dec.Alarm {
			t.Errorf("phase %s: allow=%v alarm=%v, want a plain allow: a dead owner reads as no active cycle", phase, dec.Allow, dec.Alarm)
		}
	}
}

func TestRole_UnreadableLeaseRefusesAProtectedWriteWithTheIntegrityReason(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	dec := decideOutside(leaseGuard(t, cs, func(int) bool { return false }), protectedTarget)
	if dec.Allow || !dec.Alarm || !strings.Contains(dec.Reason, "INTEGRITY VIOLATION") ||
		!strings.Contains(dec.Reason, "run lease could not be read") {
		t.Errorf("allow=%v alarm=%v reason=%q, want an alarmed integrity deny that names the lease", dec.Allow, dec.Alarm, dec.Reason)
	}
}

func TestRole_UnreadableLeaseReasonSaysTheLeaseCouldNotBeRead(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	dec := decideOutside(leaseGuard(t, cs, func(int) bool { return false }), selfHealTarget)
	if dec.Allow || !strings.Contains(dec.Reason, "run lease could not be read") {
		t.Errorf("allow=%v reason=%q, want a deny that says the lease could not be read", dec.Allow, dec.Reason)
	}
}

func TestRole_UnreadableLeaseStillAllowsAWorkspaceWrite(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 7, Phase: "aborted", WorkspacePath: ws}
	if dec := decideOutside(leaseGuard(t, cs, func(int) bool { return false }), ws+"/report.md"); !dec.Allow {
		t.Errorf("a workspace write must pass: %q", dec.Reason)
	}
}

func TestRole_LivePhaseRefusesAProtectedWriteWithTheIntegrityReason(t *testing.T) {
	t.Parallel()
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: t.TempDir()}
	dec := decideOutside(leaseGuard(t, cs, func(int) bool { return false }), protectedTarget)
	if dec.Allow || !dec.Alarm || !strings.Contains(dec.Reason, "INTEGRITY VIOLATION") {
		t.Errorf("allow=%v alarm=%v reason=%q, want an alarmed integrity deny", dec.Allow, dec.Alarm, dec.Reason)
	}
}

func TestNewRole_DefaultsCarryTheProductionSeams(t *testing.T) {
	t.Parallel()
	g := newRoleWithHome(nil, false, fixtureHome)
	if g.now == nil || g.alive == nil {
		t.Fatal("now and alive must default to the real clock and pid probe")
	}
	if !g.alive(os.Getpid()) {
		t.Error("the default pid probe must see the running test process")
	}
	if time.Since(g.now()) > time.Minute {
		t.Error("the default clock must be the real clock")
	}
}

func TestProbe_EmptyWorkspaceIgnoresALeaseInTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	writeLease(t, dir, runlease.Lease{OwnerPID: 99}, activeCycleNow.Add(-time.Minute))
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	cs := core.CycleState{CycleID: 7, Phase: "aborted"}
	if active, _ := leaseGuard(t, cs, func(int) bool { return true }).cycleActive(cs); active {
		t.Error("empty workspace must not read a lease from the working directory")
	}
}
