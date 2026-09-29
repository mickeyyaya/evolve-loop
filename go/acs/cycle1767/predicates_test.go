//go:build acs

// Package cycle1767 pins the shrinks of phasesCreate, phasesValidate and
// runPhaseVerify in go/internal/cli/phasecmd to the size ratchet.
package cycle1767

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseCommit   = "e404b914d41968fc2ba4124694c46f5dd7826b7b"
	offendersRel = "go/internal/sizeratchet/offenders.json"
	phasecmdDir  = "internal/cli/phasecmd"
)

var targetKeys = map[string]int{
	phasecmdDir + ".phasesCreate":   123,
	phasecmdDir + ".phasesValidate": 69,
	phasecmdDir + ".runPhaseVerify": 74,
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel))
}

func walkModule(t *testing.T) []sizeratchet.FuncSpan {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	return spans
}

func TestC1767_001_ThreeTargetFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for key := range targetKeys {
		n, found := sizes[key]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", key, phasecmdDir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, key+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1767_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling and this lane never edits the file; a shrunk function's entry is slack the boundary tighten removes:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for key, allowance := range targetKeys {
		if got, listed := offenders[key]; !listed || got != allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", key, got, listed, allowance)
		}
	}
}

func TestC1767_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1767_004_TargetPackageTestsPass(t *testing.T) {
	if r := execGo(goModuleDir(t), "test", "-count=1", "./"+phasecmdDir); r.err != nil || r.code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", phasecmdDir, r.code, r.err, r.out)
	}
}

func TestC1767_005_NoCommentLinesAdded(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	args := []string{"comments", "-base", baseCommit}
	scoped := false
	for _, rel := range changed {
		if strings.HasPrefix(rel, "go/"+phasecmdDir+"/") && strings.HasSuffix(rel, ".go") {
			scoped = true
			break
		}
	}
	if !scoped {
		return
	}
	args = append(args, filepath.Join(goModuleDir(t), phasecmdDir))
	var out, errOut strings.Builder
	if code := commentaudit.Main(args, &out, &errOut, git); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent):\n%s%s", code, phasecmdDir, out.String(), errOut.String())
	}
}

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return goRun{out: string(out)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return goRun{out: string(out), err: fmt.Errorf("go %s: %w", strings.Join(args, " "), err)}
	}
	return goRun{out: string(out), code: exitErr.ExitCode()}
}

type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }
