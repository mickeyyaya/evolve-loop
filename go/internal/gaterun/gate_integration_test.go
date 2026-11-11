//go:build integration

package gaterun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	gateFlag    = "--gaterun-gate"
	fixturePath = "GATERUN_FIXTURE_PATH"
)

func init() {
	if i := slices.Index(os.Args, gateFlag); i >= 0 {
		if err := Serve(context.Background(), integrationEntry(os.Args[i+1]), os.Stdin, os.Stdout); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
}

func integrationEntry(name string) gatecontract.Entry {
	return gatecontract.Entry{Name: name, Run: func(ctx context.Context, in gatecontract.Input, ports gatecontract.Ports) (gatecontract.Verdict, error) {
		switch name {
		case "gitlog":
			cmd := sysexec.Command(ctx, "git", "log")
			cmd.Dir = in.Root
			if out, err := cmd.CombinedOutput(); err != nil {
				return gatecontract.Verdict{}, fmt.Errorf("git log: %w: %s", err, out)
			}
		case "readpath":
			if _, err := os.ReadFile(os.Getenv(fixturePath)); err != nil {
				return gatecontract.Verdict{}, err
			}
		case "ports":
			files, err := ports.Lane.TrackedFiles(in.Root, ".")
			if err != nil {
				return gatecontract.Verdict{}, err
			}
			tested, err := ports.Toolchain.Test(ctx, gatecontract.Call{Dir: in.Root, Args: []string{"-count=1", "."}})
			if err != nil || tested.ExitCode != 0 {
				return gatecontract.Verdict{}, fmt.Errorf("go test exit %d: %v: %s%s", tested.ExitCode, err, tested.Stdout, tested.Stderr)
			}
			return gatecontract.Verdict{Notes: files}, nil
		}
		return gatecontract.Verdict{Notes: []string{"ran " + name}}, nil
	}}
}

const fixtureTest = `package fixture

import (
	"os/exec"
	"testing"
)

func TestGitInitInATempDir(t *testing.T) {
	if out, err := exec.Command("git", "init", t.TempDir()).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
`

type integrationLane struct{ main, lane, gitdir, common string }

func makeLane(t *testing.T) integrationLane {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go.mod", "module fixture\n\ngo 1.23\n")
	repo.Write("fixture_test.go", fixtureTest)
	repo.Git("add", "-A")
	repo.Git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "fixture")
	main, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	lane := filepath.Join(filepath.Dir(main), "lane")
	repo.Git("worktree", "add", "-q", "-b", "lane", lane)
	return integrationLane{main: main, lane: lane, gitdir: filepath.Join(main, ".git", "worktrees", "lane"), common: filepath.Join(main, ".git")}
}

func requireNestedSandbox(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the gate profile test needs sandbox-exec")
	}
	if probe := sandbox.Probe(); !probe.Capable {
		t.Skipf("nested sandboxing is refused here: %s", probe.Reason)
	}
}

