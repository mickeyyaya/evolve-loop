package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func adoptableRoot(t *testing.T, laneA string) (string, continuation.Continuation) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "base\n")
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "-b", "cycle-2035")
	repo.Write("a.txt", laneA)
	repo.Write("lane.go", "package lane\n")
	repo.Git("add", "a.txt", "lane.go")
	repo.Git("commit", "-q", "-m", "salvage snapshot")
	snapshot := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "main")
	repo.Write("a.txt", "main moved\n")
	repo.Git("commit", "-q", "-am", "main moves on")
	ws := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-2035")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	findings := filepath.Join(ws, "audit-fail-reason.json")
	if err := os.WriteFile(findings, []byte(`{"phase":"audit","reasons":["lane test red"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo.Dir, continuation.Continuation{Branch: "cycle-2035", SnapshotSHA: snapshot, BaseSHA: base, FindingsPath: findings, Cycle: 2035}
}

func fileStampedItem(t *testing.T, root, id string, c *continuation.Continuation) {
	t.Helper()
	item := map[string]any{"id": id, "title": "t", "kind": "fix"}
	if c != nil {
		item["continuation"] = c
	}
	body, _ := json.Marshal(item)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, id+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func adoptRun(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runContinuation(append([]string{"adopt"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunContinuationAdopt_TheItemsStampGivesAWorktreeAndAFindingsFile(t *testing.T) {
	t.Parallel()
	root, c := adoptableRoot(t, "base\n")
	fileStampedItem(t, root, "retro-minting", &c)
	wt := filepath.Join(t.TempDir(), "adopted")

	code, out, se := adoptRun("retro-minting", "--worktree", wt, "--project-root", root)

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, se)
	}
	for _, f := range []string{"lane.go", "a.txt"} {
		if _, err := os.Stat(filepath.Join(wt, f)); err != nil {
			t.Errorf("worktree misses %s: %v", f, err)
		}
	}
	if body, _ := os.ReadFile(filepath.Join(wt, "a.txt")); string(body) != "main moved\n" {
		t.Errorf("a.txt = %q; the worktree must hold main merged into the snapshot", body)
	}
	for _, want := range []string{wt, wt + ".findings.md", "adopt/retro-minting", "cycle 2035"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout must name %q:\n%s", want, out)
		}
	}
	if body, err := os.ReadFile(wt + ".findings.md"); err != nil || !strings.Contains(string(body), "lane test red") {
		t.Errorf("findings file: %v\n%s", err, body)
	}
}

func TestRunContinuationAdopt_FallsBackToTheRegistryBinding(t *testing.T) {
	t.Parallel()
	root, c := adoptableRoot(t, "base\n")
	fileStampedItem(t, root, "scoped", nil)
	if err := continuation.WriteRegistryEntry(root, "scoped", c); err != nil {
		t.Fatal(err)
	}

	code, _, se := adoptRun("scoped", "--project-root", root, "--worktree", filepath.Join(t.TempDir(), "w"))

	if code != 0 {
		t.Errorf("a registry binding must adopt: code = %d, stderr = %s", code, se)
	}
}

func TestRunContinuationAdopt_RefusesAConflictWithTheFileList(t *testing.T) {
	t.Parallel()
	root, c := adoptableRoot(t, "lane edit\n")
	fileStampedItem(t, root, "retro-minting", &c)
	wt := filepath.Join(t.TempDir(), "adopted")

	code, _, se := adoptRun("retro-minting", "--worktree", wt, "--project-root", root)

	if code != 1 || !strings.Contains(se, "a.txt") || !strings.Contains(se, "conflicts with main") {
		t.Errorf("code = %d, stderr = %q; want 1 naming the conflicting file", code, se)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("a refused adoption made a worktree: %v", err)
	}
}

func TestRunContinuationAdopt_RefusesWithoutABindingOrWithBadArguments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fileStampedItem(t, root, "plain", nil)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "typed.json"), []byte(`{"id":"typed","title":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(continuation.RegistryPath(broken)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(broken), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"plain", "--worktree", "w", "--project-root", root}, 1, "no continuation binding"},
		{[]string{"ghost", "--worktree", "w", "--project-root", root}, 1, "no continuation binding"},
		{[]string{"plain", "--worktree", "w", "--project-root", broken}, 1, "registry unreadable"},
		{[]string{"typed", "--worktree", "w", "--project-root", root}, 1, "unreadable"},
		{[]string{"plain", "--project-root", root}, 10, "--worktree"},
		{[]string{}, 10, "<item-id>"},
		{[]string{"--worktree", "w", "--project-root", root}, 10, "<item-id>"},
		{[]string{"plain", "extra", "--worktree", "w", "--project-root", root}, 10, "<item-id>"},
		{[]string{"plain", "--worktree", "w", "--project-root", root, "extra"}, 10, "<item-id>"},
		{[]string{"plain", "--bogus"}, 10, ""},
	} {
		code, _, se := adoptRun(tc.args...)
		if code != tc.code || !strings.Contains(se, tc.want) {
			t.Errorf("%v: code = %d, stderr = %q; want %d naming %q", tc.args, code, se, tc.code, tc.want)
		}
	}
}

