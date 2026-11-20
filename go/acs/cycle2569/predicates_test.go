//go:build acs

package cycle2569

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	day            = 24 * time.Hour
	widgetFile     = "go/internal/widget/add.go"
	widgetTestFile = "go/internal/widget/add_test.go"
	notesFile      = "docs/notes.md"
	baseFile       = "go/internal/widget/base.go"
	tolerance      = 1e-9
)

var fixedNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

var binDir string

var (
	buildOnce sync.Once
	builtPath string
	buildFail error
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2569-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	binDir = dir
	return m.Run()
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		builtPath = filepath.Join(binDir, "evolve-under-test")
		out, code, err := moduleGo(t, "build", "-o", builtPath, "./cmd/evolve")
		if err != nil || code != 0 {
			buildFail = fmt.Errorf("go build ./cmd/evolve: code=%d err=%v\n%s", code, err, acsassert.OutputTail(out, 30))
		}
	})
	if buildFail != nil {
		t.Fatalf("%v", buildFail)
	}
	return builtPath
}

func moduleGo(t *testing.T, args ...string) (string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", append([]string{"-C", acsassert.GoModuleDir(t)}, args...)...)
	return stdout + stderr, code, err
}

func numbered(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s%d\n", prefix, i)
	}
	return b.String()
}

func commitAt(t *testing.T, repo *gittest.Repo, when time.Time, files map[string]string, messages ...string) string {
	t.Helper()
	stamp := when.UTC().Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", stamp)
	t.Setenv("GIT_COMMITTER_DATE", stamp)
	for rel, body := range files {
		path := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	args := []string{"commit", "-q", "--allow-empty"}
	for _, m := range messages {
		args = append(args, "-m", m)
	}
	repo.Git(args...)
	return repo.Git("rev-parse", "HEAD")
}

func widgetRepo(t *testing.T, now time.Time, rewriteAfter time.Duration) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	commitAt(t, repo, now.Add(-40*day), map[string]string{baseFile: numbered("base", 5)}, "chore: base")
	added := now.Add(-20 * day)
	commitAt(t, repo, added, map[string]string{
		widgetFile:     numbered("line", 10),
		widgetTestFile: numbered("check", 3),
		notesFile:      numbered("note", 3),
	}, "feat: add widget")
	if rewriteAfter > 0 {
		rewritten := strings.Replace(numbered("line", 10), "line4\n", "line4 rewritten\n", 1)
		commitAt(t, repo, added.Add(rewriteAfter), map[string]string{widgetFile: rewritten}, "fix: rewrite line four")
	}
	return repo
}

func churnOf(t *testing.T, repo *gittest.Repo, now time.Time) qualitypanel.ChurnResult {
	t.Helper()
	res, err := qualitypanel.MeasureChurn(qualitypanel.ChurnInput{Repo: repo.Dir, Now: now, Window: "commits:14-30d", Source: "git", Cycle: 2569})
	if err != nil {
		t.Fatalf("MeasureChurn: %v", err)
	}
	return res
}

func revertsOf(t *testing.T, repo *gittest.Repo, now time.Time) qualitypanel.RevertRateResult {
	t.Helper()
	res, err := qualitypanel.MeasureRevertRate(qualitypanel.RevertRateInput{Repo: repo.Dir, Now: now, Window: "days:30", Source: "git", Cycle: 2569})
	if err != nil {
		t.Fatalf("MeasureRevertRate: %v", err)
	}
	return res
}

func near(a, b float64) bool { return math.Abs(a-b) < tolerance }

