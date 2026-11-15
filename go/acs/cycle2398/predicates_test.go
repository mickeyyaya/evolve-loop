//go:build acs

package cycle2398

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg      = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	guardTest         = "TestModuleScanGuard"
	loaderFile        = "go/cmd/evolve/module_scan_loader_test.go"
	cmdEvolveDoc      = "docs/architecture/packages/cmd-evolve.md"
	secondsBudget     = 5.0
	inboxWritersTest  = "TestInboxWriters_EveryAutofiledItemCarriesAKnownClass"
	ledgerRootsTest   = "TestUnobservedLedgerRootsArePinned"
	ledgerRootsSource = "go/cmd/evolve/cmd_cycle_ledger_roots_test.go"
	writersScanSource = "go/cmd/evolve/inbox_writers_scan_test.go"
)

var passLine = regexp.MustCompile(`--- PASS: (\S+) \(([0-9.]+)s\)`)

func runCmdEvolveTests(t *testing.T, pattern string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, evolveCmdPkg)
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC2398_001_TheGuardFailsOnAnyCmdEvolveTestFileThatWalksTheModuleOutsideTheSharedLoader(t *testing.T) {
	out, err := runCmdEvolveTests(t, "^"+guardTest+"$")
	if err != nil || !strings.Contains(out, "--- PASS: "+guardTest+" ") {
		t.Fatalf("RED: %s did not pass (err=%v)\n%s", guardTest, err, acsassert.OutputTail(out, 25))
	}
}

func TestC2398_002_TheSharedLoaderFileIsTheOneHomeOfTheModuleWalkAndIsTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, loaderFile)) {
		t.Fatalf("RED: %s missing on disk", loaderFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", loaderFile); code != 0 {
		t.Errorf("RED: %s untracked — may be gitignored (dropped at ship)", loaderFile)
	}
	walks := acsassert.CountOccurrencesAny(filepath.Join(root, loaderFile), "filepath.WalkDir(", "filepath.Walk(", "fs.WalkDir(")
	if walks != 1 {
		t.Errorf("RED: the loader holds %d walk call(s), want exactly 1: one walk per test binary", walks)
	}
	if acsassert.CountOccurrencesAny(filepath.Join(root, loaderFile), "sync.OnceValue", "sync.Once") == 0 {
		t.Errorf("RED: the loader does not memoize its walk with sync.Once or sync.OnceValue")
	}
}

func TestC2398_003_TheTwoNamedTestsRunUnderFiveSecondsAndNoLongerWalkTheModuleThemselves(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{ledgerRootsSource, writersScanSource} {
		acsassert.FileNotContains(t, filepath.Join(root, rel), "filepath.WalkDir(")
	}
	out, err := runCmdEvolveTests(t, "^("+inboxWritersTest+"|"+ledgerRootsTest+")$")
	if err != nil {
		t.Fatalf("RED: go test exited with %v\n%s", err, acsassert.OutputTail(out, 40))
	}
	seconds := map[string]float64{}
	for _, m := range passLine.FindAllStringSubmatch(out, -1) {
		value, perr := strconv.ParseFloat(m[2], 64)
		if perr != nil {
			t.Fatal(perr)
		}
		seconds[m[1]] = value
	}
	for _, name := range []string{inboxWritersTest, ledgerRootsTest} {
		got, ran := seconds[name]
		if !ran {
			t.Errorf("RED: %s did not report a pass\n%s", name, acsassert.OutputTail(out, 40))
			continue
		}
		if got >= secondsBudget {
			t.Errorf("RED: %s took %.2fs, want under %.0fs", name, got, secondsBudget)
		}
	}
}

func TestC2398_004_TheDocOfTheSharedLoaderNamesTheLoaderAndTheGuardAndEveryLineAboutThemPassesSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	path := filepath.Join(root, cmdEvolveDoc)
	findings, err := stelint.LintFile(path, stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", cmdEvolveDoc, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Split(string(raw), "\n")
	named := map[string]int{"module_scan_loader_test.go": 0, guardTest: 0}
	for i, line := range body {
		for term := range named {
			if !strings.Contains(line, term) {
				continue
			}
			named[term]++
			for _, f := range findings {
				if f.Line == i+1 {
					t.Errorf("RED: %s:%d names %s and fails STE: %v", cmdEvolveDoc, i+1, term, f)
				}
			}
		}
	}
	for term, lines := range named {
		if lines == 0 {
			t.Errorf("RED: %s has no line that names %s", cmdEvolveDoc, term)
		}
	}
}
