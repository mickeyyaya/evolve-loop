package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestCampaignCycleFromWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		want      int
	}{
		{name: "valid", workspace: "/tmp/runs/cycle-17", want: 17},
		{name: "non cycle", workspace: "/tmp/runs/current", want: 0},
		{name: "empty", workspace: "", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cycleFromWorkspace(tt.workspace); got != tt.want {
				t.Fatalf("cycleFromWorkspace(%q) = %d, want %d", tt.workspace, got, tt.want)
			}
		})
	}
}

func TestCampaignRenderMissingPlan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := renderCampaignPlan(filepath.Join(t.TempDir(), "missing.json"), &stdout, &stderr); code != 1 {
		t.Fatalf("renderCampaignPlan code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read plan") {
		t.Fatalf("stderr = %q, want read-plan error", stderr.String())
	}
}

func TestRunCampaign_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCampaign([]string{}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("runCampaign code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "study") {
		t.Fatalf("stderr = %q, want campaign usage", stderr.String())
	}
}

func TestRunCampaign_UnknownSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCampaign([]string{"unknown"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("runCampaign code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want unknown-subcommand error", stderr.String())
	}
}

func TestRenderCampaignPlan_Valid(t *testing.T) {
	path := writeCampaignTestPlan(t)
	var stdout, stderr bytes.Buffer
	if code := renderCampaignPlan(path, &stdout, &stderr); code != 0 {
		t.Fatalf("renderCampaignPlan code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("renderCampaignPlan produced empty stdout on valid plan")
	}
}

func TestCampaignLoadVerifiedPlanPropagatesVerifyError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign-plan.json")
	if err := os.WriteFile(path, []byte(`{"version":0,"goal":"test","cycles":[{"id":"c1"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadVerifiedCampaignPlan(path); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("loadVerifiedCampaignPlan error = %v, want version error", err)
	}
}

func TestCampaignLocalizedRetrySucceeds(t *testing.T) {
	planPath := writeCampaignTestPlan(t)
	attempts := 0
	withCampaignLaunchFactory(t, func(string, bool, string, string, io.Writer, io.Writer) fleet.LaunchFn {
		return func(context.Context, fleet.CycleSpec) (int, error) {
			attempts++
			if attempts == 1 {
				return 1, nil
			}
			return 0, nil
		}
	})

	var stdout, stderr bytes.Buffer
	if code := runCampaignRun([]string{"--plan", planPath, "--project-root", filepath.Dir(planPath)}, &stdout, &stderr); code != 0 {
		t.Fatalf("runCampaignRun code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if attempts != 2 {
		t.Fatalf("launch attempts = %d, want 2", attempts)
	}
}

func TestCampaignLocalizedRetryFails(t *testing.T) {
	planPath := writeCampaignTestPlan(t)
	attempts := 0
	withCampaignLaunchFactory(t, func(string, bool, string, string, io.Writer, io.Writer) fleet.LaunchFn {
		return func(context.Context, fleet.CycleSpec) (int, error) {
			attempts++
			return 1, nil
		}
	})

	var stdout, stderr bytes.Buffer
	if code := runCampaignRun([]string{"--plan", planPath, "--project-root", filepath.Dir(planPath)}, &stdout, &stderr); code != 1 {
		t.Fatalf("runCampaignRun code = %d, want 1", code)
	}
	if attempts != 2 {
		t.Fatalf("launch attempts = %d, want 2", attempts)
	}
}

func TestCampaignRun_FiniteConcurrency(t *testing.T) {
	planPath := writeCampaignTestPlanCount(t, 2)
	var active, maxActive int32
	withCampaignLaunchFactory(t, func(string, bool, string, string, io.Writer, io.Writer) fleet.LaunchFn {
		return func(context.Context, fleet.CycleSpec) (int, error) {
			current := atomic.AddInt32(&active, 1)
			for {
				observed := atomic.LoadInt32(&maxActive)
				if current <= observed || atomic.CompareAndSwapInt32(&maxActive, observed, current) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return 0, nil
		}
	})

	var stdout, stderr bytes.Buffer
	if code := runCampaignRun([]string{"--plan", planPath, "--concurrency", "1", "--project-root", filepath.Dir(planPath)}, &stdout, &stderr); code != 0 {
		t.Fatalf("runCampaignRun code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("maximum concurrent launches = %d, want 1", got)
	}
}

func TestCycleRunArgs_IncludesProjectRoot(t *testing.T) {
	root := t.TempDir()
	got := strings.Join(cycleRunArgs("abc123", "", false, root), " ")
	want := "cycle run --goal-hash abc123 --project-root " + root
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q", got, want)
	}
}

func TestCycleRunArgs_EmptyRootOmitsFlag(t *testing.T) {
	if got := strings.Join(cycleRunArgs("abc123", "", false, ""), " "); strings.Contains(got, "project-root") {
		t.Fatalf("cycleRunArgs empty root = %q, want project-root omitted", got)
	}
}

func withCampaignLaunchFactory(
	t *testing.T,
	factory func(string, bool, string, string, io.Writer, io.Writer) fleet.LaunchFn,
) {
	t.Helper()
	previous := campaignLaunchFactory
	campaignLaunchFactory = factory
	t.Cleanup(func() { campaignLaunchFactory = previous })
}

func writeCampaignTestPlan(t *testing.T) string {
	return writeCampaignTestPlanCount(t, 1)
}

func writeCampaignTestPlanCount(t *testing.T, cycleCount int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign-plan.json")
	cycles := make([]string, cycleCount)
	for i := range cycles {
		cycles[i] = fmt.Sprintf(`{"id":"c%d","files":["file%d.go"]}`, i+1, i+1)
	}
	data := []byte(`{"version":1,"goal":"test","cycles":[` + strings.Join(cycles, ",") + `]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
