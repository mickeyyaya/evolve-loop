//go:build acs

package cycle1929

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1929: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1929-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func isolatedEnv(root string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "EVOLVE_PROJECT_ROOT="+root, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func evolveInbox(t *testing.T, root string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	argv := append([]string{"inbox"}, args...)
	cmd := exec.CommandContext(ctx, evolveBin(t), argv...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = root, isolatedEnv(root), 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("evolve %s did not return within its budget: %s", strings.Join(argv, " "), r)
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %s: %v", strings.Join(argv, " "), err)
	}
	return r
}

type lintEntry struct {
	ID                 string   `json:"id"`
	ProtectedBasenames []string `json:"protected_basenames"`
}

type lintDoc struct {
	Undeclared []lintEntry `json:"undeclared"`
}

func decodeLint(t *testing.T, r result) lintDoc {
	t.Helper()
	var doc lintDoc
	if err := json.Unmarshal(cliEnvelopeData(t, r.stdout), &doc); err != nil {
		t.Fatalf("evolve inbox lint --json printed no JSON document: %v\n%s", err, r)
	}
	return doc
}

func entryFor(doc lintDoc, idFragment string) (lintEntry, bool) {
	for _, e := range doc.Undeclared {
		if strings.Contains(e.ID, idFragment) {
			return e, true
		}
	}
	return lintEntry{}, false
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeItem(t *testing.T, root, name string, item map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".evolve", "inbox", name+".json"), string(raw))
}

const protectedFile = "go/internal/core/disposition_gate.go"

func mixedInboxRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, protectedFile), "package core\n")
	writeFile(t, filepath.Join(root, "go/internal/widget/plain_helper.go"), "package widget\n")
	writeItem(t, root, "undeclared-cites-protected", map[string]any{
		"id": "undeclared-cites-protected", "kind": "bug", "weight": 0.9, "priority_class": "correctness",
		"title":   "the verdict check in disposition_gate.go misreads a row",
		"summary": "disposition_gate.go drops the second row; plain_helper.go formats it.",
	})
	writeItem(t, root, "undeclared-cites-nothing", map[string]any{
		"id": "undeclared-cites-nothing", "kind": "bug", "weight": 0.8, "priority_class": "correctness",
		"title": "the queue renders slowly", "summary": "no file is named here",
	})
	writeItem(t, root, "undeclared-basename-only-files", map[string]any{
		"id": "undeclared-basename-only-files", "kind": "bug", "weight": 0.7, "priority_class": "correctness",
		"title": "a fix in one file", "files": []string{"plain_helper.go"},
	})
	writeItem(t, root, "declared-by-files", map[string]any{
		"id": "declared-by-files", "kind": "bug", "weight": 0.6, "priority_class": "correctness",
		"title": "a fix whose surface is typed", "summary": "disposition_gate.go is only context",
		"files": []string{"go/internal/widget/plain_helper.go"},
	})
	writeItem(t, root, "declared-by-pipeline-kind", map[string]any{
		"id": "declared-by-pipeline-kind", "kind": "pipeline-repair", "weight": 0.5, "priority_class": "correctness",
		"title": "a pipeline repair", "summary": "disposition_gate.go misbehaves",
	})
	writeItem(t, root, "console-routed-undeclared", map[string]any{
		"id": "console-routed-undeclared", "kind": "bug", "route": "console", "weight": 0.4, "priority_class": "correctness",
		"title": "an operator-owned item", "summary": "disposition_gate.go needs an operator",
	})
	return root
}

func TestC1929_001_LintJSONReportsEveryUndeclaredDispatchableItemWithItsCitedProtectedBasenames(t *testing.T) {
	root := mixedInboxRoot(t)
	r := evolveInbox(t, root, "lint", "--json")
	if r.code != 1 {
		t.Errorf("evolve inbox lint --json over undeclared items: exit %d, want 1\n%s", r.code, r)
	}
	doc := decodeLint(t, r)
	wantBasenames := map[string][]string{
		"undeclared-cites-protected":     {"disposition_gate.go"},
		"undeclared-cites-nothing":       nil,
		"undeclared-basename-only-files": nil,
	}
	for id, want := range wantBasenames {
		e, ok := entryFor(doc, id)
		if !ok {
			t.Errorf("undeclared dispatchable item %q is not reported\n%s", id, r)
			continue
		}
		got := slices.Clone(e.ProtectedBasenames)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("item %q protected_basenames = %q, want %q", id, e.ProtectedBasenames, want)
		}
	}
	for _, id := range []string{"declared-by-files", "declared-by-pipeline-kind", "console-routed-undeclared"} {
		if _, ok := entryFor(doc, id); ok {
			t.Errorf("item %q declares a surface or is not dispatchable, yet the lint reports it\n%s", id, r)
		}
	}
	if len(doc.Undeclared) != len(wantBasenames) {
		t.Errorf("lint reported %d items, want %d: %+v", len(doc.Undeclared), len(wantBasenames), doc.Undeclared)
	}
}

