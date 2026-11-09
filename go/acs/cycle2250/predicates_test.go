//go:build acs

package cycle2250

import (
	"encoding/json"
	"fmt"
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
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleBaselineSHA = "b25471c0ccaa9d1fbcc616a8a8777931e81658b1"
	salvageAfter     = 24
	uncommittedLine  = "uncommitted edit of the ended lane"
)

type hub struct {
	t    *testing.T
	repo *gittest.Repo
	base string
	now  time.Time
}

type lane struct {
	cycle  int
	path   string
	branch string
}

func newHub(t *testing.T) *hub {
	t.Helper()
	repo := gittest.Fixture(t)
	write(t, filepath.Join(repo.Dir, "f.txt"), "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	return &hub{t: t, repo: repo, base: filepath.Join(repo.Dir, ".evolve", "worktrees"), now: time.Now()}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (h *hub) addLane(cycle int, age time.Duration) lane {
	h.t.Helper()
	leaf := fmt.Sprintf("cycle-cd3ae73e-%d", cycle)
	l := lane{cycle: cycle, path: filepath.Join(h.base, leaf), branch: leaf}
	h.repo.Git("worktree", "add", "-q", l.path, "-b", l.branch)
	write(h.t, filepath.Join(l.path, "f.txt"), "base\nlane edit\n")
	h.repo.Git("-C", l.path, "commit", "-q", "-am", "lane work")
	write(h.t, filepath.Join(l.path, "f.txt"), "base\nlane edit\n"+uncommittedLine+"\n")
	mtime := h.now.Add(-age)
	if err := os.Chtimes(l.path, mtime, mtime); err != nil {
		h.t.Fatal(err)
	}
	return l
}

func (h *hub) opts(salvageAfterHours int) gc.WorktreeOptions {
	return gc.WorktreeOptions{
		ProjectRoot:  h.repo.Dir,
		WorktreeBase: h.base,
		EvolveDir:    filepath.Join(h.repo.Dir, ".evolve"),
		Policy:       gc.WorktreesPolicy{SalvageAfterHours: salvageAfterHours},
		Now:          func() time.Time { return h.now },
		Exec:         sysexec.DefaultRunner,
		PidAlive:     func(int) bool { return true },
		LeaseTTL:     time.Hour,
	}
}

func (h *hub) plan(o gc.WorktreeOptions) gc.WorktreeManifest {
	h.t.Helper()
	m, err := gc.PlanWorktrees(o)
	if err != nil {
		h.t.Fatalf("PlanWorktrees: %v", err)
	}
	return m
}

func (h *hub) apply(o gc.WorktreeOptions, m gc.WorktreeManifest) {
	h.t.Helper()
	if err := gc.ApplyWorktrees(o, m); err != nil {
		h.t.Fatalf("ApplyWorktrees: %v", err)
	}
}

func actionFor(m gc.WorktreeManifest, path string) gc.WorktreeAction {
	for _, it := range m.Items {
		if it.Path == path {
			return it.Action
		}
	}
	return ""
}

func onDisk(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (h *hub) writeQuotaPause(l lane) {
	h.t.Helper()
	runDir := filepath.Join(h.repo.Dir, ".evolve", "runs", fmt.Sprintf("cycle-%d", l.cycle))
	scope, err := json.Marshal(core.LaneScope{TodoIDs: []string{fmt.Sprintf("item-%d", l.cycle)}, GoalHash: "wave-goal"})
	if err != nil {
		h.t.Fatal(err)
	}
	write(h.t, filepath.Join(runDir, core.LaneScopeFile), string(scope))
	state, err := json.Marshal(map[string]any{
		"cycle_id": l.cycle,
		"phase":    "build",
		"checkpoint": map[string]any{
			"enabled":               true,
			"reason":                "quota-likely",
			"resumeFromPhase":       "build",
			"worktreePath":          l.path,
			"savedAt":               time.Now().UTC().Format(time.RFC3339),
			"autoResumeAttempts":    0,
			"autoResumeMaxAttempts": 3,
		},
	})
	if err != nil {
		h.t.Fatal(err)
	}
	write(h.t, filepath.Join(runDir, core.CycleStateFile), string(state))
	paused, err := core.PausedLaneScopes(h.repo.Dir, filepath.Join(h.repo.Dir, ".evolve"), core.ResumeOptions{})
	if err != nil {
		h.t.Fatal(err)
	}
	for _, p := range paused {
		if p.Cycle == l.cycle {
			return
		}
	}
	h.t.Fatalf("fixture error: cycle %d is not a paused lane scope (%+v)", l.cycle, paused)
}

func (h *hub) bindContinuation(scopeID string, l lane) {
	h.t.Helper()
	err := continuation.WriteRegistryEntry(h.repo.Dir, scopeID, continuation.Continuation{Worktree: l.path, Branch: l.branch, Cycle: l.cycle})
	if err != nil {
		h.t.Fatal(err)
	}
}

func TestC2250_001_TheDirtyLaneOfAnOldCycleWithNoRunDirectoryIsSavedAndRemoved(t *testing.T) {
	h := newHub(t)
	ended := h.addLane(1677, 48*time.Hour)
	tip := h.repo.Git("rev-parse", ended.branch)
	if onDisk(filepath.Join(h.repo.Dir, ".evolve", "runs", "cycle-1677")) {
		t.Fatal("fixture error: cycle 1677 must have no run directory")
	}
	o := h.opts(salvageAfter)

	m := h.plan(o)
	if got := actionFor(m, ended.path); got != gc.WorktreeActionSalvageRemove {
		t.Fatalf("RED: planned %q for the dirty lane of an old cycle with no run directory, want %q", got, gc.WorktreeActionSalvageRemove)
	}
	h.apply(o, m)

	if onDisk(ended.path) {
		t.Errorf("RED: %s is still on disk after the salvage-remove", ended.path)
	}
	if got := h.repo.Git("rev-parse", ended.branch); got != tip {
		t.Errorf("branch tip %q, want %q: the lane commits stay on the kept branch", got, tip)
	}
	patch := filepath.Join(gc.OperatorSalvageDir(o.EvolveDir), filepath.Base(ended.path), "uncommitted.patch")
	body, err := os.ReadFile(patch)
	if err != nil {
		t.Fatalf("RED: the lane was removed without a saved patch: %v", err)
	}
	if !strings.Contains(string(body), uncommittedLine) {
		t.Errorf("saved patch does not hold the uncommitted edit:\n%s", body)
	}
}

func TestC2250_002_AQuotaPausedLaneStaysWhileAnEndedSiblingIsReleased(t *testing.T) {
	h := newHub(t)
	paused := h.addLane(1700, 48*time.Hour)
	ended := h.addLane(1701, 48*time.Hour)
	h.writeQuotaPause(paused)
	o := h.opts(salvageAfter)

	m := h.plan(o)
	h.apply(o, m)

	if got := actionFor(m, ended.path); got != gc.WorktreeActionSalvageRemove {
		t.Errorf("RED: ended sibling planned %q, want %q: the keep rule must not keep every old lane", got, gc.WorktreeActionSalvageRemove)
	}
	if got := actionFor(m, paused.path); got == gc.WorktreeActionSalvageRemove || got == gc.WorktreeActionRemove {
		t.Errorf("RED: the quota-paused lane is planned %q, a paused lane must stay", got)
	}
	if !onDisk(paused.path) {
		t.Errorf("RED: the quota-paused lane %s was removed", paused.path)
	}
}

func TestC2250_003_AContinuationBoundLaneStaysWhileAnUnboundOldLaneIsReleased(t *testing.T) {
	h := newHub(t)
	bound := h.addLane(1710, 48*time.Hour)
	unbound := h.addLane(1711, 48*time.Hour)
	h.bindContinuation("item-1710", bound)
	o := h.opts(salvageAfter)

	m := h.plan(o)
	h.apply(o, m)

	if got := actionFor(m, unbound.path); got != gc.WorktreeActionSalvageRemove {
		t.Errorf("RED: unbound old lane planned %q, want %q: a registry entry of another branch keeps nothing here", got, gc.WorktreeActionSalvageRemove)
	}
	if got := actionFor(m, bound.path); got == gc.WorktreeActionSalvageRemove || got == gc.WorktreeActionRemove {
		t.Errorf("RED: the continuation-bound lane is planned %q, a continuation lane must stay", got)
	}
	if !onDisk(bound.path) {
		t.Errorf("RED: the continuation-bound lane %s was removed", bound.path)
	}
	if onDisk(unbound.path) {
		t.Errorf("RED: the unbound old lane %s is still on disk", unbound.path)
	}
}

func TestC2250_004_ALaneYoungerThanTheSalvageAgeStaysAndZeroMeansNever(t *testing.T) {
	h := newHub(t)
	young := h.addLane(1720, 23*time.Hour)
	old := h.addLane(1721, 25*time.Hour)

	m := h.plan(h.opts(salvageAfter))
	if got := actionFor(m, young.path); got != gc.WorktreeActionFlagDirty {
		t.Errorf("lane 23h old with a 24h salvage age planned %q, want %q", got, gc.WorktreeActionFlagDirty)
	}
	if got := actionFor(m, old.path); got != gc.WorktreeActionSalvageRemove {
		t.Errorf("RED: lane 25h old with a 24h salvage age planned %q, want %q", got, gc.WorktreeActionSalvageRemove)
	}

	never := h.plan(h.opts(0))
	for _, l := range []lane{young, old} {
		if got := actionFor(never, l.path); got != gc.WorktreeActionFlagDirty {
			t.Errorf("salvage_after_hours 0 planned %q for %s, want %q: zero means never", got, filepath.Base(l.path), gc.WorktreeActionFlagDirty)
		}
	}
}

func TestC2250_005_ALiveLaneOfAnOldCycleIsNeverPlanned(t *testing.T) {
	h := newHub(t)
	live := h.addLane(1730, 48*time.Hour)
	runDir := filepath.Join(h.repo.Dir, ".evolve", "runs", "cycle-1730")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-1730", OwnerPID: os.Getpid()}, h.now); err != nil {
		t.Fatal(err)
	}

	m := h.plan(h.opts(salvageAfter))

	if got := actionFor(m, live.path); got != "" {
		t.Errorf("live lane planned %q, want no item", got)
	}
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

type lineSpan struct{ from, to int }

func changedSpans(t *testing.T, root string) map[string][]lineSpan {
	t.Helper()
	spans := map[string][]lineSpan{}
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--unified=0", "--no-color", cycleBaselineSHA, "--", "go/internal/gc")
	if err != nil || code != 0 {
		t.Fatalf("git diff against the cycle baseline failed: code=%d err=%v %s", code, err, stderr)
	}
	var file string
	for _, line := range strings.Split(diff, "\n") {
		if rest, ok := strings.CutPrefix(line, "+++ b/"); ok {
			file = rest
			continue
		}
		m := hunkRe.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		from, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		to := from + count - 1
		if count == 0 {
			to = from
		}
		spans[file] = append(spans[file], lineSpan{from, to})
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", "go/internal/gc")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files failed: code=%d err=%v", code, err)
	}
	for _, f := range strings.Fields(untracked) {
		spans[f] = append(spans[f], lineSpan{1, 1 << 30})
	}
	return spans
}

func changedFunctions(t *testing.T, root string) []string {
	t.Helper()
	names := map[string]bool{}
	for file, spans := range changedSpans(t, root) {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, filepath.Join(root, file), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			start, end := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
			for _, s := range spans {
				if s.from <= end && s.to >= start {
					names[fn.Name.Name] = true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestC2250_006_EachChangedFunctionOfTheGcPackageHasFullCoverage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changed := changedFunctions(t, root)
	if len(changed) == 0 {
		t.Fatal("RED: no function of go/internal/gc changed against the cycle baseline, so the ended rule is not built")
	}
	goDir := filepath.Join(root, "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, "./internal/gc")
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/gc exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 20), acsassert.OutputTail(stderr, 20))
	}
	funcs, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover -func exit=%d err=%v\n%s", code, err, stderr)
	}
	for _, name := range changed {
		pct, err := acsassert.FuncCoverage(funcs, name)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct < 100 {
			t.Errorf("changed function %s has %.1f%% coverage, want 100%%", name, pct)
		}
	}
}

// acs-predicate: config-check
func TestC2250_007_TheBuildExplanationNamesThePlanDecisionAndComponent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2250-*.md"))
	if err != nil || len(docs) == 0 {
		t.Fatalf("RED: no build explanation docs/explain/builds/cycle-2250-*.md, the text that the pull request carries (err=%v)", err)
	}
	for _, id := range []string{"D6", "X5", "docs/plans/gc-closes-the-legacy-gap-2026-11.md"} {
		if !acsassert.FileMatchesRegex(t, docs[0], `(^|[^A-Za-z0-9])`+regexp.QuoteMeta(id)+`([^A-Za-z0-9]|$)`) {
			t.Errorf("RED: %s does not name %s", filepath.Base(docs[0]), id)
		}
	}
	rel, err := filepath.Rel(root, docs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
		t.Errorf("RED: %s is gitignored and is dropped at ship", rel)
	}
}
