//go:build acs

package cycle1265

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	cmd := exec.Command("go", args...)
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return string(out), code
}

func TestC1265_001_UnifiedBuilderContractIsGreen(t *testing.T) {
	out, code := goTest(t, "test", "-count=1", "-v", "-run", "^TestUnifiedCandidateBuilder", "./internal/llmroute")
	if code != 0 {
		t.Errorf("cycle-1265 C1: `go test -run ^TestUnifiedCandidateBuilder ./internal/llmroute` exit=%d, want 0\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("cycle-1265 C1: the frozen contract matched no tests — candidates_unified_test.go was renamed or deleted (doNotModifyTests)\n%s", out)
	}
	for _, name := range []string{
		"TestUnifiedCandidateBuilder_SharedCore",
		"TestUnifiedCandidateBuilder_ExcludeIsNegative",
		"TestUnifiedCandidateBuilder_ChainForDelegates",
		"TestUnifiedCandidateBuilder_ResolveDelegates",
		"TestUnifiedCandidateBuilder_DispatchWalkerUntouched",
	} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("cycle-1265 C1: missing `--- PASS: %s` — the contract did not run green\n%s", name, out)
		}
	}
}

func TestC1265_002_ExactlyOneCandidateChainBuilder(t *testing.T) {
	root := acsassert.RepoRoot(t)
	llmrouteGo := filepath.Join(root, "go", "internal", "llmroute", "llmroute.go")
	dispatchGo := filepath.Join(root, "go", "internal", "llmroute", "dispatch.go")

	if !acsassert.FileNotContains(t, llmrouteGo, "func candidatesFrom(") {
		t.Errorf("cycle-1265 C2: candidatesFrom is still defined in llmroute.go — the duplicate builder was not removed")
	}
	if !acsassert.FileNotContains(t, dispatchGo, "func chainCandidates(") {
		t.Errorf("cycle-1265 C2: chainCandidates is still defined in dispatch.go — the duplicate builder was not removed")
	}
	defs := acsassert.CountOccurrencesAny(dispatchGo, "func buildCandidates(") +
		acsassert.CountOccurrencesAny(llmrouteGo, "func buildCandidates(")
	if defs != 1 {
		t.Errorf("cycle-1265 C2: found %d definitions of buildCandidates across llmroute.go+dispatch.go, want exactly 1 (single authority)", defs)
	}

	out, code := goTest(t, "test", "-count=1", "-run", "^TestResolve|^TestChainFor|^TestDispatch", "./internal/llmroute")
	if code != 0 {
		t.Errorf("cycle-1265 C2: pre-existing Resolve/ChainFor/Dispatch suite regressed after the dedup, exit=%d\n%s", code, out)
	}
}

func TestC1265_003_ResolveAndChainForCallTheSharedBuilder(t *testing.T) {
	root := acsassert.RepoRoot(t)
	llmrouteGo := filepath.Join(root, "go", "internal", "llmroute", "llmroute.go")
	dispatchGo := filepath.Join(root, "go", "internal", "llmroute", "dispatch.go")

	nResolve, err := acsassert.CountInGoFunc(llmrouteGo, "Resolve", "buildCandidates(")
	if err != nil {
		t.Fatalf("cycle-1265 C3: scanning Resolve in llmroute.go: %v", err)
	}
	if nResolve != 1 {
		t.Errorf("cycle-1265 C3: Resolve calls buildCandidates %d time(s), want exactly 1 (Resolve must delegate, not re-implement)", nResolve)
	}
	nChainFor, err := acsassert.CountInGoFunc(dispatchGo, "ChainFor", "buildCandidates(")
	if err != nil {
		t.Fatalf("cycle-1265 C3: scanning ChainFor in dispatch.go: %v", err)
	}
	if nChainFor != 1 {
		t.Errorf("cycle-1265 C3: ChainFor calls buildCandidates %d time(s), want exactly 1 (ChainFor must delegate, not re-implement)", nChainFor)
	}
}

func TestC1265_004_OutOfScopeSeamsUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	llmrouteGo := filepath.Join(root, "go", "internal", "llmroute", "llmroute.go")
	dispatchGo := filepath.Join(root, "go", "internal", "llmroute", "dispatch.go")

	if !acsassert.FileContains(t, dispatchGo, "func Dispatch(plan Plan, launch func(cli string) (exitCode int, err error)) DispatchResult") {
		t.Errorf("cycle-1265 C4: Dispatch's signature changed — the walker is out of scope for this dedup")
	}
	if !acsassert.FileContains(t, llmrouteGo, "func resolveTriggers(prof *profiles.Profile) []int") {
		t.Errorf("cycle-1265 C4: resolveTriggers' signature changed — it is already single-authority and out of scope")
	}
	out, code := goTest(t, "vet", "./internal/llmroute")
	if code != 0 {
		t.Errorf("cycle-1265 C4: `go vet ./internal/llmroute` exit=%d, want 0\n%s", code, out)
	}
}