func runIntegrationGate(t *testing.T, lane integrationLane, mode, name, path string) (gatecontract.Verdict, error, *recorder) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pol, err := policy.Parse([]byte(`{"gates":{"hermetic":{"mode":"` + mode + `"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	setup := SetupFor(pol, binary, policyPath)
	setup.Signals, setup.HomeDir = rec, t.TempDir()
	strategy, err := NewStrategy(setup)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{
		Strategy: strategy, Ports: RealPorts(), ScratchParent: t.TempDir(),
		Command: func(ctx context.Context, entry string) *exec.Cmd {
			cmd := sysexec.Command(ctx, binary, gateFlag, entry)
			cmd.Env = append(os.Environ(), fixturePath+"="+path)
			return cmd
		},
	}
	verdict, runErr := Run(context.Background(), gatecontract.Entry{Name: name}, gatecontract.Input{Root: lane.lane}, opts)
	return verdict, runErr, rec
}

func hasViolation(rec *recorder, entry string, match func(signalcenter.Event) bool) bool {
	return slices.ContainsFunc(rec.events, func(e signalcenter.Event) bool {
		return e.Code == CodeViolation && e.Fields["entry"] == entry && match(e)
	})
}

func TestGate_AnEntryThatRunsGitItselfIsRefusedAtEnforceAndReportedAtShadow(t *testing.T) {
	requireNestedSandbox(t)
	lane := makeLane(t)
	if _, err, _ := runIntegrationGate(t, lane, policy.HermeticEnforce, "gitlog", ""); err == nil {
		t.Error("enforce must refuse an entry that starts git")
	}
	verdict, err, rec := runIntegrationGate(t, lane, policy.HermeticShadow, "gitlog", "")
	if err != nil || len(verdict.Notes) == 0 {
		t.Fatalf("shadow lets the entry run: %v %+v", err, verdict)
	}
	execOfGit := func(e signalcenter.Event) bool {
		return strings.HasPrefix(e.Fields["operation"], "process-exec") && filepath.Base(e.Fields["path"]) == "git"
	}
	if !hasViolation(rec, "gitlog", execOfGit) {
		t.Errorf("shadow must report the exec of git with the entry, the operation and the path: %+v", rec.events)
	}
}

func TestGate_AnEntryThatReadsAGitEntryIsRefusedAlthoughTheLaneRootIsGranted(t *testing.T) {
	requireNestedSandbox(t)
	lane := makeLane(t)
	if _, err, _ := runIntegrationGate(t, lane, policy.HermeticEnforce, "readpath", filepath.Join(lane.lane, "go.mod")); err != nil {
		t.Fatalf("the lane root is granted, so go.mod reads: %v", err)
	}
	measured := 0
	for _, path := range []string{filepath.Join(lane.lane, ".git"), filepath.Join(lane.gitdir, "HEAD"), filepath.Join(lane.common, "HEAD")} {
		if _, err, _ := runIntegrationGate(t, lane, policy.HermeticEnforce, "readpath", path); err == nil {
			t.Errorf("enforce must refuse a read of %s", path)
		}
		_, err, rec := runIntegrationGate(t, lane, policy.HermeticShadow, "readpath", path)
		readOfPath := func(e signalcenter.Event) bool { return e.Fields["path"] == path }
		reported, dropped := hasViolation(rec, "readpath", readOfPath), hasNoEvidence(rec)
		if err != nil || reported == dropped {
			t.Errorf("shadow must report %s or say that the sandbox dropped the reports, never neither: err %v events %+v", path, err, rec.events)
		}
		if reported {
			measured++
		}
	}
	if measured == 0 {
		t.Error("no shadow run delivered its reports, so the test proved nothing about the report profile")
	}
}

func hasNoEvidence(rec *recorder) bool {
	return slices.ContainsFunc(rec.events, func(e signalcenter.Event) bool { return e.Code == CodeNoEvidence })
}

func TestGate_ACleanShadowRunReportsNothingBecauseTheChildReadsBothCanaries(t *testing.T) {
	requireNestedSandbox(t)
	lane := makeLane(t)
	for attempt := range 3 {
		_, err, rec := runIntegrationGate(t, lane, policy.HermeticShadow, "readpath", filepath.Join(lane.lane, "go.mod"))
		if err != nil {
			t.Fatalf("a read of go.mod at shadow: %v", err)
		}
		if len(rec.events) == 0 {
			return
		}
		t.Logf("attempt %d: %+v", attempt, rec.events)
	}
	t.Error("three clean shadow runs each gave events: the child must read both canaries and touch nothing outside the grants")
}

func TestGate_AnEntryThatUsesThePortsPassesAtEnforce(t *testing.T) {
	requireNestedSandbox(t)
	lane := makeLane(t)
	verdict, err, _ := runIntegrationGate(t, lane, policy.HermeticEnforce, "ports", "")
	if err != nil || !slices.Contains(verdict.Notes, "go.mod") {
		t.Fatalf("the ports run git and go test in the parent: %v %+v", err, verdict)
	}
}
