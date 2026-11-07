//go:build acs

package cycle2210

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
)

const (
	reviewRecordPath = "docs/operations/release-content-review.json"
	reviewSeat       = "docs-review"
	bypassFlag       = "--skip-content-review"
	bypassStep       = "content-review-bypass"
)

type journeyRepo struct {
	repo    *gittest.Repo
	journal string
	now     time.Time
}

type reviewRun struct {
	runID   string
	seat    string
	verdict string
	ts      time.Time
}

func setCommitDate(t *testing.T, when time.Time) {
	t.Helper()
	stamp := when.UTC().Format(time.RFC3339)
	t.Setenv("GIT_COMMITTER_DATE", stamp)
	t.Setenv("GIT_AUTHOR_DATE", stamp)
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (j *journeyRepo) land(t *testing.T, version string, ageHours int) {
	t.Helper()
	setCommitDate(t, j.now.Add(-time.Duration(ageHours)*time.Hour))
	versionDoc := `{"version": "` + version + `"}` + "\n"
	writeFile(t, j.repo.Dir, ".claude-plugin/plugin.json", versionDoc)
	writeFile(t, j.repo.Dir, ".claude-plugin/marketplace.json", versionDoc)
	j.repo.Git("add", "-A")
	j.repo.Git("commit", "-q", "-m", "release: v"+version)
	j.repo.Git("tag", "v"+version)
}

func newJourney(t *testing.T) *journeyRepo {
	t.Helper()
	j := &journeyRepo{repo: gittest.Fixture(t), journal: t.TempDir(), now: time.Now().UTC()}
	t.Setenv("TMPDIR", j.journal)
	t.Setenv("EVOLVE_PROJECT_ROOT", j.repo.Dir)
	j.land(t, "23.2.1", 48)
	return j
}

func (j *journeyRepo) recordRun(t *testing.T, run reviewRun) {
	t.Helper()
	rec := launchrecord.Record{
		RunID: run.runID, Profile: run.seat, CLI: "claude", Family: "claude", Model: "m",
		Verdict: run.verdict, TS: run.ts.UTC().Format(time.RFC3339),
	}
	if _, err := launchrecord.Write(filepath.Join(j.repo.Dir, ".evolve"), rec); err != nil {
		t.Fatal(err)
	}
}

func (j *journeyRepo) commitRecord(t *testing.T, release, runID string, ageHours int) {
	t.Helper()
	setCommitDate(t, j.now.Add(-time.Duration(ageHours)*time.Hour))
	body, err := json.Marshal(map[string]string{"release": release, "run_id": runID})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, j.repo.Dir, reviewRecordPath, string(body)+"\n")
	j.repo.Git("add", "-A")
	j.repo.Git("commit", "-q", "-m", "docs: content review for v"+release)
}

type releaseOutcome struct {
	code   int
	stderr string
	steps  []map[string]any
}

