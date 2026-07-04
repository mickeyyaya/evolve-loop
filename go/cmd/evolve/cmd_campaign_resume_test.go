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

// TestCampaignRun_AutoResumeSkipsCompletedWaves drives the full cmd wiring
// (goal-hash, .evolve progress path, PlanSHA binding, RunWaves call) through the
// campaignLaunchFactory DI seam: with wave 0 pre-recorded complete, only wave 1
// should launch, and the status command should report both waves done afterward.
func TestCampaignRun_AutoResumeSkipsCompletedWaves(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "campaign-plan.json")
	planJSON := `{"version":1,"goal":"harden the loop","cycles":[` +
		`{"id":"a","files":["fa.go"]},` +
		`{"id":"b","files":["fb.go"],"depends_on":["a"]}]}`
	if err := os.WriteFile(planPath, []byte(planJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-seed progress: wave 0 (cycle a) already shipped, bound to THIS plan.
	plan, err := loadVerifiedCampaignPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	goalHash := campaignGoalHash(plan)
	progressPath := campaign.ProgressPath(filepath.Join(dir, ".evolve"), goalHash)
	raw, _ := os.ReadFile(planPath)
	seed := &campaign.CampaignProgress{
		PlanSHA:           campaign.HashPlan(raw),
		CompletedWaves:    []int{0},
		CompletedCycleIDs: []string{"a"},
	}
	if err := seed.Save(progressPath); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var launched [][]string
	orig := campaignLaunchFactory
	campaignLaunchFactory = func(_ string, _ bool, _ string, _ string, _, _ io.Writer) fleet.LaunchFn {
		return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
			mu.Lock()
			launched = append(launched, spec.Scope)
			mu.Unlock()
			return 0, nil
		}
	}
	defer func() { campaignLaunchFactory = orig }()

	var out, errBuf bytes.Buffer
	if rc := runCampaignRun([]string{"--plan", planPath, "--project-root", dir}, &out, &errBuf); rc != 0 {
		t.Fatalf("runCampaignRun rc=%d, stderr=%s", rc, errBuf.String())
	}

	if len(launched) != 1 || len(launched[0]) != 1 || launched[0][0] != "b" {
		t.Fatalf("launched %v, want only wave-1 cycle [b] (wave 0 should be skipped)", launched)
	}

	// status now reports both waves done.
	var sout, serr bytes.Buffer
	if rc := runCampaignStatus([]string{"--plan", planPath, "--project-root", dir}, &sout, &serr); rc != 0 {
		t.Fatalf("runCampaignStatus rc=%d, stderr=%s", rc, serr.String())
	}
	if !strings.Contains(sout.String(), "2/2 waves complete") {
		t.Errorf("status output = %q, want it to report 2/2 waves complete", sout.String())
	}
}
