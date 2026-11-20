//go:build acs

package cycle2572

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const contractDoc = "docs/architecture/agent-sandbox-contract.md"

var evolveMainPackage = filepath.Join("cmd", "evolve")

func goTestInBridge(t *testing.T, pathPrefix string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test", "-tags", "integration", "-count=1", "-v"}, args...)...)
	cmd.Dir = acsassert.GoModuleDir(t)
	if pathPrefix != "" {
		cmd.Env = append(os.Environ(), "PATH="+pathPrefix+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func countingGoShim(t *testing.T) (shimDir, logPath string) {
	t.Helper()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	shimDir = t.TempDir()
	logPath = filepath.Join(shimDir, "go-calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\nexec '" + realGo + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shimDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return shimDir, logPath
}

func evolveBuildCalls(t *testing.T, logPath string) int {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "build ") && strings.Contains(line, evolveMainPackage) {
			calls++
		}
	}
	return calls
}

func TestC2572_001_TheBuildCountGuardPassesAtOneBuild(t *testing.T) {
	out, err := goTestInBridge(t, "", "-run", "^TestEvolveBuildRunsOnce$", "./internal/bridge")
	if err != nil || !strings.Contains(out, "--- PASS: TestEvolveBuildRunsOnce") {
		t.Fatalf("RED: the build-count guard must run and pass: %v\n%s", err, acsassert.OutputTail(out, 20))
	}
}

func TestC2572_002_TheContractTestsOfTheBinaryTakeOneBuildTogether(t *testing.T) {
	shimDir, logPath := countingGoShim(t)
	out, err := goTestInBridge(t, shimDir, "-run",
		"^(TestAgentSandboxContract|TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead|TestAgentSandboxContract_TheSelfhealOrchestratorSeatHoldsN3AndN4)$",
		"./internal/bridge")
	if err != nil {
		t.Fatalf("the contract tests must pass: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
	if strings.Contains(out, "--- SKIP: TestAgentSandboxContract_TheHostHolds") {
		t.Skip("the agent sandbox is unavailable on this host; the build count is not observable")
	}
	for _, name := range []string{"TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead", "TestAgentSandboxContract_TheSelfhealOrchestratorSeatHoldsN3AndN4"} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Fatalf("%s did not pass in the counted run\n%s", name, acsassert.OutputTail(out, 30))
		}
	}
	if calls := evolveBuildCalls(t, logPath); calls > 1 {
		t.Errorf("RED: the three contract tests of one test binary took %d go build calls of the evolve main package; want at most 1", calls)
	}
}

func TestC2572_003_TheContractDocNamesTheSingleBuildAndPassesSte(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, contractDoc)
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"TestEvolveBuildRunsOnce", "once"} {
		if !strings.Contains(string(raw), needle) {
			t.Errorf("RED: %s must state that the test binary builds the evolve binary once and name the guard %q", contractDoc, needle)
		}
	}
	cmd := exec.Command("go", "run", "./"+evolveMainPackage, "docs", "ste-lint", "--strict", "--project-root", root, contractDoc)
	cmd.Dir = acsassert.GoModuleDir(t)
	out, lintErr := cmd.CombinedOutput()
	if lintErr != nil || !strings.Contains(string(out), "0 finding(s)") {
		t.Errorf("RED: %s must pass ste-lint --strict: %v\n%s", contractDoc, lintErr, out)
	}
}
