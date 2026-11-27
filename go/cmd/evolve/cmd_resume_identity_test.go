package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestResumeIdentityConfig_DefaultsToShadowAndReadsThePolicyStage(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]config.Stage{
		`{}`: config.StageShadow,
		`{"resume":{"identity":{"stage":"off"}}}`:     config.StageOff,
		`{"resume":{"identity":{"stage":"enforce"}}}`: config.StageEnforce,
	} {
		pol, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if got := resumeIdentityConfig(pol).Stage; got != want {
			t.Errorf("%s: stage %v, want %v", raw, got, want)
		}
	}
}

func TestResumeIdentityConfig_ComputeReadsTheRegistryAndThePolicyOfTheRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, []byte(`{"phases":["scout"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	compute := resumeIdentityConfig(policy.Policy{}).Compute
	first, err := compute(root)
	if err != nil || first.Graph == "" || first.Contract == "" || first.Policy == "" {
		t.Fatalf("identity %+v, %v", first, err)
	}
	if err := os.WriteFile(registry, []byte(`{"phases":["build"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := compute(root)
	if err != nil || second.Graph == first.Graph {
		t.Fatalf("the graph hash did not follow the registry: %+v %+v %v", first, second, err)
	}
}

func TestCycleResume_AcceptIdentityChangeReachesTheLoopConfig(t *testing.T) {
	root := newLoopStatusRoot(t)
	pausedCycle(t, pausedAtBuild, nil)
	launches := installRecordingLoopLauncher(t)
	var stdout, stderr bytes.Buffer
	if rc := runCycle([]string{"resume", "--cycle", "41", "--accept-identity-change", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	if len(*launches) != 1 || !(*launches)[0].AcceptIdentityChange {
		t.Fatalf("launches = %+v, want the migration flag set", *launches)
	}
	plain := installRecordingLoopLauncher(t)
	if rc := runCycle([]string{"resume", "--cycle", "41", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK || (*plain)[0].AcceptIdentityChange {
		t.Fatalf("a plain resume must not carry the flag: rc=%d %+v", rc, *plain)
	}
}

func TestResumeLoopArgs_AddsTheMigrationFlagOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	if got := resumeLoopArgs("/r", false); len(got) != 3 {
		t.Errorf("plain args = %v", got)
	}
	if got := resumeLoopArgs("/r", true); len(got) != 4 || got[3] != "--accept-identity-change" {
		t.Errorf("migration args = %v", got)
	}
}

func TestParseLoopArgs_AcceptIdentityChangeFlag(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--resume", "--accept-identity-change", "--project-root", t.TempDir()}, &stderr)
	if rc != 0 || !cfg.AcceptIdentityChange {
		t.Fatalf("rc=%d cfg=%+v stderr=%s", rc, cfg, stderr.String())
	}
}

type requestCapturingRunner struct{ got *core.CycleRequest }

func (r requestCapturingRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return core.CycleResult{}, errors.New("RunCycle is not a resume")
}

func (r requestCapturingRunner) RunCycleFromPhase(_ context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	*r.got = req
	return core.CycleResult{Cycle: 7}, core.ErrAllFamiliesExhausted
}

func (requestCapturingRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func TestRunResumeBatch_TheMigrationFlagReachesTheCycleRequest(t *testing.T) {
	for _, accept := range []bool{true, false} {
		root, evolveDir := seedQuotaPause(t, 0)
		var got core.CycleRequest
		var out, errOut bytes.Buffer
		lr := loopResult{}
		cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true, AcceptIdentityChange: accept}
		runResumeBatch(context.Background(), cfg, requestCapturingRunner{got: &got}, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)
		if got.AcceptIdentityChange != accept {
			t.Errorf("accept=%v: request carries %v\n%s", accept, got.AcceptIdentityChange, errOut.String())
		}
	}
}