func TestRunContinuation_DispatchesEachSubcommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{nil, 10, "continuation adopt <item-id> --worktree <path>"},
		{[]string{"bogus"}, 10, "want: list, release, adopt"},
		{[]string{"list", "--project-root", root}, 0, ""},
		{[]string{"release", "--project-root", root}, 10, "exactly one <scope-id>"},
		{[]string{"adopt"}, 10, "<item-id>"},
	} {
		var out, errb bytes.Buffer
		if code := runContinuation(tc.args, nil, &out, &errb); code != tc.code || !strings.Contains(errb.String(), tc.want) {
			t.Errorf("%v: code = %d, stderr = %q; want %d naming %q", tc.args, code, errb.String(), tc.code, tc.want)
		}
	}
}

func tildeBinding(t *testing.T, root string, c continuation.Continuation) continuation.Continuation {
	t.Helper()
	t.Setenv("HOME", root)
	rel, err := filepath.Rel(root, c.FindingsPath)
	if err != nil {
		t.Fatal(err)
	}
	c.FindingsPath = "~/" + rel
	return c
}

func TestRunContinuationAdopt_ATildePathOnTheStampIsExpanded(t *testing.T) {
	root, c := adoptableRoot(t, "base\n")
	stamp := tildeBinding(t, root, c)
	fileStampedItem(t, root, "tilde-stamp", &stamp)
	wt := filepath.Join(t.TempDir(), "w")

	if code, _, se := adoptRun("tilde-stamp", "--worktree", wt, "--project-root", root); code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, se)
	}
	if body, _ := os.ReadFile(wt + ".findings.md"); !strings.Contains(string(body), "lane test red") {
		t.Errorf("a ~ findings path on the stamp must be expanded:\n%s", body)
	}
}

func TestRunContinuationAdopt_ATildePathInTheRegistryIsExpanded(t *testing.T) {
	root, c := adoptableRoot(t, "base\n")
	fileStampedItem(t, root, "tilde-registry", nil)
	if err := continuation.WriteRegistryEntry(root, "tilde-registry", tildeBinding(t, root, c)); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "w")

	if code, _, se := adoptRun("tilde-registry", "--worktree", wt, "--project-root", root); code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, se)
	}
	if body, _ := os.ReadFile(wt + ".findings.md"); !strings.Contains(string(body), "lane test red") {
		t.Errorf("a ~ findings path in the registry must be expanded:\n%s", body)
	}
}

func TestRunContinuationAdopt_AStampWithoutASnapshotFallsBackToTheRegistry(t *testing.T) {
	t.Parallel()
	root, c := adoptableRoot(t, "base\n")
	fileStampedItem(t, root, "partial", &continuation.Continuation{Cycle: 2035})
	if err := continuation.WriteRegistryEntry(root, "partial", c); err != nil {
		t.Fatal(err)
	}

	if code, _, se := adoptRun("partial", "--worktree", filepath.Join(t.TempDir(), "w"), "--project-root", root); code != 0 {
		t.Errorf("a stamp with no snapshot is no binding, so the registry decides: code = %d, stderr = %s", code, se)
	}
}