func TestC2569_001_OneOfTenAddedLinesRewrittenWithinFourteenDaysIsTenPercentChurn(t *testing.T) {
	repo := widgetRepo(t, fixedNow, 10*day)
	res := churnOf(t, repo, fixedNow)
	if res.Added != 10 || res.Survived != 9 {
		t.Errorf("added=%d survived=%d, want 10 9: only the production Go lines of the one commit that is 14 to 30 days old count", res.Added, res.Survived)
	}
	if !near(res.OQ2.Interval.Value, 0.10) {
		t.Errorf("OQ2 value = %v, want 0.10", res.OQ2.Interval.Value)
	}
	if res.OQ2.Index != "OQ2" || res.OQ2.N != 1 || res.OQ2.Method != qualitypanel.MethodBootstrap95 {
		t.Errorf("OQ2 reading = index %q n %d method %q, want OQ2 1 bootstrap95", res.OQ2.Index, res.OQ2.N, res.OQ2.Method)
	}
	if iv := res.OQ2.Interval; iv.Lo > iv.Value+tolerance || iv.Hi < iv.Value-tolerance {
		t.Errorf("interval = %+v, want lo <= value <= hi", iv)
	}
}

func TestC2569_002_ALineRewrittenAfterFourteenDaysDoesNotCount(t *testing.T) {
	repo := widgetRepo(t, fixedNow, 15*day)
	res := churnOf(t, repo, fixedNow)
	if res.Added != 10 || res.Survived != 10 {
		t.Errorf("added=%d survived=%d, want 10 10: a rewrite 15 days after the add is outside the snapshot", res.Added, res.Survived)
	}
	if res.OQ2.Interval.Value != 0 {
		t.Errorf("OQ2 value = %v, want 0", res.OQ2.Interval.Value)
	}
}

func TestC2569_003_NoCommitInTheWindowIsNoData(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixedNow.Add(-40*day), map[string]string{baseFile: numbered("base", 5)}, "chore: base")
	commitAt(t, repo, fixedNow.Add(-3*day), map[string]string{baseFile: numbered("base", 6)}, "chore: recent")
	res := churnOf(t, repo, fixedNow)
	ix, _ := qualitypanel.Lookup("OQ2")
	if res.Added != 0 || res.OQ2.N != 0 || qualitypanel.Judge(ix, res.OQ2) != qualitypanel.StatusNoData {
		t.Errorf("added=%d n=%d status=%s, want 0 0 no_data for a window with no commit", res.Added, res.OQ2.N, qualitypanel.Judge(ix, res.OQ2))
	}
}

func TestC2569_004_ADirectoryThatIsNotARepositoryIsAnErrorNotAZeroChurn(t *testing.T) {
	notRepo := t.TempDir()
	if res, err := qualitypanel.MeasureChurn(qualitypanel.ChurnInput{Repo: notRepo, Now: fixedNow}); err == nil {
		t.Errorf("MeasureChurn on a non-repository = %+v, nil error, want an error", res)
	}
	if res, err := qualitypanel.MeasureRevertRate(qualitypanel.RevertRateInput{Repo: notRepo, Now: fixedNow}); err == nil {
		t.Errorf("MeasureRevertRate on a non-repository = %+v, nil error, want an error", res)
	}
}

func TestC2569_005_ACommitWhoseBodySaysThisRevertsCommitIsARevert(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixedNow.Add(-45*day), map[string]string{baseFile: "x\n"}, "Revert \"ancient change\"")
	commitAt(t, repo, fixedNow.Add(-10*day), map[string]string{widgetFile: "a\n"}, "feat: add a")
	target := commitAt(t, repo, fixedNow.Add(-9*day), map[string]string{widgetFile: "b\n"}, "feat: add b")
	commitAt(t, repo, fixedNow.Add(-8*day), map[string]string{widgetFile: "a\n"}, "fix: undo b", "This reverts commit "+target+".")
	res := revertsOf(t, repo, fixedNow)
	if res.Reverts != 1 || res.Commits != 3 {
		t.Errorf("reverts=%d commits=%d, want 1 3: the body trailer counts and the 45-day-old commit is outside the 30-day window", res.Reverts, res.Commits)
	}
	if want := qualitypanel.Wilson(1, 3); !near(res.OQ3.Interval.Value, want.Value) || !near(res.OQ3.Interval.Hi, want.Hi) {
		t.Errorf("OQ3 interval = %+v, want Wilson(1, 3) = %+v", res.OQ3.Interval, want)
	}
	if res.OQ3.Index != "OQ3" || res.OQ3.N != 3 || res.OQ3.Method != qualitypanel.MethodWilson95 {
		t.Errorf("OQ3 reading = index %q n %d method %q, want OQ3 3 wilson95", res.OQ3.Index, res.OQ3.N, res.OQ3.Method)
	}
}

