package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func writeLeaseTestPlan(t *testing.T, dir string) (string, *campaign.Plan) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "campaign-plan.json")
	planJSON := `{"version":1,"goal":"lease-test goal","cycles":[{"id":"a","files":["fa.go"]}]}`
	if err := os.WriteFile(planPath, []byte(planJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := loadVerifiedCampaignPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	return planPath, plan
}

func fakeLaunchTracking(t *testing.T) *[][]string {
	t.Helper()
	var mu sync.Mutex
	launched := &[][]string{}
	orig := campaignLaunchFactory
	campaignLaunchFactory = func(_ string, _ bool, _ string, _ string, _ string, _, _ io.Writer) fleet.LaunchFn {
		return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
			mu.Lock()
			*launched = append(*launched, spec.Scope)
			mu.Unlock()
			return 0, nil
		}
	}
	t.Cleanup(func() { campaignLaunchFactory = orig })
	return launched
}

func TestCampaignRun_RefusesWhenLeaseHeld(t *testing.T) {
	dir := t.TempDir()
	planPath, plan := writeLeaseTestPlan(t, dir)
	launched := fakeLaunchTracking(t)

	goalHash := campaignGoalHash(plan)
	leaseDir := campaignLeaseDir(dir)
	incumbent, err := campaign.AcquireOwnership(leaseDir, goalHash, campaign.Owner{PID: 99999, Worktree: "/incumbent"})
	if err != nil {
		t.Fatalf("seed incumbent lease: %v", err)
	}
	defer incumbent.Release()

	var out, errBuf bytes.Buffer
	rc := runCampaignRun([]string{"--plan", planPath, "--project-root", dir}, &out, &errBuf)
	if rc == 0 {
		t.Fatal("runCampaignRun must refuse (non-zero) when the lease is already held")
	}
	if !strings.Contains(errBuf.String(), "already owned") {
		t.Fatalf("refusal must name the incumbent owner, stderr=%q", errBuf.String())
	}
	if len(*launched) != 0 {
		t.Fatalf("a refused campaign must launch NO cycles, launched %v", *launched)
	}
}

func TestCampaignRun_SimulateSkipsOwnershipLease(t *testing.T) {
	dir := t.TempDir()
	planPath, plan := writeLeaseTestPlan(t, dir)
	launched := fakeLaunchTracking(t)

	goalHash := campaignGoalHash(plan)
	incumbent, err := campaign.AcquireOwnership(campaignLeaseDir(dir), goalHash, campaign.Owner{PID: 99999, Worktree: "/incumbent"})
	if err != nil {
		t.Fatalf("seed incumbent lease: %v", err)
	}
	defer incumbent.Release()

	var out, errBuf bytes.Buffer
	rc := runCampaignRun([]string{"--plan", planPath, "--project-root", dir, "--simulate"}, &out, &errBuf)
	if rc != 0 {
		t.Fatalf("--simulate must NOT take ownership, rc=%d stderr=%s", rc, errBuf.String())
	}
	if len(*launched) == 0 {
		t.Fatal("--simulate must still run the wave despite a held lease")
	}
}

func TestCampaignRun_AcquiresAndReleasesLease(t *testing.T) {
	dir := t.TempDir()
	planPath, plan := writeLeaseTestPlan(t, dir)
	_ = fakeLaunchTracking(t)

	var out, errBuf bytes.Buffer
	if rc := runCampaignRun([]string{"--plan", planPath, "--project-root", dir}, &out, &errBuf); rc != 0 {
		t.Fatalf("runCampaignRun rc=%d, stderr=%s", rc, errBuf.String())
	}

	leaseDir := campaignLeaseDir(dir)
	lease, err := campaign.AcquireOwnership(leaseDir, campaignGoalHash(plan), campaign.Owner{PID: 1})
	if err != nil {
		t.Fatalf("lease must be released after the run, got %v", err)
	}
	lease.Release()
}
