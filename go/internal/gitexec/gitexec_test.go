package gitexec_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestGit_Capture_NonzeroExit_ReturnsCodeNotError(t *testing.T) {
	// `git diff --quiet` exits 1 to mean "there are differences" — a non-zero
	// exit that is NOT a failure. Capture must surface the code, not an error.
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git diff": {ExitCode: 1},
	}}
	g := gitexec.Git{Dir: "/wt", Exec: fake.Run}

	stdout, _, code, err := g.Capture(context.Background(), "diff", "--quiet")
	if err != nil {
		t.Fatalf("Capture err = %v, want nil (non-zero exit is not an error)", err)
	}
	if code != 1 {
		t.Errorf("exitCode = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if len(fake.Calls) != 1 || fake.Calls[0].Name != "git" || fake.Calls[0].Dir != "/wt" {
		t.Fatalf("recorded call = %+v, want one git call in /wt", fake.Calls)
	}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, []string{"diff", "--quiet"}) {
		t.Errorf("args = %v, want [diff --quiet]", got)
	}
}

func TestGit_Output_TrimsStdoutAndNonzeroIsError(t *testing.T) {
	ok := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git rev-parse": {Stdout: "  main\n"},
	}}
	got, err := (gitexec.Git{Exec: ok.Run}).Output(context.Background(), "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("Output err = %v", err)
	}
	if got != "main" {
		t.Errorf("Output = %q, want trimmed %q", got, "main")
	}

	bad := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git rev-parse": {ExitCode: 128, Stderr: "fatal: not a git repository"},
	}}
	if _, err := (gitexec.Git{Exec: bad.Run}).Output(context.Background(), "rev-parse", "HEAD"); err == nil {
		t.Error("Output must return an error on non-zero exit")
	}
}

func TestGit_Run_SuccessNilFailureErr(t *testing.T) {
	ok := &fixtures.FakeExec{} // zero value: success
	if err := (gitexec.Git{Exec: ok.Run}).Run(context.Background(), "add", "-A"); err != nil {
		t.Errorf("Run success = %v, want nil", err)
	}
	if got := ok.Calls[0].Args; !reflect.DeepEqual(got, []string{"add", "-A"}) {
		t.Errorf("Run dispatched git %v, want git add -A", got)
	}
	bad := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{"git add": {ExitCode: 1}}}
	if err := (gitexec.Git{Exec: bad.Run}).Run(context.Background(), "add", "-A"); err == nil {
		t.Error("Run must return an error on non-zero exit")
	}
}

func TestGit_HEAD_ReturnsTrimmedRevParse(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git rev-parse": {Stdout: "abc123def\n"},
	}}
	head, err := (gitexec.Git{Dir: "/r", Exec: fake.Run}).HEAD(context.Background())
	if err != nil {
		t.Fatalf("HEAD err = %v", err)
	}
	if head != "abc123def" {
		t.Errorf("HEAD = %q, want abc123def", head)
	}
	if fake.Calls[0].Key != "git rev-parse" ||
		!reflect.DeepEqual(fake.Calls[0].Args, []string{"rev-parse", "HEAD"}) {
		t.Errorf("ran %v args %v, want git rev-parse HEAD", fake.Calls[0].Key, fake.Calls[0].Args)
	}
}

func TestGit_DirtyPaths_SortedWithRenameOld(t *testing.T) {
	status := " M b.go\n?? a.txt\nR  old.go -> z.go\n"
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git status": {Stdout: status},
	}}
	paths, err := (gitexec.Git{Dir: "/r", Exec: fake.Run}).DirtyPaths(context.Background())
	if err != nil {
		t.Fatalf("DirtyPaths err = %v", err)
	}
	want := []string{"a.txt", "b.go", "old.go", "z.go"} // sorted; rename dirties BOTH sides
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("DirtyPaths = %v, want %v", paths, want)
	}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, []string{"status", "--porcelain", "-uall"}) {
		t.Errorf("ran git %v, want git status --porcelain -uall", got)
	}
}

