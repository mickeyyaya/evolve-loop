//go:build acs

package cycle1832

import (
	"bufio"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	releasePkg        = "./internal/releasepipeline"
	releasePkgDir     = "internal/releasepipeline"
	preCycleCommit    = "6ac7bf0e6cd15915c0c0aa587efe93c3799facdf"
	releaseRunFile    = "release_run.go"
	releasePipeFile   = "releasepipeline.go"
	maxReportedOutput = 6000
)

func moduleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func tail(s string) string {
	if len(s) <= maxReportedOutput {
		return s
	}
	return "...\n" + s[len(s)-maxReportedOutput:]
}

func goTool(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleDir(t)
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type testEvent struct {
	Action string
	Test   string
	Output string
}

func runPattern(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

func requireTestsPass(t *testing.T, names []string, extraArgs ...string) bool {
	t.Helper()
	args := append([]string{"test", "-count=1", "-json", "-run", runPattern(names)}, extraArgs...)
	out, runErr := goTool(t, append(args, releasePkg)...)
	outcome := map[string]string{}
	var failureOutput strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 0, 1<<16), 1<<22)
	for scanner.Scan() {
		var ev testEvent
		if json.Unmarshal(scanner.Bytes(), &ev) != nil {
			failureOutput.WriteString(scanner.Text() + "\n")
			continue
		}
		failureOutput.WriteString(ev.Output)
		if slices.Contains(names, ev.Test) && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip") {
			outcome[ev.Test] = ev.Action
		}
	}
	isAllPassed := runErr == nil
	for _, name := range names {
		switch got := outcome[name]; got {
		case "pass":
		case "":
			isAllPassed = false
			t.Errorf("RED: %s did not run in %s (absent or renamed)", name, releasePkg)
		default:
			isAllPassed = false
			t.Errorf("RED: %s %s", name, got)
		}
	}
	if !isAllPassed {
		t.Errorf("RED: go %s: %v\n%s", strings.Join(args, " "), runErr, tail(failureOutput.String()))
	}
	return isAllPassed
}

func releaseSource(t *testing.T, file string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(moduleDir(t), releasePkgDir, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return src
}

func funcParamTypes(t *testing.T, file, funcName string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, releaseSource(t, file), parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != funcName {
			continue
		}
		var params []string
		for _, field := range fn.Type.Params.List {
			for range max(1, len(field.Names)) {
				params = append(params, types.ExprString(field.Type))
			}
		}
		return params
	}
	t.Fatalf("RED: %s has no func %s", file, funcName)
	return nil
}

func TestC1832_001_JournalWriteFailureAfterInitSurfacesFromRun(t *testing.T) {
	requireTestsPass(t, []string{"TestRun_JournalWriteError"})
}

func TestC1832_002_InitJournalDropsUnusedFromTag(t *testing.T) {
	want := []string{"Options", "time.Time"}
	if got := funcParamTypes(t, releasePipeFile, "initJournal"); !slices.Equal(got, want) {
		t.Errorf("RED: initJournal parameters = %v, want %v (the fromTag string is never read)", got, want)
	}
	requireTestsPass(t, []string{
		"TestInitJournal_DryRun",
		"TestInitJournal_RealPath",
		"TestInitJournal_JournalDirOverride",
		"TestInitJournal_WriteJournalFails",
		"TestRun_HappyPath",
	})
}

func TestC1832_003_ResolveInitCommitEmptyOutputIsAnErrorWithNoDeadBranch(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "resolve-init-commit.cover")
	if !requireTestsPass(t, []string{
		"TestResolveInitCommit_EmptyGitOutputIsAnError",
		"TestResolveInitCommit_NonGitDir",
		"TestResolveInitCommit_ValidGitRepo",
	}, "-coverprofile="+profile) {
		return
	}
	out, err := goTool(t, "tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "resolveInitCommit" {
			if fields[2] != "100.0%" {
				t.Errorf("RED: resolveInitCommit statement coverage = %s under its own tests, want 100.0%% (an unreachable branch remains)", fields[2])
			}
			return
		}
	}
	t.Errorf("RED: resolveInitCommit absent from the coverage report:\n%s", tail(out))
}

func TestC1832_004_ReleaseClassComputedBeforeShipCommitsPinsTheOrder(t *testing.T) {
	requireTestsPass(t, []string{"TestRun_ReleaseClassComputedBeforeShipCommits"})
}

// acs-predicate: config-check
func TestC1832_005_ClassifyWhyCommentGoneAndNoCommentAdded(t *testing.T) {
	if n := commentaudit.Stats(releaseSource(t, releaseRunFile)).Comment; n != 0 {
		t.Errorf("RED: %s holds %d comment line(s), want 0 (TestRun_ReleaseClassComputedBeforeShipCommits carries the why)", releaseRunFile, n)
	}
	root := acsassert.RepoRoot(t)
	for _, file := range []string{releaseRunFile, releasePipeFile} {
		rel := "go/" + releasePkgDir + "/" + file
		before, err := exec.Command("git", "-C", root, "show", preCycleCommit+":"+rel).Output()
		if err != nil {
			t.Fatalf("read %s at %s: %v", rel, preCycleCommit, err)
		}
		if added := commentaudit.AddedComments(before, releaseSource(t, file)); len(added) != 0 {
			t.Errorf("RED: %s adds comment line(s) %q", rel, added)
		}
	}
}

func TestC1832_006_ReleasepipelinePackageStaysGreen(t *testing.T) {
	if out, err := goTool(t, "test", "-count=1", releasePkg); err != nil {
		t.Errorf("RED: go test %s: %v\n%s", releasePkg, err, tail(out))
	}
}
