//go:build acs

package cycle2654

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseSHA      = "92ab6246c0753563d4fdce0eba19c4c1ce8b6982"
	modulePrefix = "github.com/mickeyyaya/evolve-loop/go/"
	laneTests    = "^TestLaneIdentity_"
)

type focusedRun struct {
	out     string
	profile string
}

var focusedRuns = map[string]focusedRun{}

func focused(t *testing.T, pkg string) focusedRun {
	t.Helper()
	if run, ok := focusedRuns[pkg]; ok {
		return run
	}
	profilePath := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go",
		"test", "-count=1", "-v", "-run", laneTests, "-coverprofile="+profilePath, "./"+pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -run %s ./%s exit=%d err=%v\n%s", laneTests, pkg, code, err, acsassert.OutputTail(out+errOut, 40))
	}
	profile, readErr := os.ReadFile(profilePath)
	if readErr != nil {
		t.Fatalf("no cover profile for ./%s: %v", pkg, readErr)
	}
	run := focusedRun{out: out, profile: string(profile)}
	focusedRuns[pkg] = run
	return run
}

func TestC2654_001_AComposedCheckpointRecordsThePoolTheLaneCycleAndTheScope(t *testing.T) {
	run := focused(t, "internal/checkpoint")
	required := []string{
		"TestLaneIdentity_ComposeRecordsThePoolTheLaneCycleAndTheScopeOfAFleetLane",
		"TestLaneIdentity_ALanePinWithoutAPoolIdKeepsItsScopeAndNoPool",
		"TestLaneIdentity_EveryComposeVariantCarriesTheIdentity",
		"TestLaneIdentity_TheBoundaryCheckpointersWriteTheIdentityIntoTheStateFile",
		"TestLaneIdentity_BindLaneRestoresTheBindingOfItsCaller",
		"TestLaneIdentity_ABoundPoolMakesTheCycleALaneEvenWithoutAPin",
	}
	for _, name := range required {
		if !strings.Contains(run.out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
	if strings.Contains(run.out, "--- FAIL") {
		t.Errorf("a test failed:\n%s", acsassert.OutputTail(run.out, 30))
	}
}

func TestC2654_002_ASequentialCheckpointStaysWithoutIdentitySoALegacyFileIsDetectable(t *testing.T) {
	run := focused(t, "internal/checkpoint")
	required := []string{"TestLaneIdentity_ComposeLeavesASequentialCycleWithoutAnyLaneIdentity"}
	for _, name := range required {
		if !strings.Contains(run.out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
	if strings.Contains(run.out, "--- FAIL") {
		t.Errorf("a test failed:\n%s", acsassert.OutputTail(run.out, 30))
	}
}

func TestC2654_003_TheResumePointReadsTheIdentityBackAndTheCycleRunBindsItsPool(t *testing.T) {
	core := focused(t, "internal/core")
	cmd := focused(t, "cmd/evolve")
	runs := []struct {
		out      string
		required []string
	}{
		{core.out, []string{
			"TestLaneIdentity_LoadResumeStateReadsTheIdentityOfALaneCheckpoint",
			"TestLaneIdentity_LoadResumeStateReadsNoIdentityFromALegacyCheckpoint",
		}},
		{cmd.out, []string{"TestLaneIdentity_TheCycleRunBindsItsPoolIdForEveryCheckpointItWritesAndReleasesItAtTheEnd"}},
	}
	for _, run := range runs {
		for _, name := range run.required {
			if !strings.Contains(run.out, "--- PASS: "+name+" ") {
				t.Errorf("%s did not run and pass", name)
			}
		}
		if strings.Contains(run.out, "--- FAIL") {
			t.Errorf("a test failed:\n%s", acsassert.OutputTail(run.out, 30))
		}
	}
}

func TestC2654_004_AResumedLaneCheckpointAtEnforceIsWiredToDeliverToTheWaveRecordAndNotToMain(t *testing.T) {
	cmd := focused(t, "cmd/evolve")
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go",
		"test", "-tags", "integration", "-count=1", "-v", "-run", "^TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord$", "./internal/phases/ship")
	if err != nil || code != 0 {
		t.Fatalf("the ship delivery proof failed: exit=%d err=%v\n%s", code, err, acsassert.OutputTail(out+errOut, 30))
	}
	runs := []struct {
		out      string
		required []string
	}{
		{cmd.out, []string{
			"TestLaneIdentity_AResumedLaneCheckpointAtEnforceTakesItsPoolAndScopeFromTheCheckpoint",
			"TestLaneIdentity_AResumedLaneCheckpointAtShadowTakesItsPoolAndScopeFromTheCheckpoint",
			"TestLaneIdentity_AnEnforceLaneWiredWithAPoolAndAScopeBuildsItsOrchestrator",
		}},
		{out, []string{"TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord"}},
	}
	for _, run := range runs {
		for _, name := range run.required {
			if !strings.Contains(run.out, "--- PASS: "+name+" ") {
				t.Errorf("%s did not run and pass", name)
			}
		}
		if strings.Contains(run.out, "--- FAIL") {
			t.Errorf("a test failed:\n%s", acsassert.OutputTail(run.out, 30))
		}
	}
}

func TestC2654_005_ALegacyLaneCheckpointAtEnforceIsRefusedAndOtherResumesKeepWorking(t *testing.T) {
	cmd := focused(t, "cmd/evolve")
	required := []string{
		"TestLaneIdentity_ALegacyLaneCheckpointAtEnforceIsRefusedBeforeAnyPhaseRuns",
		"TestLaneIdentity_AnEnforceLaneWiredWithAScopeAndNoPoolStopsBeforeAnyDispatch",
		"TestLaneIdentity_ALegacyLaneCheckpointAtShadowStillResumesAsBefore",
		"TestLaneIdentity_ASequentialCheckpointAtEnforceStillResumesWithoutAPool",
	}
	for _, name := range required {
		if !strings.Contains(cmd.out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
	if strings.Contains(cmd.out, "--- FAIL") {
		t.Errorf("a test failed:\n%s", acsassert.OutputTail(cmd.out, 30))
	}
}

type lineSet map[int]bool

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func productionGoFile(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "acs/")
}

func gitLines(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutputIn(dir, "git", args...)
	if err != nil || code != 0 {
		t.Fatalf("git %s exit=%d err=%v\n%s", strings.Join(args, " "), code, err, errOut)
	}
	return strings.Split(strings.TrimSpace(out), "\n")
}

func changedProductionLines(t *testing.T) map[string]lineSet {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	changed := map[string]lineSet{}
	for _, rel := range gitLines(t, mod, "diff", "--name-only", "--relative", baseSHA, "--", ".") {
		if !productionGoFile(rel) {
			continue
		}
		set := lineSet{}
		for _, line := range gitLines(t, mod, "diff", "-U0", "--relative", baseSHA, "--", rel) {
			m := hunkHeader.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			for i := 0; i < count; i++ {
				set[start+i] = true
			}
		}
		changed[rel] = set
	}
	for _, rel := range gitLines(t, mod, "ls-files", "--others", "--exclude-standard", "--", ".") {
		if !productionGoFile(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mod, rel))
		if err != nil {
			t.Fatal(err)
		}
		set := lineSet{}
		for i := 1; i <= strings.Count(string(data), "\n")+1; i++ {
			set[i] = true
		}
		changed[rel] = set
	}
	return changed
}

func requireIdentityPlumbingChanged(t *testing.T, changed map[string]lineSet) {
	t.Helper()
	for _, rel := range []string{"internal/checkpoint/checkpoint.go"} {
		if len(changed[rel]) == 0 {
			t.Fatalf("%s has no change against %s: the lane identity is not built yet", rel, baseSHA[:9])
		}
	}
}

var coverBlock = regexp.MustCompile(`^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

func TestC2654_006_EveryChangedProductionLineIsCoveredByTheLaneIdentityTests(t *testing.T) {
	changed := changedProductionLines(t)
	requireIdentityPlumbingChanged(t, changed)
	packages := map[string]bool{}
	for rel := range changed {
		packages[filepath.ToSlash(filepath.Dir(rel))] = true
	}
	var dirs []string
	for dir := range packages {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		for _, line := range strings.Split(focused(t, dir).profile, "\n") {
			m := coverBlock.FindStringSubmatch(line)
			if m == nil || m[4] != "0" {
				continue
			}
			rel := strings.TrimPrefix(m[1], modulePrefix)
			set := changed[rel]
			from, _ := strconv.Atoi(m[2])
			to, _ := strconv.Atoi(m[3])
			for l := from; l <= to; l++ {
				if set[l] {
					t.Errorf("%s:%d changed and not covered by a TestLaneIdentity_ test", rel, l)
					break
				}
			}
		}
	}
}

func TestC2654_007_NoChangedProductionLineCarriesACodeComment(t *testing.T) {
	changed := changedProductionLines(t)
	requireIdentityPlumbingChanged(t, changed)
	mod := acsassert.GoModuleDir(t)
	for rel, set := range changed {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(mod, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, group := range file.Comments {
			if group == file.Doc {
				continue
			}
			checkGroup(t, fset, rel, group, set)
		}
	}
}

func checkGroup(t *testing.T, fset *token.FileSet, rel string, group *ast.CommentGroup, set lineSet) {
	t.Helper()
	for _, c := range group.List {
		if strings.HasPrefix(c.Text, "//go:") {
			continue
		}
		from, to := fset.Position(c.Pos()).Line, fset.Position(c.End()).Line
		for l := from; l <= to; l++ {
			if set[l] {
				t.Errorf("%s:%d carries the code comment %q", rel, l, c.Text)
				break
			}
		}
	}
}