func TestC2569_006_ACommitWhoseSubjectStartsWithRevertIsARevertAndLookalikesAreNot(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixedNow.Add(-10*day), map[string]string{widgetFile: "a\n"}, "feat: add a")
	commitAt(t, repo, fixedNow.Add(-9*day), map[string]string{widgetFile: "b\n"}, "Revert \"feat: add a\"")
	commitAt(t, repo, fixedNow.Add(-8*day), map[string]string{widgetFile: "c\n"}, "Reverse the list order", "this reverts nothing at all")
	res := revertsOf(t, repo, fixedNow)
	if res.Reverts != 1 || res.Commits != 3 {
		t.Errorf("reverts=%d commits=%d, want 1 3: a Revert subject counts; Reverse and a lower-case body do not", res.Reverts, res.Commits)
	}
}

func TestC2569_007_ALogWithNoRevertGivesZeroWithTheWilsonUpperBound(t *testing.T) {
	repo := gittest.Fixture(t)
	for i := 0; i < 10; i++ {
		commitAt(t, repo, fixedNow.Add(-time.Duration(20-i)*day), nil, fmt.Sprintf("chore: step %d", i))
	}
	res := revertsOf(t, repo, fixedNow)
	if res.Reverts != 0 || res.Commits != 10 {
		t.Fatalf("reverts=%d commits=%d, want 0 10", res.Reverts, res.Commits)
	}
	iv := res.OQ3.Interval
	if iv.Value != 0 || iv.Lo != 0 || math.Abs(iv.Hi-0.2775) > 0.001 {
		t.Errorf("interval = %+v, want value 0, lo 0, hi 0.2775 (Wilson upper bound for 0 of 10)", iv)
	}
}

func TestC2569_008_ARevertOlderThanThirtyDaysIsNotInTheWindow(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixedNow.Add(-31*day), map[string]string{widgetFile: "a\n"}, "Revert \"too old\"")
	commitAt(t, repo, fixedNow.Add(-29*day), map[string]string{widgetFile: "b\n"}, "feat: inside the window")
	res := revertsOf(t, repo, fixedNow)
	if res.Reverts != 0 || res.Commits != 1 {
		t.Errorf("reverts=%d commits=%d, want 0 1", res.Reverts, res.Commits)
	}
}

func recoveryDossier(cycle int, verdict, task, sha, started, ended string) *dossier.Dossier {
	tasks := []string{task}
	d := &dossier.Dossier{
		Cycle: cycle, Goal: "recovery-fixture", FinalVerdict: verdict, Tasks: &tasks, CommitSHA: sha, StartedAt: started, EndedAt: ended,
		Phases: []dossier.PhaseRecord{{Name: "scout", Verdict: dossier.VerdictPass}},
	}
	if verdict == dossier.VerdictFail {
		d.Defects = []dossier.Defect{{ID: "D1", Severity: "HIGH", Summary: "fixture failure"}}
		d.Carryover = []dossier.Carryover{{ID: "C1", Action: "fix the fixture failure", Priority: "P1"}}
	}
	return d
}

func recoveryOf(t *testing.T, dossiers ...*dossier.Dossier) pipelinerates.RecoveryResult {
	t.Helper()
	return pipelinerates.MeasureRecovery(pipelinerates.RecoveryInput{Dossiers: dossiers, Window: "wave:1", Source: "knowledge-base/cycles", Cycle: 2569})
}

