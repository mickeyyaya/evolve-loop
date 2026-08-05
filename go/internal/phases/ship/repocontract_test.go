package ship

// repocontract_test.go — the ship-time repo-contract scanner pack. The gate
// exists because four lane landings redded main in one week; these tests pin:
// off skips, enforce-green passes, enforce-RED fails with the DEDICATED code
// (never a git-failure alias), unknown stage fails toward enforce, and the
// module dir is the lane worktree's go/.

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func swapRepoContractTest(t *testing.T, err error) *[]string {
	t.Helper()
	var dirs []string
	prev := repoContractTestFn
	t.Cleanup(func() { repoContractTestFn = prev })
	repoContractTestFn = func(ctx context.Context, moduleDir string, stderr io.Writer) error {
		dirs = append(dirs, moduleDir)
		return err
	}
	return &dirs
}

func TestRepoContractGate_OffSkips(t *testing.T) {
	dirs := swapRepoContractTest(t, errors.New("would fail"))
	for _, stage := range []string{"", "off"} {
		if err := runRepoContractGate(context.Background(), stage, "/lane", io.Discard); err != nil {
			t.Fatalf("stage %q must skip, got %v", stage, err)
		}
	}
	if len(*dirs) != 0 {
		t.Fatalf("off must not run the pack, ran in %v", *dirs)
	}
}

func TestRepoContractGate_EnforceGreenPasses(t *testing.T) {
	dirs := swapRepoContractTest(t, nil)
	if err := runRepoContractGate(context.Background(), "enforce", "/lane/worktree", io.Discard); err != nil {
		t.Fatalf("green pack must pass: %v", err)
	}
	if len(*dirs) != 1 || (*dirs)[0] != "/lane/worktree/go" {
		t.Fatalf("pack must run in the lane worktree module dir, got %v", *dirs)
	}
}

func TestRepoContractGate_EnforceRedFailsWithDedicatedCode(t *testing.T) {
	swapRepoContractTest(t, errors.New("exit status 1"))
	err := runRepoContractGate(context.Background(), "enforce", "/lane", io.Discard)
	if err == nil {
		t.Fatal("RED pack must fail the ship")
	}
	var se *shiperr.ShipError
	if !errors.As(err, &se) {
		t.Fatalf("must be a structured ShipError, got %T: %v", err, err)
	}
	if se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("code = %q, want REPO_CONTRACT_GATE (never a git-failure alias)", se.Code)
	}
}

func TestRepoContractGate_UnknownStageFailsTowardEnforce(t *testing.T) {
	dirs := swapRepoContractTest(t, nil)
	var warn strings.Builder
	if err := runRepoContractGate(context.Background(), "shadwo", "/lane", &warn); err != nil {
		t.Fatalf("unknown stage with green pack: %v", err)
	}
	if len(*dirs) != 1 {
		t.Fatal("unknown stage must RUN the pack (typo must not disable a red-main guard)")
	}
	if !strings.Contains(warn.String(), "unknown stage") {
		t.Fatalf("unknown stage must WARN, got %q", warn.String())
	}
}

// TestNew_ThreadsRepoContractGate is the cycle-1064 anti-trap: the production
// construction site must thread the dial or the gate is permanently off no
// matter what policy says.
func TestNew_ThreadsRepoContractGate(t *testing.T) {
	p := New(Config{RepoContractGate: "enforce"})
	if p.repoContractGate != "enforce" {
		t.Fatalf("Config.RepoContractGate must thread into the Phase, got %q", p.repoContractGate)
	}
	_ = os.Stderr // keep os import parallel with production file expectations
}
