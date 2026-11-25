package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const historyBase = "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n"

const historyTrimmed = "package x\n\nfunc F() {}\n"

const historyRootPage = commentaudit.HistoryArchiveDir + "/root.md"

type historyLane struct {
	repo *gittest.Repo
	base string
}

const historyBulkBody = "func A() {}\nfunc B() {}\nfunc C() {}\nfunc D() {}\nfunc E() {}\nfunc G() {}\n"

const historySecond = "package y\n\n// H is the helper.\n// It used to live in cycle 7's queue.\nfunc H() {}\n"

func newCommittedHistoryLane(t *testing.T, template string, committed map[string]string, files map[string]string) historyLane {
	t.Helper()
	repo := gittest.FromTemplate(t, template, func(r *gittest.Repo) error {
		for path, text := range committed {
			r.Write(path, text)
		}
		r.Git("add", "-A")
		r.Git("commit", "-q", "-m", "base")
		return nil
	})
	for path, text := range files {
		writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(path)), text)
	}
	return historyLane{repo: repo, base: headSHA(t, repo.Dir)}
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	head, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(string(head), "ref:"))
	sha, err := os.ReadFile(filepath.Join(dir, ".git", filepath.FromSlash(ref)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(sha))
}

func renameInLane(t *testing.T, lane historyLane, from, to string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(lane.repo.Dir, from))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(lane.repo.Dir, from)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(lane.repo.Dir, to), string(body))
}

func newHistoryLane(t *testing.T, files map[string]string) historyLane {
	t.Helper()
	return newCommittedHistoryLane(t, "core-history-floor-base", map[string]string{"x.go": historyBase}, files)
}

func (l historyLane) review(t *testing.T, policyJSON string, signals *signalcenter.Center, warn *strings.Builder) ReviewInput {
	t.Helper()
	root := t.TempDir()
	if policyJSON != "" {
		writeFile(t, filepath.Join(root, ".evolve", "policy.json"), policyJSON)
	}
	return ReviewInput{Phase: string(PhaseBuild), ProjectRoot: root, Worktree: l.repo.Dir, WorktreeBaseSHA: l.base, Signals: signals, Stderr: warn}
}

func recordedPage(t *testing.T) string {
	t.Helper()
	removed := []commentaudit.HistoryEntry{{File: "x.go", Line: 3, Anchor: "func F() {}", Lines: []string{"// F is the entry point.", "// It used to live in cycle 42's retry loop."}}}
	return commentaudit.RenderHistorySection("round 1", removed)
}

func historyFailureText(failures []string) string { return strings.Join(failures, "\n") }

func TestStaticBuildFloorChecks_AtEnforceAnUnrecordedHistoryRemovalGoesBackToBuild(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed})
	var warn strings.Builder

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	text := historyFailureText(failures)
	for _, want := range []string{"x.go:3", "-base " + lane.base + " ", commentaudit.HistoryArchiveDir} {
		if !strings.Contains(text, want) {
			t.Errorf("the failure lacks %q:\n%s", want, text)
		}
	}
}

func TestStaticBuildFloorChecks_AtEnforceARecordedRemovalPasses(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed, historyRootPage: recordedPage(t)})
	var warn strings.Builder

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if strings.Contains(historyFailureText(failures), "x.go:3") {
		t.Fatalf("a recorded removal must pass: %v", failures)
	}
}

func TestStaticBuildFloorChecks_AtTheDefaultShadowWarnsEmitsAndDoesNotFail(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed})
	var warn strings.Builder
	center := signalcenter.New()

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, "", center, &warn))

	if strings.Contains(historyFailureText(failures), "x.go:3") {
		t.Fatalf("shadow must not fail: %v", failures)
	}
	if !strings.Contains(warn.String(), "history floor (shadow)") || !strings.Contains(warn.String(), "x.go:3") {
		t.Errorf("shadow must warn and name the entry: %q", warn.String())
	}
	center.Flush()
	var events []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == CodeCommentHistoryShadow {
			events = append(events, e)
		}
	}
	if len(events) != 1 || events[0].Fields["count"] != "1" || !strings.Contains(events[0].Fields["entries"], "x.go:3") {
		t.Fatalf("shadow must emit one event with count and entries: %+v", events)
	}
}