const (
	failStart = "2026-11-01T00:00:00Z"
	failEnd   = "2026-11-01T01:30:00Z"
	passStart = "2026-11-02T04:00:00Z"
	passEnd   = "2026-11-02T08:00:00Z"
)

func TestC2569_009_OneFailedAndOneLaterShippedDossierIsOneRecoveryWithTheExactHours(t *testing.T) {
	res := recoveryOf(t,
		recoveryDossier(10, dossier.VerdictFail, "item-x", "", failStart, failEnd),
		recoveryDossier(11, dossier.VerdictPass, "item-x", "abc123", passStart, passEnd),
	)
	if res.Recoveries != 1 {
		t.Fatalf("recoveries = %d, want 1", res.Recoveries)
	}
	if !slices.Equal(res.Hours, []float64{30.5}) || !near(res.MedianHours, 30.5) {
		t.Errorf("hours = %v median = %v, want [30.5] 30.5: from the end of the first failed try to the end of the shipped try", res.Hours, res.MedianHours)
	}
	if res.OQ4.Index != "OQ4" || res.OQ4.N != 1 || !near(res.OQ4.Interval.Value, 1) || res.OQ4.Method != qualitypanel.MethodWilson95 {
		t.Errorf("OQ4 reading = index %q n %d value %v method %q, want OQ4 1 1 wilson95", res.OQ4.Index, res.OQ4.N, res.OQ4.Interval.Value, res.OQ4.Method)
	}
}

func TestC2569_010_TimeToGreenCountsFromTheFirstFailureAndTheMedianSpansItems(t *testing.T) {
	res := recoveryOf(t,
		recoveryDossier(20, dossier.VerdictFail, "item-a", "", "2026-11-03T00:00:00Z", "2026-11-03T00:30:00Z"),
		recoveryDossier(21, dossier.VerdictPass, "item-a", "a1", "2026-11-03T09:00:00Z", "2026-11-03T10:30:00Z"),
		recoveryDossier(22, dossier.VerdictFail, "item-b", "", "2026-11-04T00:00:00Z", "2026-11-04T02:00:00Z"),
		recoveryDossier(23, dossier.VerdictFail, "item-b", "", "2026-11-04T03:00:00Z", "2026-11-04T07:00:00Z"),
		recoveryDossier(24, dossier.VerdictPass, "item-b", "b1", "2026-11-04T20:00:00Z", "2026-11-05T02:00:00Z"),
		recoveryDossier(25, dossier.VerdictFail, "item-c", "", "2026-11-06T00:00:00Z", "2026-11-06T01:00:00Z"),
		recoveryDossier(26, dossier.VerdictPass, "item-c", "c1", "2026-11-08T00:00:00Z", "2026-11-08T03:00:00Z"),
	)
	hours := slices.Clone(res.Hours)
	slices.Sort(hours)
	if res.Recoveries != 3 || !slices.Equal(hours, []float64{10, 24, 50}) || !near(res.MedianHours, 24) {
		t.Errorf("recoveries=%d hours=%v median=%v, want 3 [10 24 50] 24", res.Recoveries, hours, res.MedianHours)
	}
}

func TestC2569_011_AnItemThatNeverShipsOrNeverFailedIsNotARecovery(t *testing.T) {
	failedOnly := recoveryOf(t, recoveryDossier(30, dossier.VerdictFail, "item-y", "", failStart, failEnd))
	if failedOnly.Recoveries != 0 || len(failedOnly.Hours) != 0 || failedOnly.OQ4.N != 1 || failedOnly.OQ4.Interval.Value != 0 {
		t.Errorf("failed only: recoveries=%d hours=%v n=%d value=%v, want 0 [] 1 0", failedOnly.Recoveries, failedOnly.Hours, failedOnly.OQ4.N, failedOnly.OQ4.Interval.Value)
	}
	shippedFirstTime := recoveryOf(t, recoveryDossier(31, dossier.VerdictPass, "item-z", "def456", passStart, passEnd))
	ix, _ := qualitypanel.Lookup("OQ4")
	if shippedFirstTime.Recoveries != 0 || shippedFirstTime.OQ4.N != 0 || qualitypanel.Judge(ix, shippedFirstTime.OQ4) != qualitypanel.StatusNoData {
		t.Errorf("shipped first time: recoveries=%d n=%d, want 0 0 and no_data", shippedFirstTime.Recoveries, shippedFirstTime.OQ4.N)
	}
}

