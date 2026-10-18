//go:build acs

package cycle1944

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	coreBatchTestPattern  = "^TestCoreCoverageBatch_"
	coverageBatchesParent = "core-package-coverage-batches-after-1942"
	coverageUmbrella      = "core-package-coverage-to-100"
	priorBatchesParent    = "core-package-coverage-remaining-batches"
)

var coreBatchFunctions = []string{
	"lanescope.go:materializeLaneScope",
	"lanescope.go:scoutReportGoalHash",
	"size_budget.go:buildBudgetScale",
	"disposition_seed.go:SeedDispositionSkeleton",
}

func sourceRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(ipcenv.WorktreeRootKey); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func funcCoverage(report string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		file := filepath.Base(strings.SplitN(fields[0], ":", 2)[0])
		out[file+":"+fields[1]] = fields[len(fields)-1]
	}
	return out
}

func TestC1944_001_CoreLaneScopeSizeBudgetAndSeedBatchReachesFullCoverage(t *testing.T) {
	goDir := filepath.Join(sourceRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "core-batch.cov")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1",
		"-run", coreBatchTestPattern, "-coverprofile="+profile, "./internal/core")
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/core: code=%d err=%v\n%s%s", coreBatchTestPattern, code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no tests to run") {
		t.Fatalf("no internal/core test matches %s — the batch's behavior tests are absent", coreBatchTestPattern)
	}
	report, rerr, rcode, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if rcode != 0 {
		t.Fatalf("go tool cover -func: code=%d err=%v\n%s", rcode, err, rerr)
	}
	got := funcCoverage(report)
	for _, fn := range coreBatchFunctions {
		pct, ok := got[fn]
		if !ok {
			t.Errorf("%s is absent from the coverage report — renamed or removed instead of tested", fn)
			continue
		}
		if pct != "100.0%" {
			t.Errorf("%s is at %s under the %s batch tests; want 100.0%% (one behavior test per uncovered branch, error branches included)", fn, pct, coreBatchTestPattern)
		}
	}
}

func TestC1944_002_UnmetCoverageRemainderIsQueuedAsTrackedInboxItem(t *testing.T) {
	src := sourceRoot(t)
	inboxDir := filepath.Join(src, ".evolve", "inbox")
	items, _, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s): %v", inboxDir, err)
	}
	parents := map[string]bool{coverageBatchesParent: true, coverageUmbrella: true, priorBatchesParent: true}
	var found *inboxbatch.Item
	for i := range items {
		it := items[i]
		if parents[it.ID] {
			continue
		}
		for _, edge := range append(append([]string{}, it.ConnectsTo...), it.Deps...) {
			if edge == coverageBatchesParent {
				found = &it
			}
		}
	}
	if found == nil {
		t.Fatalf("no new inbox-root item links to %s (connects_to/deps) — a PASS retires this batch with the rest of internal/core and the .cover-strict registration recorded nowhere", coverageBatchesParent)
	}
	if found.Title == "" || found.Weight <= 0 || len(found.Acceptance) == 0 {
		t.Errorf("remainder %s needs a title, a positive weight and acceptance; got title=%q weight=%v acceptance=%d", found.ID, found.Title, found.Weight, len(found.Acceptance))
	}
	path, ferr := inboxmover.FindFileByTaskID(inboxDir, found.ID)
	if ferr != nil {
		t.Fatalf("remainder %s is not claimable: %v", found.ID, ferr)
	}
	rel, _ := filepath.Rel(src, path)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", src, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is on disk but not git-tracked — it would be dropped at ship", rel)
	}
}