func (j *journeyRepo) release(t *testing.T, target string, extra ...string) releaseOutcome {
	t.Helper()
	args := append([]string{target, "--dry-run", "--skip-tests"}, extra...)
	var stdout, stderr bytes.Buffer
	code := opscmd.RunReleasePipeline(args, nil, &stdout, &stderr)
	out := releaseOutcome{code: code, stderr: stderr.String()}
	raw, err := os.ReadFile(filepath.Join(j.journal, "release-pipeline-dryrun-"+target+".json"))
	if err != nil {
		return out
	}
	var doc struct {
		Steps []map[string]any `json:"steps"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("journal of %s is not JSON: %v", target, err)
	}
	out.steps = doc.Steps
	return out
}

func (o releaseOutcome) refusedForContentReview() bool {
	return o.code != 0 && strings.Contains(strings.ToLower(o.stderr), "content review")
}

func (o releaseOutcome) step(name string) (map[string]any, bool) {
	for _, s := range o.steps {
		if s["step"] == name {
			return s, true
		}
	}
	return nil, false
}

func (j *journeyRepo) shipReleaseA(t *testing.T) {
	t.Helper()
	j.recordRun(t, reviewRun{runID: "review-a", seat: reviewSeat, verdict: "PASS", ts: j.now.Add(-24 * time.Hour)})
	j.commitRecord(t, "23.3.0", "review-a", 20)
	outcome := j.release(t, "23.3.0")
	if outcome.code != 0 {
		t.Fatalf("release A (23.3.0) with a content review must pass preflight; exit=%d stderr=%s", outcome.code, outcome.stderr)
	}
	j.land(t, "23.3.0", 12)
}

func TestC2210_001_MinorReleaseWithNoNewReviewIsRefused(t *testing.T) {
	j := newJourney(t)
	j.shipReleaseA(t)

	refused := j.release(t, "23.4.0")

	if !refused.refusedForContentReview() {
		t.Fatalf("release B (23.4.0) has no review of its own; want a nonzero exit naming the content review, got exit=%d stderr=%s", refused.code, refused.stderr)
	}
	if step, ok := refused.step("preflight"); !ok || step["status"] != "fail" {
		t.Errorf("the release journal must record the failed preflight step, got %v", refused.steps)
	}
}

func TestC2210_002_MinorReleaseWithItsOwnReviewPasses(t *testing.T) {
	j := newJourney(t)
	j.shipReleaseA(t)
	j.recordRun(t, reviewRun{runID: "review-b", seat: reviewSeat, verdict: "PASS", ts: j.now.Add(-1 * time.Hour)})
	j.commitRecord(t, "23.4.0", "review-b", 1)

	outcome := j.release(t, "23.4.0")

	if outcome.code != 0 {
		t.Fatalf("release B with its own PASS review must pass preflight; exit=%d stderr=%s", outcome.code, outcome.stderr)
	}
}

func TestC2210_003_PatchReleaseNeedsNoReview(t *testing.T) {
	j := newJourney(t)

	outcome := j.release(t, "23.2.2")

	if outcome.code != 0 {
		t.Fatalf("a patch release must not need a content review; exit=%d stderr=%s", outcome.code, outcome.stderr)
	}
}

func TestC2210_004_StandalonePreflightRefusesTheSameWay(t *testing.T) {
	newJourney(t)
	var stdout, stderr bytes.Buffer

	code := opscmd.RunReleasePreflight([]string{"23.3.0", "--dry-run", "--skip-tests"}, nil, &stdout, &stderr)

	if code == 0 || !strings.Contains(strings.ToLower(stderr.String()), "content review") {
		t.Fatalf("release-preflight 23.3.0 with no review record must refuse and name the content review; exit=%d stderr=%s", code, stderr.String())
	}
}

func TestC2210_005_ReplayedRunOfAnEarlierReleaseIsRefused(t *testing.T) {
	j := newJourney(t)
	j.shipReleaseA(t)
	j.commitRecord(t, "23.4.0", "review-a", 1)

	outcome := j.release(t, "23.4.0")

	if !outcome.refusedForContentReview() {
		t.Fatalf("a record for 23.4.0 that cites the run of release A (run before the v23.3.0 tag) is a replay; want refusal naming the content review, got exit=%d stderr=%s", outcome.code, outcome.stderr)
	}
}

func TestC2210_006_RunOfAnotherSeatIsRefused(t *testing.T) {
	cases := []struct {
		name string
		run  reviewRun
	}{
		{"another seat", reviewRun{runID: "review-x", seat: "code-reviewer", verdict: "PASS"}},
		{"verdict FAIL", reviewRun{runID: "review-x", seat: reviewSeat, verdict: "FAIL"}},
		{"verdict WARN", reviewRun{runID: "review-x", seat: reviewSeat, verdict: "WARN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := newJourney(t)
			run := tc.run
			run.ts = j.now.Add(-1 * time.Hour)
			j.recordRun(t, run)
			j.commitRecord(t, "23.3.0", run.runID, 1)

			outcome := j.release(t, "23.3.0")

			if !outcome.refusedForContentReview() {
				t.Fatalf("%s must be refused; exit=%d stderr=%s", tc.name, outcome.code, outcome.stderr)
			}
		})
	}
}

func TestC2210_007_RecordThatNamesNoRealRunOrNoCommitIsRefused(t *testing.T) {
	t.Run("run id has no launch record", func(t *testing.T) {
		j := newJourney(t)
		j.commitRecord(t, "23.3.0", "never-ran", 1)

		outcome := j.release(t, "23.3.0")

		if !outcome.refusedForContentReview() {
			t.Fatalf("a record citing an unknown run must be refused; exit=%d stderr=%s", outcome.code, outcome.stderr)
		}
	})
	t.Run("record file is not committed", func(t *testing.T) {
		j := newJourney(t)
		j.recordRun(t, reviewRun{runID: "review-u", seat: reviewSeat, verdict: "PASS", ts: j.now.Add(-1 * time.Hour)})
		writeFile(t, j.repo.Dir, reviewRecordPath, `{"release":"23.3.0","run_id":"review-u"}`+"\n")

		outcome := j.release(t, "23.3.0")

		if !outcome.refusedForContentReview() {
			t.Fatalf("an untracked review record must be refused (the gate reads the committed file); exit=%d stderr=%s", outcome.code, outcome.stderr)
		}
	})
}

func TestC2210_008_BypassIsAReleaseParameterAndJournalsItsReason(t *testing.T) {
	const reason = "models down during the v23.3.0 window"
	j := newJourney(t)

	outcome := j.release(t, "23.3.0", bypassFlag, reason)

	if outcome.code != 0 {
		t.Fatalf("evolve release %s <reason> must let the minor release pass preflight; exit=%d stderr=%s", bypassFlag, outcome.code, outcome.stderr)
	}
	step, ok := outcome.step(bypassStep)
	if !ok {
		t.Fatalf("the release journal must hold a %q step, got %v", bypassStep, outcome.steps)
	}
	if note, _ := step["note"].(string); !strings.Contains(note, reason) {
		t.Errorf("the %q step must carry the reason %q in its note, got %v", bypassStep, reason, step)
	}
}

func TestC2210_009_BypassWithoutAReasonIsRefusedAndNoFlagWritesNoBypassStep(t *testing.T) {
	j := newJourney(t)

	noReason := j.release(t, "23.3.0", bypassFlag, "")
	if noReason.code == 0 || !strings.Contains(strings.ToLower(noReason.stderr), "reason") {
		t.Errorf("%s with an empty reason must be refused and name the reason; exit=%d stderr=%s", bypassFlag, noReason.code, noReason.stderr)
	}
	if _, ok := noReason.step(bypassStep); ok {
		t.Errorf("a refused bypass must not journal a bypass step, got %v", noReason.steps)
	}

	plain := j.release(t, "23.2.2")
	if _, ok := plain.step(bypassStep); ok {
		t.Errorf("a release run without %s must not journal a bypass, got %v", bypassFlag, plain.steps)
	}
}

func TestC2210_010_ReviewRecordIsOutsideTheVersionBumpSet(t *testing.T) {
	root := t.TempDir()

	for _, file := range versionbump.DefaultPaths(root).Files() {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.ToSlash(rel) == reviewRecordPath {
			t.Fatalf("the version bump must not rewrite the review record %s", reviewRecordPath)
		}
	}
}