func TestC2569_012_ARecoveryWhoseDossiersCarryNoTimeStillCountsAsARecoveryWithNoHours(t *testing.T) {
	res := recoveryOf(t,
		recoveryDossier(40, dossier.VerdictFail, "item-w", "", "", ""),
		recoveryDossier(41, dossier.VerdictPass, "item-w", "w1", "", ""),
	)
	if res.Recoveries != 1 || len(res.Hours) != 0 || res.MedianHours != 0 {
		t.Errorf("recoveries=%d hours=%v median=%v, want 1 [] 0", res.Recoveries, res.Hours, res.MedianHours)
	}
}

func TestC2569_013_TheRetrospectiveDetectorStillNeedsTwoFailures(t *testing.T) {
	fail := recoveryDossier(50, dossier.VerdictFail, "item-v", "", failStart, failEnd)
	pass := recoveryDossier(51, dossier.VerdictPass, "item-v", "v1", passStart, passEnd)
	if got := recoveryretro.Recoveries([]*dossier.Dossier{fail, pass}, pass); len(got) != 0 {
		t.Errorf("recoveryretro.Recoveries with one failure = %d recoveries, want 0: MinFailures %d keeps the retrospective gate", len(got), recoveryretro.MinFailures)
	}
}

var funcLine = regexp.MustCompile(`^\S+/([A-Za-z0-9_]+\.go):\d+:\s+\S+\s+([0-9.]+)%$`)

func coverageOfFiles(t *testing.T, pkg, runPattern string, filePrefixes ...string) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, code, err := moduleGo(t, "test", "-count=1", "-timeout", "300s", "-run", runPattern, "-coverprofile="+profile, pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test %s -run %s: code=%d err=%v\n%s", pkg, runPattern, code, err, acsassert.OutputTail(out, 30))
	}
	funcs, code, err := moduleGo(t, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover: code=%d err=%v\n%s", code, err, acsassert.OutputTail(funcs, 30))
	}
	seen := map[string]int{}
	for _, line := range strings.Split(funcs, "\n") {
		m := funcLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		pct, _ := strconv.ParseFloat(m[2], 64)
		for _, prefix := range filePrefixes {
			if strings.HasPrefix(m[1], prefix) && !strings.HasSuffix(m[1], "_test.go") {
				seen[prefix]++
				if pct < 100 {
					t.Errorf("%s: %q is %.1f%% covered, want 100%%", pkg, strings.TrimSpace(line), pct)
				}
			}
		}
	}
	for _, prefix := range filePrefixes {
		if seen[prefix] == 0 {
			t.Errorf("%s: no function in %s*.go was covered by %s, want the new reader there", pkg, prefix, runPattern)
		}
	}
}

func TestC2569_014_TheChurnAndRevertReadersAreFullyCoveredByTheirOwnTests(t *testing.T) {
	coverageOfFiles(t, "./internal/qualitypanel", "^Test(Churn|RevertRate)_", "churn", "revert_rate")
}

func TestC2569_015_TheRecoveryReaderIsFullyCoveredByItsOwnTests(t *testing.T) {
	coverageOfFiles(t, "./internal/pipelinerates", "^TestRecovery_", "recovery")
}