func TestStaticBuildFloorChecks_AtOffTheHistoryFloorIsSilent(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed})
	var warn strings.Builder
	center := signalcenter.New()

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"off"}}`, center, &warn))

	center.Flush()
	if strings.Contains(historyFailureText(failures), "x.go:3") || strings.Contains(warn.String(), "history floor") || len(center.Recent()) != 0 {
		t.Fatalf("off must be silent: %v %q %v", failures, warn.String(), center.Recent())
	}
}

func TestStaticBuildFloorChecks_AHistoryCommentMovedToAnotherFileIsNotRemoved(t *testing.T) {
	t.Parallel()
	moved := "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n"
	lane := newHistoryLane(t, map[string]string{"x.go": "package x\n", "y.go": moved})
	var warn strings.Builder

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if strings.Contains(historyFailureText(failures), "x.go:3") {
		t.Fatalf("a moved comment is not removed: %v", failures)
	}
}

func TestStaticBuildFloorChecks_ARenamedFileKeepsItsHistoryComment(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, nil)
	renameInLane(t, lane, "x.go", "z.go")
	var warn strings.Builder

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if strings.Contains(historyFailureText(failures), "x.go:3") {
		t.Fatalf("a renamed file keeps its comment: %v", failures)
	}
}

func TestStaticBuildFloorChecks_AnUnreadableBaseWarnsAndDoesNotFail(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed})
	lane.base = "0000000000000000000000000000000000000000"
	var warn strings.Builder

	failures := historyFloorFailures(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if failures != nil || !strings.Contains(warn.String(), "history floor could not read the diff") {
		t.Fatalf("an unreadable base warns: %v %q", failures, warn.String())
	}
}

func TestStaticBuildFloorChecks_AnUnparsableGoFileWarnsAndDoesNotFail(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": "package x\nfunc ("})
	var warn strings.Builder

	failures := historyFloorFailures(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if failures != nil || !strings.Contains(warn.String(), "history floor could not read the diff") {
		t.Fatalf("an unparsable file warns: %v %q", failures, warn.String())
	}
}

func TestHistoryFloorFailures_WithoutAWorktreeDoesNothing(t *testing.T) {
	t.Parallel()

	if got := historyFloorFailures(context.Background(), ReviewInput{ProjectRoot: t.TempDir()}); got != nil {
		t.Fatalf("no worktree, no failures: %v", got)
	}
}

func TestHistoryFloorResult_ByStage(t *testing.T) {
	t.Parallel()
	one := []commentaudit.HistoryEntry{{File: "a.go", Line: 4, Anchor: "func A() {}"}}

	for stage, want := range map[config.Stage][2]bool{
		config.StageOff:     {false, false},
		config.StageShadow:  {false, true},
		config.StageEnforce: {true, false},
	} {
		failures, warning := historyFloorResult(stage, one, "base1")
		if (len(failures) == 1) != want[0] || (warning != "") != want[1] {
			t.Errorf("stage %v: failures=%v warning=%q", stage, failures, warning)
		}
	}
	if failures, warning := historyFloorResult(config.StageEnforce, nil, "base1"); failures != nil || warning != "" {
		t.Errorf("no entries, no output: %v %q", failures, warning)
	}
}

func TestHistoryFloorStage_ABadPolicyFailsClosedAndTheStageIsReadTyped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), `{"comment_floor":{"history_stage":"bogus"}}`)
	var warn strings.Builder

	if got := historyFloorStage(root, &warn); got != config.StageEnforce {
		t.Fatalf("a refused policy: %v", got)
	}
	if got := historyFloorStage("", &warn); got != config.StageShadow {
		t.Fatalf("no project root runs the compiled default: %v", got)
	}
}

func TestHistoryFloorFailures_AtOffDoesNotReadTheDiff(t *testing.T) {
	t.Parallel()
	lane := newHistoryLane(t, map[string]string{"x.go": historyTrimmed})
	lane.base = "0000000000000000000000000000000000000000"
	var warn strings.Builder

	failures := historyFloorFailures(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"off"}}`, nil, &warn))

	if failures != nil || warn.String() != "" {
		t.Fatalf("off must not touch git: %v %q", failures, warn.String())
	}
}

func TestStaticBuildFloorChecks_ARenamedFileThatDropsItsHistoryCommentIsNamed(t *testing.T) {
	t.Parallel()
	lane := newCommittedHistoryLane(t, "core-history-floor-bulk", map[string]string{"x.go": historyBase + historyBulkBody}, nil)
	renameInLane(t, lane, "x.go", "z.go")
	writeFile(t, filepath.Join(lane.repo.Dir, "z.go"), historyTrimmed+historyBulkBody)
	var warn strings.Builder

	failures := StaticBuildFloorChecks(context.Background(), lane.review(t, `{"comment_floor":{"history_stage":"enforce"}}`, nil, &warn))

	if !strings.Contains(historyFailureText(failures), "x.go:3") {
		t.Fatalf("a rename that drops the comment must be named: %v", failures)
	}
}

func TestStaticBuildFloorChecks_ShadowCountsEveryUnrecordedGroup(t *testing.T) {
	t.Parallel()
	lane := newCommittedHistoryLane(t, "core-history-floor-pair", map[string]string{"x.go": historyBase, "y.go": historySecond}, nil)
	writeFile(t, filepath.Join(lane.repo.Dir, "x.go"), historyTrimmed)
	writeFile(t, filepath.Join(lane.repo.Dir, "y.go"), "package y\n\nfunc H() {}\n")
	var warn strings.Builder
	center := signalcenter.New()

	StaticBuildFloorChecks(context.Background(), lane.review(t, "", center, &warn))

	center.Flush()
	for _, e := range center.Recent() {
		if e.Code == CodeCommentHistoryShadow {
			if e.Fields["count"] != "2" {
				t.Fatalf("count = %q, want 2", e.Fields["count"])
			}
			return
		}
	}
	t.Fatal("no shadow event")
}
