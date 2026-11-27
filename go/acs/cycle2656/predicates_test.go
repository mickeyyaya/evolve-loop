//go:build acs

package cycle2656

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cardRel      = "docs/architecture/integration-driver-enforce.md"
	missingTest  = "TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts"
	journeyTest  = "TestProductionEnforceActor_AnEjectedMemberLeavesDeliveredForTheQueueWithItsBrief"
	briefFailure = "TestProductionEnforceActor_ABriefThatCannotBeWrittenKeepsTheItemDelivered"
	promptTest   = "TestInboxBatchesSection_AnItemWithABriefPathShowsThePathToTheNextLane"
)

func runNamedTest(t *testing.T, pkg, name string) {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exited %d: %v\n%s", pkg, name, code, err, acsassert.OutputTail(out+errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("RED: %s in %s did not run and pass\n%s", name, pkg, acsassert.OutputTail(out, 20))
	}
}

func TestC2656_001_ProductionPortsLeaveOnlyPromoteMissingAtEnforce(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", missingTest)
}

func TestC2656_002_AConflictingMemberLeavesDeliveredForTheQueueWithTheBriefPathThroughTheProductionPorts(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", journeyTest)
}

func TestC2656_003_ABriefThatCannotBeWrittenKeepsTheItemDeliveredAndReportsTheBriefStep(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", briefFailure)
}

func TestC2656_004_TheNextLanePromptShowsTheBriefPathOfARequeuedItemAndNoneForAPlainItem(t *testing.T) {
	runNamedTest(t, "./internal/phases/triage", promptTest)
}

func cardSection(t *testing.T, from, until string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), cardRel))
	if err != nil {
		t.Fatalf("read %s: %v", cardRel, err)
	}
	text := string(raw)
	start := strings.Index(text, "\n"+from)
	end := strings.Index(text, "\n"+until)
	if start < 0 || end < start {
		t.Fatalf("%s has no section between %q and %q", cardRel, from, until)
	}
	return text[start:end]
}

func TestC2656_005_CardSectionsTwoAndFourStateTheKeptPortsAndPassTheSteLint(t *testing.T) {
	sections := map[string]string{
		"2": cardSection(t, "## 2. EnforceActor", "## 3. F2"),
		"4": cardSection(t, "## 4. F3", "## 5. F11"),
	}
	for num, text := range sections {
		for _, kept := range []string{"Requeuer", "BriefStore"} {
			if !strings.Contains(text, kept) {
				t.Errorf("RED: card section %s never names the kept port %s", num, kept)
			}
		}
		for _, stale := range []string{"are removed", "No `Requeue` port", "no `Briefs` port"} {
			if strings.Contains(text, stale) {
				t.Errorf("RED: card section %s still says %q although the ports are kept", num, stale)
			}
		}
	}
	if !strings.Contains(sections["4"], "Ejection.WriteBrief") || !strings.Contains(sections["4"], "BriefPath") {
		t.Errorf("RED: card section 4 must tie BriefStore to Ejection.WriteBrief and the item field BriefPath")
	}
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for num, text := range sections {
		for _, f := range stelint.Check([]byte(text), stelint.Options{Words: standard}) {
			if !f.IsSkipNotice() {
				t.Error(fmt.Sprintf("card section %s line %d [%s] %s", num, f.Line, f.Rule, f.Message))
			}
		}
	}
}

var addedHunk = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

func addedProductionLines(t *testing.T, root string) map[string]map[int]bool {
	t.Helper()
	base := "HEAD"
	for _, ref := range []string{"refs/heads/main", "refs/remotes/origin/main"} {
		if out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			base = strings.TrimSpace(out)
			break
		}
	}
	added := map[string]map[int]bool{}
	keep := func(rel string) bool {
		return strings.HasPrefix(rel, "go/") && strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "go/acs/")
	}
	diff, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", "--no-color", base, "--")
	var file string
	for _, line := range strings.Split(diff, "\n") {
		if rel, ok := strings.CutPrefix(line, "+++ b/"); ok {
			file = ""
			if keep(rel) {
				file = rel
				added[file] = map[int]bool{}
			}
			continue
		}
		m := addedHunk.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		from, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for n := from; n < from+count; n++ {
			added[file][n] = true
		}
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard")
	for _, rel := range strings.Fields(untracked) {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if !keep(rel) || err != nil {
			continue
		}
		added[rel] = map[int]bool{}
		for n := 1; n <= strings.Count(string(raw), "\n")+1; n++ {
			added[rel][n] = true
		}
	}
	return added
}

var coverBlock = regexp.MustCompile(`^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

func uncoveredAddedLines(t *testing.T, root, pkg string, added map[string]map[int]bool) []string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-coverprofile="+profile, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s exited %d: %v\n%s", pkg, code, err, acsassert.OutputTail(out+errOut, 30))
	}
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	var gaps []string
	for _, line := range strings.Split(string(raw), "\n") {
		m := coverBlock.FindStringSubmatch(line)
		if m == nil || m[4] != "0" {
			continue
		}
		rel := "go/" + strings.TrimPrefix(m[1], modulePath)
		from, _ := strconv.Atoi(m[2])
		to, _ := strconv.Atoi(m[3])
		for n := from; n <= to; n++ {
			if added[rel][n] {
				gaps = append(gaps, fmt.Sprintf("%s:%d", rel, n))
				break
			}
		}
	}
	return gaps
}

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

func TestC2656_006_EveryAddedProductionLineOutsideTheCmdPackageIsCoveredByItsPackageTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	added := addedProductionLines(t, root)
	packages := map[string]bool{}
	for rel := range added {
		if dir := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, "go/"))); dir != "cmd/evolve" {
			packages["./"+dir] = true
		}
	}
	if len(packages) == 0 {
		t.Fatalf("RED: no production Go file outside cmd/evolve changed; the ports and the item field live in internal packages")
	}
	for pkg := range packages {
		for _, gap := range uncoveredAddedLines(t, root, pkg, added) {
			t.Errorf("RED: %s: the added line %s is not covered, want 100%% on each changed function", pkg, gap)
		}
	}
}