func TestC2569_016_TheDefaultStagesAreReportForChurnAndRevertAndShadowForRecovery(t *testing.T) {
	want := map[string]qualitypanel.Stage{"OQ2": qualitypanel.StageReport, "OQ3": qualitypanel.StageReport, "OQ4": qualitypanel.StageShadow}
	for id, stage := range want {
		if got := qualitypanel.StageFor(id, nil); got != stage {
			t.Errorf("StageFor(%s) = %s, want %s", id, got, stage)
		}
	}
	res := recoveryOf(t,
		recoveryDossier(60, dossier.VerdictFail, "item-u", "", failStart, failEnd),
		recoveryDossier(61, dossier.VerdictPass, "item-u", "u1", passStart, passEnd),
	)
	for _, e := range res.Events(qualitypanel.StageFor("OQ4", nil)) {
		if string(e.Kind) == "quality.breach" {
			t.Errorf("OQ4 event %+v at the default stage, want no breach at shadow", e)
		}
	}
}

type channelFields map[string]string

func readingsOnChannel(t *testing.T, root string) map[string]channelFields {
	t.Helper()
	segments, err := filepath.Glob(filepath.Join(root, ".evolve", "events", "ch", "quality.reading", "seg-*.ndjson"))
	if err != nil || len(segments) == 0 {
		t.Fatalf("no quality.reading segment under %s (glob err %v)", root, err)
	}
	out := map[string]channelFields{}
	for _, seg := range segments {
		raw, err := os.ReadFile(seg)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			var rec struct {
				Signal struct {
					Fields map[string]string `json:"fields"`
				} `json:"signal"`
			}
			if json.Unmarshal([]byte(line), &rec) == nil && rec.Signal.Fields["index"] != "" {
				out[rec.Signal.Fields["index"]] = rec.Signal.Fields
			}
		}
	}
	return out
}

func runQualityReport(t *testing.T, root, scope string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "quality", "report", "--scope", scope, "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("evolve quality report --scope %s: code=%d err=%v\n%s", scope, code, err, acsassert.OutputTail(stdout+stderr, 20))
	}
}

func TestC2569_017_TheTreeScopeCommandReadsChurnAndRevertRateAtStageReport(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	repo := widgetRepo(t, now, 10*day)
	runQualityReport(t, repo.Dir, "tree")
	readings := readingsOnChannel(t, repo.Dir)
	oq2, oq3 := readings["OQ2"], readings["OQ3"]
	if oq2["value"] != "0.100" || oq2["n"] != "1" || oq2["stage"] != "report" {
		t.Errorf("OQ2 fields = %v, want value 0.100 n 1 stage report", oq2)
	}
	if oq3["value"] != "0.000" || oq3["n"] != "2" || oq3["stage"] != "report" {
		t.Errorf("OQ3 fields = %v, want value 0.000 n 2 stage report", oq3)
	}
}

func TestC2569_018_TheWaveScopeCommandReadsRecoveryFromBothDossierDirectoriesAtStageShadow(t *testing.T) {
	root := t.TempDir()
	cycles, pending := dossier.CyclesDir(root), dossier.PendingDir(root)
	for _, dir := range []string{cycles, pending} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := dossier.Write(recoveryDossier(70, dossier.VerdictFail, "item-t", "", failStart, failEnd), cycles, false); err != nil {
		t.Fatal(err)
	}
	if err := dossier.Write(recoveryDossier(71, dossier.VerdictPass, "item-t", "t1", passStart, passEnd), pending, false); err != nil {
		t.Fatal(err)
	}
	runQualityReport(t, root, "wave")
	oq4 := readingsOnChannel(t, root)["OQ4"]
	if oq4["value"] != "1.000" || oq4["n"] != "1" || oq4["stage"] != "shadow" {
		t.Errorf("OQ4 fields = %v, want value 1.000 n 1 stage shadow", oq4)
	}
	if breaches, _ := filepath.Glob(filepath.Join(root, ".evolve", "events", "ch", "quality.breach", "seg-*.ndjson")); len(breaches) > 0 {
		for _, seg := range breaches {
			if raw, _ := os.ReadFile(seg); strings.Contains(string(raw), `"OQ4"`) {
				t.Errorf("quality.breach names OQ4 at stage shadow: %s", raw)
			}
		}
	}
}
