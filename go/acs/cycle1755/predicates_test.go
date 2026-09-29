//go:build acs

package cycle1755

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseSHA      = "ad310db6bd35d64a676c4e820186140697d67b76"
	offendersRel = "go/internal/sizeratchet/offenders.json"
	modulePath   = "github.com/mickeyyaya/evolve-loop/go"
)

var shrunkKeys = []string{
	"cmd/evolve.runACSSuite",
	"internal/cyclehealth.Check",
	"pkg/naminguard.Fix",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return acsassert.RepoRoot(t)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "go")
}

func spanLines(t *testing.T, key string) int {
	t.Helper()
	spans, err := sizeratchet.Walk(moduleRoot(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	lines := -1
	for _, s := range spans {
		if s.Key == key && s.Lines > lines {
			lines = s.Lines
		}
	}
	if lines < 0 {
		t.Fatalf("%s not found: the function must keep its name and package, a rename is not a shrink", key)
	}
	return lines
}

func git(root string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func changedSinceBase(t *testing.T, root string, paths ...string) []string {
	t.Helper()
	tracked, err := git(root, append([]string{"diff", "--name-only", "--no-renames", baseSHA, "--"}, paths...)...)
	if err != nil {
		t.Fatal(err)
	}
	untracked, err := git(root, append([]string{"ls-files", "--others", "--exclude-standard", "--full-name", "--"}, paths...)...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked))
}

type gitView struct{ root string }

func (g gitView) ChangedFiles(base string) ([]string, error) {
	tracked, err := git(g.root, "diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := git(g.root, "ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g gitView) Show(base, path string) ([]byte, error) {
	return commentaudit.ReadAtBase(func(args ...string) ([]byte, error) { return git(g.root, args...) }, base)(path)
}

func (g gitView) Root() (string, error) { return g.root, nil }

func loadCurrentOffenders(t *testing.T) map[string]int {
	t.Helper()
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(repoRoot(t), filepath.FromSlash(offendersRel)))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	return offenders
}

func TestC1755_001_RunACSSuiteWithinSizeRatchetLimit(t *testing.T) {
	if lines := spanLines(t, "cmd/evolve.runACSSuite"); lines > sizeratchet.MaxLines {
		t.Errorf("RED: cmd/evolve.runACSSuite is %d lines > %d: extract one cohesive block (flag parsing or verdict reporting) into an unexported helper", lines, sizeratchet.MaxLines)
	}
}

func TestC1755_002_CyclehealthCheckWithinSizeRatchetLimit(t *testing.T) {
	if lines := spanLines(t, "internal/cyclehealth.Check"); lines > sizeratchet.MaxLines {
		t.Errorf("RED: internal/cyclehealth.Check is %d lines > %d", lines, sizeratchet.MaxLines)
	}
}

func TestC1755_003_NaminguardFixWithinSizeRatchetLimit(t *testing.T) {
	if lines := spanLines(t, "pkg/naminguard.Fix"); lines > sizeratchet.MaxLines {
		t.Errorf("RED: pkg/naminguard.Fix is %d lines > %d", lines, sizeratchet.MaxLines)
	}
}

func TestC1755_004_ModuleSizeRatchetCheckPasses(t *testing.T) {
	spans, err := sizeratchet.Walk(moduleRoot(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	if err := sizeratchet.Check(spans, loadCurrentOffenders(t)); err != nil {
		t.Errorf("RED: the module-wide size ratchet must pass: no extracted helper past %d lines, no listed function past its allowance: %v", sizeratchet.MaxLines, err)
	}
}

func TestC1755_005_OffendersJSONDropsTheThreeShrunkEntries(t *testing.T) {
	offenders := loadCurrentOffenders(t)
	for _, key := range shrunkKeys {
		if allowance, listed := offenders[key]; listed {
			t.Errorf("RED: %s still lists %s at %d: the function fits %d lines, so its entry must be removed (an allowance cannot be lowered to %d or fewer)", offendersRel, key, allowance, sizeratchet.MaxLines, sizeratchet.MaxLines)
		}
	}
}

func TestC1755_006_OffendersJSONNeverGainsOrRaisesAnEntry(t *testing.T) {
	root := repoRoot(t)
	body, err := git(root, "show", baseSHA+":"+offendersRel)
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]int
	if err := json.Unmarshal(body, &base); err != nil {
		t.Fatalf("parse %s at base: %v", offendersRel, err)
	}
	for key, allowance := range loadCurrentOffenders(t) {
		baseAllowance, existed := base[key]
		switch {
		case !existed:
			t.Errorf("RED: %s gained %s (%d): a new function past %d lines must be shrunk, never listed", offendersRel, key, allowance, sizeratchet.MaxLines)
		case allowance > baseAllowance:
			t.Errorf("RED: %s raised %s from %d to %d: allowances may only shrink", offendersRel, key, baseAllowance, allowance)
		}
	}
}

func TestC1755_007_RunACSSuiteCLIContractPreserved(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: modulePath + "/cmd/evolve",
		Pattern: "^(TestRunACSSuite_|TestACSSuiteRoot|TestSuiteProjectRoot_)",
		Names: []string{
			"TestRunACSSuite_FlagParseErrorExits10",
			"TestRunACSSuite_MissingOrNonPositiveCycleExits10",
			"TestRunACSSuite_SuiteErrorIsHardFailureExit1",
			"TestRunACSSuite_NoPredicateTreePassesAndWritesVerdict",
			"TestRunACSSuite_JSONFalseSkipsVerdictWrite",
			"TestRunACSSuite_VerdictWriteFailureExits1AfterSummary",
			"TestRunACSSuite_DotRootResolvesActiveWorktreeAndPlaneRoot",
			"TestRunACSSuite_ExplicitRootIsNotOverriddenByActiveWorktree",
			"TestRunACSSuite_RedPredicateExits2AndListsIt",
			"TestACSSuiteRootAutosolve",
			"TestACSSuiteRootFallback",
			"TestACSSuiteRootAutosolve_NullAndTypeMismatch_Amp",
			"TestACSSuiteRootAutosolve_PathPreservation_Amp",
			"TestACSSuiteRootAutosolve_WrongCycle_Amp",
			"TestSuiteProjectRoot_NestedPlaneWorktree",
			"TestSuiteProjectRoot_FallbackToGitDerivation",
		},
	})
}

func TestC1755_008_AlreadyShrunkPackagesLeftUntouched(t *testing.T) {
	if changed := changedSinceBase(t, repoRoot(t), "go/internal/cyclehealth", "go/pkg/naminguard"); len(changed) > 0 {
		t.Errorf("RED: Check (33 lines) and Fix (37 lines) already fit the ratchet at base, so their packages stay byte-identical; changed: %v", changed)
	}
}

func TestC1755_009_NoCommentsAddedUnderCmdEvolve(t *testing.T) {
	root := repoRoot(t)
	var stdout, stderr bytes.Buffer
	args := []string{"comments", "-base", baseSHA, filepath.Join(root, "go", "cmd", "evolve")}
	if code := commentaudit.Main(args, &stdout, &stderr, gitView{root: root}); code != 0 {
		t.Errorf("RED: commentaudit comments exited %d: the extraction must carry its intent in names, not comments\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
}

func TestC1755_010_TouchedFilesGofmtAndVetClean(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), "cmd", "evolve")
	for _, name := range []string{"cmd_acs.go", "cmd_acs_suite_test.go"} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", filepath.Join(dir, name))
		if err != nil || code != 0 {
			t.Fatalf("gofmt -l %s: exit=%d err=%v stderr=%s", name, code, err, stderr)
		}
		if stdout != "" {
			t.Errorf("RED: gofmt reports %s unformatted", name)
		}
	}
	_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", moduleRoot(t), "./cmd/evolve/")
	if err != nil || code != 0 {
		t.Errorf("RED: go vet ./cmd/evolve/ failed: exit=%d err=%v\n%s", code, err, stderr)
	}
}

func TestC1755_011_NoProtectedSurfaceTouched(t *testing.T) {
	for _, path := range changedSinceBase(t, repoRoot(t), ".") {
		if guards.IsProtectedSurface(path) {
			t.Errorf("RED: %s is a protected control-plane surface; a size-ratchet shrink never touches one", path)
		}
	}
}