func TestC1929_002_LintTextNamesTheBasenameAndNeverGuessesItsPath(t *testing.T) {
	root := mixedInboxRoot(t)
	r := evolveInbox(t, root, "lint")
	if r.code != 1 {
		t.Errorf("evolve inbox lint over undeclared items: exit %d, want 1\n%s", r.code, r)
	}
	for _, id := range []string{"undeclared-cites-protected", "undeclared-cites-nothing", "undeclared-basename-only-files"} {
		if !strings.Contains(r.stdout, id) {
			t.Errorf("lint text does not name undeclared item %q\n%s", id, r)
		}
	}
	if !strings.Contains(r.stdout, "disposition_gate.go") {
		t.Errorf("lint text does not name the cited protected basename disposition_gate.go\n%s", r)
	}
	if strings.Contains(r.stdout+r.stderr, protectedFile) {
		t.Errorf("lint output guesses the path %s for a basename the item never placed\n%s", protectedFile, r)
	}
	for _, id := range []string{"declared-by-files", "declared-by-pipeline-kind", "console-routed-undeclared"} {
		if strings.Contains(r.stdout, id) {
			t.Errorf("lint text names %q, which declares a surface or is not dispatchable\n%s", id, r)
		}
	}
}

func TestC1929_003_LintIsAnInboxVerbThatExitsZeroOnADeclaredInboxAndTenOnABadArgument(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, "declared-only", map[string]any{
		"id": "declared-only", "kind": "bug", "weight": 0.5, "priority_class": "correctness",
		"title": "typed", "files": []string{"go/internal/widget/plain_helper.go"},
	})
	clean := evolveInbox(t, root, "lint", "--json")
	if clean.code != 0 {
		t.Errorf("evolve inbox lint --json over a declared inbox: exit %d, want 0\n%s", clean.code, clean)
	}
	if doc := decodeLint(t, clean); len(doc.Undeclared) != 0 {
		t.Errorf("a declared inbox reports undeclared items: %+v", doc.Undeclared)
	}
	if bad := evolveInbox(t, root, "lint", "--no-such-flag"); bad.code != 10 {
		t.Errorf("evolve inbox lint --no-such-flag: exit %d, want 10\n%s", bad.code, bad)
	}
	help := evolveInbox(t, root, "--help")
	if help.code != 0 || !strings.Contains(help.stdout, "evolve inbox lint") {
		t.Errorf("evolve inbox --help does not list the lint verb\n%s", help)
	}
}

var preCurationRecords = []struct{ id, path string }{
	{"resume-does-not-refresh-a-stale-worktree-base", ".evolve/inbox/2026-09-13T23-30-00Z-resume-does-not-refresh-a-stale-worktree-base.json"},
	{"carryover-queue-unbounded-by-design", ".evolve/inbox/2026-08-27T16-00-00Z-carryover-queue-unbounded-by-design.json"},
	{"acs-verdict-filename-literal-sweep", ".evolve/inbox/acs-verdict-filename-literal-sweep.json"},
}

const curationCommit = "a603c42f3f2c0fa28ed22f8d4dadd03dc4fd8de8"

func TestC1929_004_ReplayTheThreeHandTypedItemsOfSeptember26AreReportedUndeclared(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	root := t.TempDir()
	for _, rec := range preCurationRecords {
		body, stderr, code, err := acsassert.SubprocessOutput("git", "-C", repo, "show", curationCommit+"^:"+rec.path)
		if err != nil || code != 0 {
			t.Fatalf("read the pre-curation record %s: exit %d, %v\n%s", rec.path, code, err, stderr)
		}
		writeFile(t, filepath.Join(root, rec.path), body)
	}
	r := evolveInbox(t, root, "lint", "--json")
	if r.code != 1 {
		t.Errorf("evolve inbox lint --json over the pre-curation records: exit %d, want 1\n%s", r.code, r)
	}
	doc := decodeLint(t, r)
	for _, rec := range preCurationRecords {
		if _, ok := entryFor(doc, rec.id); !ok {
			t.Errorf("pre-curation record %s is not reported undeclared\n%s", rec.id, r)
		}
	}
	if len(doc.Undeclared) != len(preCurationRecords) {
		t.Errorf("lint reported %d items, want %d: %+v", len(doc.Undeclared), len(preCurationRecords), doc.Undeclared)
	}
}

