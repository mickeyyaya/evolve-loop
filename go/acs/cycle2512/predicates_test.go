//go:build acs

package cycle2512

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const rollbackPkg = "github.com/mickeyyaya/evolve-loop/go/internal/rollback"

var contractCases = []string{
	"ls_remote_prints_sha_tab_ref",
	"ls_remote_prints_nothing_for_an_absent_tag",
	"revert_conflict_exits_1_with_CONFLICT",
}

var contractBackends = []string{"stub", "real"}

func runContract(t *testing.T, pattern string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-tags", "integration", "-run", pattern, rollbackPkg)
	cmd.Dir = acsassert.GoModuleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func requirePass(t *testing.T, out, name string) {
	t.Helper()
	if !strings.Contains(out, "--- PASS: "+name+" (") {
		t.Errorf("test %q did not report PASS; output tail:\n%s", name, acsassert.OutputTail(out, 25))
	}
}

func TestC2512_001_ContractTableRunsTheLsRemoteShapeCaseOnTheStub(t *testing.T) {
	out := runContract(t, "^TestGitStubContract$")
	requirePass(t, out, "TestGitStubContract/ls_remote_prints_sha_tab_ref/stub")
}

func TestC2512_002_ContractTableRunsTheLsRemoteShapeCaseOnRealGit(t *testing.T) {
	out := runContract(t, "^TestGitStubContract$")
	requirePass(t, out, "TestGitStubContract/ls_remote_prints_sha_tab_ref/real")
}

func TestC2512_003_ContractTableCoversTheSameCasesOnBothBackends(t *testing.T) {
	out := runContract(t, "^TestGitStubContract$")
	for _, backend := range contractBackends {
		for _, c := range contractCases {
			requirePass(t, out, "TestGitStubContract/"+c+"/"+backend)
		}
	}
}

func TestC2512_004_LegacyBareRefOutputFailsTheShapeCase(t *testing.T) {
	out := runContract(t, "^TestGitStubContract_LegacyOutputFailsTheShapeCase$")
	requirePass(t, out, "TestGitStubContract_LegacyOutputFailsTheShapeCase")
}

func TestC2512_005_RevertConflictReachesTheFailedPathOnStubAndRealGit(t *testing.T) {
	out := runContract(t, "^TestGitStubContract_RevertConflictReachesFailedPath$")
	for _, backend := range contractBackends {
		requirePass(t, out, "TestGitStubContract_RevertConflictReachesFailedPath/"+backend)
	}
}

var shaLessLsRemoteEcho = regexp.MustCompile(`echo[^\n]*refs/tags/`)
var tabSeparator = regexp.MustCompile(`\\t|\t`)

func TestC2512_006_NoRollbackStubEchoesAShaLessLsRemoteRef(t *testing.T) {
	root := acsassert.GoModuleDir(t)
	files, err := filepath.Glob(filepath.Join(root, "internal", "rollback", "*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no rollback test files found: %v", err)
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, raw := range strings.Split(string(body), "\n") {
			line := strings.TrimSpace(raw)
			if shaLessLsRemoteEcho.MatchString(line) && !tabSeparator.MatchString(line) {
				t.Errorf("%s:%d stub echoes a ref without <sha> TAB: %s", filepath.Base(f), i+1, line)
			}
		}
	}
}