func TestGit_DirtyPaths_CleanTreeReturnsEmpty(t *testing.T) {
	// A clean worktree yields empty porcelain output; strings.Split("", "\n")
	// gives [""], which the length guard must skip — no spurious "" path.
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git status": {Stdout: ""},
	}}
	paths, err := (gitexec.Git{Exec: fake.Run}).DirtyPaths(context.Background())
	if err != nil {
		t.Fatalf("DirtyPaths err = %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("clean tree DirtyPaths = %v, want empty", paths)
	}
}

func TestGit_DirtyPaths_NonzeroIsError(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git status": {ExitCode: 128, Stderr: "fatal: not a git repository"},
	}}
	if _, err := (gitexec.Git{Exec: fake.Run}).DirtyPaths(context.Background()); err == nil {
		t.Error("DirtyPaths must return an error on non-zero git status")
	}
}

func TestPorcelainPath_RenameAndPlain(t *testing.T) {
	cases := []struct{ line, path, old string }{
		{" M internal/foo.go", "internal/foo.go", ""},
		{"?? new.txt", "new.txt", ""},
		{"R  old.go -> new.go", "new.go", "old.go"},
		{"C  src.go -> copy.go", "copy.go", "src.go"},
		{"?? \"weird name.go\"", "weird name.go", ""},
		// git quotes space-containing paths; both rename sides must unquote. This
		// is the real `git status --porcelain -uall` output (verified empirically).
		{"R  \"old name.go\" -> \"new name.go\"", "new name.go", "old name.go"},
		{"x", "", ""}, // too short to hold a path — must not panic
	}
	for _, c := range cases {
		if got := gitexec.PorcelainPath(c.line); got != c.path {
			t.Errorf("PorcelainPath(%q) = %q, want %q", c.line, got, c.path)
		}
		if got := gitexec.PorcelainOldPath(c.line); got != c.old {
			t.Errorf("PorcelainOldPath(%q) = %q, want %q", c.line, got, c.old)
		}
	}
}

func TestWorktreeToken_StableAndDistinct(t *testing.T) {
	// Cycle worktree BRANCH names embed this token so concurrent `evolve loop`
	// runs in sibling worktrees of ONE repo (which share a single branch
	// namespace) never collide on `cycle-<N>` — the collision that silently
	// dropped a run's phases into the main tree and failed the cycle.
	a := gitexec.WorktreeToken("/Users/x/ai/evolve-loop-campaign")
	b := gitexec.WorktreeToken("/Users/x/ai/evolve-loop-dossier")
	if a == "" || b == "" {
		t.Fatalf("token must be non-empty: a=%q b=%q", a, b)
	}
	if a == b {
		t.Fatalf("distinct roots must yield distinct tokens; both = %q (would collide)", a)
	}
	// Stable across calls: a resumed loop (same root) must reuse the SAME branch.
	if again := gitexec.WorktreeToken("/Users/x/ai/evolve-loop-campaign"); again != a {
		t.Fatalf("token not stable for one root: %q then %q", a, again)
	}
	// A trailing-slash / uncleaned variant of the SAME dir tokenizes identically.
	if got := gitexec.WorktreeToken("/Users/x/ai/evolve-loop-campaign/"); got != a {
		t.Fatalf("uncleaned variant token = %q, want %q (same dir)", got, a)
	}
	// Branch-safe: it embeds directly in a git ref, so no separators/whitespace.
	for _, tok := range []string{a, b} {
		if strings.ContainsAny(tok, "/ \t\n~^:?*[\\") {
			t.Errorf("token %q must be a valid git-ref fragment", tok)
		}
	}
}

func TestDefault_InjectsRunnerAndDir(t *testing.T) {
	g := gitexec.Default("/work")
	if g.Dir != "/work" {
		t.Errorf("Default Dir = %q, want /work", g.Dir)
	}
	if g.Exec == nil {
		t.Error("Default must inject a non-nil runner")
	}
}