func TestC1929_005_TheHaltAndRetroFilersDeclareASurfaceOnEveryItemTheyFile(t *testing.T) {
	names := []string{
		"TestInboxWriters_EveryAutofiledItemDeclaresSurface",
		"TestInboxWriters_EveryAutofiledItemCarriesAKnownClass",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "github.com/mickeyyaya/evolve-loop/go/cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC1929_006_TheTargetFilesStayOutsideTheProtectedSurface(t *testing.T) {
	targets := []string{
		"go/cmd/evolve/cmd_inbox.go",
		"go/internal/inboxbatch/item.go",
		"go/cmd/evolve/cmd_loop_escalation.go",
		"go/internal/faillearn/inbox.go",
		"go/internal/retrofile/retrofile.go",
	}
	for _, p := range targets {
		if guards.IsProtectedSurface(p) {
			t.Errorf("target %s is on the protected surface, so a lane cannot ship it", p)
		}
	}
	if !guards.IsProtectedSurface(protectedFile) {
		t.Errorf("the fixture file %s is no longer protected: the lint predicates lose their protected basename", protectedFile)
	}
}

func TestC1929_007_TheFailLearningFloorGoldenStaysByteIdentical(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	const goldenDir = "go/internal/core/failurelearning"
	if _, stderr, code, err := acsassert.SubprocessOutput("git", "-C", repo, "diff", "--quiet", "HEAD", "--", goldenDir); err != nil || code != 0 {
		t.Errorf("the protected %s changed; a lane cannot ship an edit there: exit %d, %v\n%s", goldenDir, code, err, stderr)
	}
	names := []string{"TestWriteFloor_WritesLessonAndReportByteIdenticalToTheGolden"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(repo, "go"),
		Package: "github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func lintBasenamesFor(t *testing.T, root, id string) []string {
	t.Helper()
	r := evolveInbox(t, root, "lint", "--json")
	e, ok := entryFor(decodeLint(t, r), id)
	if !ok {
		t.Fatalf("undeclared item %q is not reported\n%s", id, r)
	}
	got := slices.Clone(e.ProtectedBasenames)
	slices.Sort(got)
	return got
}

func TestC1929_008_LintCountsOnlyTrackedFilesAsProtectedBasenames(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("go/internal/guards/tracked_rule.go", "package guards\n")
	repo.Git("add", "go/internal/guards/tracked_rule.go")
	repo.Git("commit", "-q", "-m", "track one protected file")
	repo.Write("go/internal/guards/untracked_scratch.go", "package guards\n")
	writeItem(t, repo.Dir, "cites-tracked-and-untracked", map[string]any{
		"id": "cites-tracked-and-untracked", "kind": "bug", "weight": 0.9, "priority_class": "correctness",
		"title":   "tracked_rule.go misreads a row",
		"summary": "untracked_scratch.go is a local scratch file",
	})
	got := lintBasenamesFor(t, repo.Dir, "cites-tracked-and-untracked")
	if want := []string{"tracked_rule.go"}; !slices.Equal(got, want) {
		t.Errorf("protected_basenames = %q, want %q: an untracked file on disk is not repository state", got, want)
	}
}

func TestC1929_009_LintFindsATrackedProtectedFileUnderANestedRunsInboxOrWorktreesDirectory(t *testing.T) {
	for _, dir := range []string{"runs", "inbox", "worktrees"} {
		t.Run(dir, func(t *testing.T) {
			repo := gittest.Fixture(t)
			nested := "go/internal/guards/" + dir + "/nested_rule.go"
			repo.Write(nested, "package "+dir+"\n")
			repo.Git("add", nested)
			repo.Git("commit", "-q", "-m", "track a nested protected file")
			writeItem(t, repo.Dir, "cites-nested", map[string]any{
				"id": "cites-nested", "kind": "bug", "weight": 0.9, "priority_class": "correctness",
				"title": "nested_rule.go drops a row",
			})
			got := lintBasenamesFor(t, repo.Dir, "cites-nested")
			if want := []string{"nested_rule.go"}; !slices.Equal(got, want) {
				t.Errorf("protected_basenames = %q, want %q: tracked %s is protected", got, want, nested)
			}
		})
	}
}

func cliEnvelopeData(t *testing.T, stdout string) []byte {
	t.Helper()
	var e struct {
		Schema string          `json:"schema"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &e); err != nil || e.Schema == "" {
		t.Fatalf("stdout is not one cli-contract envelope: %v\n%s", err, stdout)
	}
	return e.Data
}
