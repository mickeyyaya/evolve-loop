package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedGit is a fake gitFn. respond is keyed off the call's args; every call
// is recorded so assertions can verify which git verbs ran.
type scriptedGit struct {
	calls   [][]string
	respond func(args []string) (string, int, error)
}

func (s *scriptedGit) capture(_ context.Context, _ string, args ...string) (string, int, error) {
	s.calls = append(s.calls, append([]string(nil), args...))
	return s.respond(args)
}

// ran reports whether any recorded call's args, joined by spaces, contains the
// joined want tokens (a loose contains-match good enough for verb assertions).
func (s *scriptedGit) ran(want ...string) bool {
	needle := strings.Join(want, " ")
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), needle) {
			return true
		}
	}
	return false
}

func joined(args []string) string { return strings.Join(args, " ") }

const cflags = "docs/architecture/control-flags.md"

// recordingRegen returns a regenFn that records the paths it was asked to
// regenerate, optionally failing.
func recordingRegen(failOn string) (func(context.Context, string, string) error, *[]string) {
	var got []string
	fn := func(_ context.Context, _ string, p string) error {
		got = append(got, p)
		if p == failOn {
			return errors.New("regen boom")
		}
		return nil
	}
	return fn, &got
}

func TestRebaseWithDerivedRegen_CleanRebase(t *testing.T) {
	g := &scriptedGit{respond: func(args []string) (string, int, error) {
		if joined(args) != "rebase main" {
			t.Fatalf("clean rebase should issue only 'rebase main', got %q", joined(args))
		}
		return "", 0, nil
	}}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false), got (%v,%v)", ok, conflict)
	}
	if len(*got) != 0 {
		t.Fatalf("regen must not run on a clean rebase, ran for %v", *got)
	}
}

func TestRebaseWithDerivedRegen_AllDerivedConflict_Resolves(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil // conflict
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return cflags + "\n", 0, nil
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			return "", 0, nil // continue completes the rebase
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false), got (%v,%v)", ok, conflict)
	}
	if len(*got) != 1 || (*got)[0] != cflags {
		t.Fatalf("regen paths = %v, want [%s]", *got, cflags)
	}
	if !g.ran("add", "--", cflags) {
		t.Fatal("must `git add` the regenerated derived artifact")
	}
	if !g.ran("rebase", "--continue") {
		t.Fatal("must continue the rebase after resolving")
	}
	if g.ran("rebase", "--abort") {
		t.Fatal("must NOT abort when all conflicts are derived + resolved")
	}
}

func TestRebaseWithDerivedRegen_NonDerivedConflict_AbortsToDebugger(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return "go/internal/flagregistry/registry_table.go\n", 0, nil
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q (must not regen/continue a real conflict)", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || !conflict {
		t.Fatalf("want (false,true) for a real SSOT conflict, got (%v,%v)", ok, conflict)
	}
	if len(*got) != 0 {
		t.Fatalf("regen must NOT run on a non-derived conflict, ran for %v", *got)
	}
	if !g.ran("rebase", "--abort") {
		t.Fatal("must abort a non-derived conflict so it routes to the debugger")
	}
}

func TestRebaseWithDerivedRegen_MixedConflict_Aborts(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			// one derived + one real conflict in the same step
			return cflags + "\ngo/internal/core/orchestrator.go\n", 0, nil
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || !conflict {
		t.Fatalf("want (false,true) when ANY conflict is non-derived, got (%v,%v)", ok, conflict)
	}
	if len(*got) != 0 {
		t.Fatalf("regen must not run when the set is not all-derived, ran for %v", *got)
	}
}

func TestRebaseWithDerivedRegen_MultiCommitDerivedConflicts_Resolves(t *testing.T) {
	continues := 0
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return cflags + "\n", 0, nil
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			continues++
			if continues == 1 {
				return "", 1, nil // a SECOND replayed commit also conflicts on the doc
			}
			return "", 0, nil // resolved on the second continue
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false) across multiple derived-conflict commits, got (%v,%v)", ok, conflict)
	}
	if len(*got) != 2 {
		t.Fatalf("regen should run once per conflicting commit, ran %d time(s)", len(*got))
	}
}

func TestRebaseWithDerivedRegen_RegenFails_Aborts(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return cflags + "\n", 0, nil
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, _ := recordingRegen(cflags) // regen fails for control-flags.md
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || conflict {
		t.Fatalf("want (false,false) on regen failure (infra, not overlap), got (%v,%v)", ok, conflict)
	}
	if !g.ran("rebase", "--abort") {
		t.Fatal("must abort when regeneration fails")
	}
}

func TestRebaseWithDerivedRegen_InfraFailureNoUnmerged_Aborts(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "fatal: something broke", 128, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return "", 0, nil // no unmerged paths → not a conflict
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || conflict {
		t.Fatalf("want (false,false) for an infra failure with no conflicts, got (%v,%v)", ok, conflict)
	}
	if len(*got) != 0 {
		t.Fatal("regen must not run on an infra failure")
	}
}

func TestRebaseWithDerivedRegen_EmptyCommitAfterResolve_Skips(t *testing.T) {
	diffCalls := 0
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			diffCalls++
			// pre-loop + first loop step see the conflict; after the (empty) continue
			// the commit has no unmerged paths.
			if diffCalls <= 2 {
				return cflags + "\n", 0, nil
			}
			return "", 0, nil
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			return "nothing to commit", 1, nil // commit became empty after resolution
		case strings.Contains(j, "rebase --skip"):
			return "", 0, nil // skip finishes the rebase
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, _ := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false) when an emptied commit is skipped, got (%v,%v)", ok, conflict)
	}
	if !g.ran("rebase", "--skip") {
		t.Fatal("must `rebase --skip` a commit emptied by resolution")
	}
}

func TestRebaseWithDerivedRegen_ContinueNeverConverges_AbortsAtBound(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return cflags + "\n", 0, nil // perpetual derived conflict
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			return "", 1, nil // never converges
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || conflict {
		t.Fatalf("want (false,false) at the replay-step bound, got (%v,%v)", ok, conflict)
	}
	if len(*got) != maxRebaseContinueSteps {
		t.Fatalf("expected regen once per bounded step (%d), got %d", maxRebaseContinueSteps, len(*got))
	}
	if !g.ran("rebase", "--abort") {
		t.Fatal("must abort after exceeding the replay-step bound")
	}
}

func TestRegenerateDerivedArtifact_UnregisteredPath_Errors(t *testing.T) {
	err := regenerateDerivedArtifact(context.Background(), "/wt", "no/such/file.md")
	if err == nil {
		t.Fatal("want an error for an unregistered derived-artifact path")
	}
	if !strings.Contains(err.Error(), "no regenerator registered") {
		t.Fatalf("error should name the missing registration, got %v", err)
	}
}

// TestRegenerateDerivedArtifact_Integration exercises the real production path:
// `go run ./cmd/evolve flags generate` from the worktree's go/ with
// EVOLVE_WORKTREE_ROOT set. Run against the repo itself — control-flags.md is
// committed in sync with the registry, so flagsRun's up-to-date guard makes this
// a no-op write (the file is byte-identical before/after). Compiles cmd/evolve,
// so it is skipped under -short.
func TestRegenerateDerivedArtifact_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles cmd/evolve via `go run`; skipped under -short")
	}
	root := repoRootForTest(t)
	docPath := filepath.Join(root, cflags)
	before, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", cflags, err)
	}
	if err := regenerateDerivedArtifact(context.Background(), root, cflags); err != nil {
		t.Fatalf("regenerate failed: %v", err)
	}
	after, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("re-read %s: %v", cflags, err)
	}
	if string(before) != string(after) {
		// Restore so a drift here does not leave the worktree dirty.
		_ = os.WriteFile(docPath, before, 0o644)
		t.Fatalf("%s changed after regeneration — the committed projection is out of sync with the registry", cflags)
	}
}

func TestRebaseWithDerivedRegen_GitAddFails_Aborts(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return cflags + "\n", 0, nil
		case strings.HasPrefix(j, "add -- "):
			return "", 1, nil // staging the regenerated file fails
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if ok || conflict {
		t.Fatalf("want (false,false) when `git add` fails, got (%v,%v)", ok, conflict)
	}
	if len(*got) != 1 {
		t.Fatalf("regen should run once before the failed add, got %d", len(*got))
	}
	if !g.ran("rebase", "--abort") {
		t.Fatal("must abort when `git add` fails")
	}
}

func TestRebaseCycleBranchOntoMain_EmptyWorktree(t *testing.T) {
	ok, conflict := rebaseCycleBranchOntoMain(context.Background(), "")
	if ok || conflict {
		t.Fatalf("empty worktree must return (false,false), got (%v,%v)", ok, conflict)
	}
}

// TestRebaseCycleBranchOntoMain_RealGit_NonDerivedConflict drives the production
// wiring against a real temp repo where the conflict is a NON-derived file, so the
// production regenerator (go run) is never invoked. Covers the delegation in
// rebaseCycleBranchOntoMain end-to-end against real git.
func TestRebaseCycleBranchOntoMain_RealGit_NonDerivedConflict(t *testing.T) {
	dir := initConflictRepo(t, "foo.go")
	ok, conflict := rebaseCycleBranchOntoMain(context.Background(), dir)
	if ok || !conflict {
		t.Fatalf("a non-derived conflict must route to the debugger: got (%v,%v)", ok, conflict)
	}
	out, _, _ := gitCapture(context.Background(), dir, "status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("worktree must be clean after a real rebase --abort, got:\n%s", out)
	}
}

// TestRebaseWithDerivedRegen_RealGit_DerivedConflict validates the ACTUAL git
// command sequence (rebase main → diff --diff-filter=U → add → -c core.editor=true
// rebase --continue) against real git, with a fake regenerator standing in for
// `flags generate` (so the test needs no go module). This is the real-git proof
// the fakes cannot give.
func TestRebaseWithDerivedRegen_RealGit_DerivedConflict(t *testing.T) {
	dir := initConflictRepo(t, cflags)
	regen := func(_ context.Context, wt, p string) error {
		return os.WriteFile(filepath.Join(wt, p), []byte("regenerated\nshared\n"), 0o644)
	}
	ok, conflict := rebaseWithDerivedRegen(context.Background(), dir, gitCapture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("real-git derived conflict should auto-resolve: got (%v,%v)", ok, conflict)
	}
	out, _, _ := gitCapture(context.Background(), dir, "status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("worktree must be clean after a resolved rebase, got:\n%s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, cflags))
	if string(got) != "regenerated\nshared\n" {
		t.Fatalf("resolved file content = %q, want the regenerated projection", string(got))
	}
}

// initConflictRepo builds a temp git repo where branch `cycle` and `main` diverge
// on relFile, so rebasing `cycle` onto `main` conflicts. Leaves `cycle` checked out.
func initConflictRepo(t *testing.T, relFile string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if _, code, err := gitCapture(context.Background(), dir, args...); err != nil || code != 0 {
			t.Fatalf("git %v: code=%d err=%v", args, code, err)
		}
	}
	write := func(content string) {
		t.Helper()
		p := filepath.Join(dir, relFile)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init")
	git("checkout", "-b", "main")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "test")
	git("config", "commit.gpgsign", "false")
	write("base\nshared\n")
	git("add", "-A")
	git("commit", "-m", "base")
	git("checkout", "-b", "cycle")
	write("cycle-change\nshared\n")
	git("add", "-A")
	git("commit", "-m", "cycle change")
	git("checkout", "main")
	write("main-change\nshared\n")
	git("add", "-A")
	git("commit", "-m", "main change")
	git("checkout", "cycle")
	return dir
}

func TestRebaseWithDerivedRegen_SkipRetriesThenCompletes(t *testing.T) {
	diffCalls, skips := 0, 0
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			diffCalls++
			if diffCalls <= 2 { // pre-loop + first step see the conflict
				return cflags + "\n", 0, nil
			}
			return "", 0, nil // emptied after resolution
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			return "nothing to commit", 1, nil
		case strings.Contains(j, "rebase --skip"):
			skips++
			if skips == 1 {
				return "transient", 1, nil // first skip fails → loop continues
			}
			return "", 0, nil // second skip finishes the rebase
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, _ := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false) after a retried skip, got (%v,%v)", ok, conflict)
	}
	if skips < 2 {
		t.Fatalf("expected the skip to be retried, skips=%d", skips)
	}
}

func TestRebaseWithDerivedRegen_SkipThenNewDerivedConflict_Resolves(t *testing.T) {
	var diffN, skipN, contN int
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			diffN++
			switch diffN {
			case 1, 2: // pre-loop + first step: conflict on the doc
				return cflags + "\n", 0, nil
			case 3: // the resolution emptied the commit → triggers --skip
				return "", 0, nil
			default: // the skip landed on a NEW commit that also conflicts on the doc
				return cflags + "\n", 0, nil
			}
		case strings.HasPrefix(j, "add -- "):
			return "", 0, nil
		case strings.Contains(j, "rebase --continue"):
			contN++
			if contN == 1 {
				return "nothing to commit", 1, nil // commit emptied by resolution
			}
			return "", 0, nil // resolves the new conflict → done
		case strings.Contains(j, "rebase --skip"):
			skipN++
			return "transient", 1, nil // skip fails → loop re-checks, finds a new conflict
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, got := recordingRegen("")
	ok, conflict := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, isDerivedArtifact)
	if !ok || conflict {
		t.Fatalf("want (true,false) for skip-fail then a new derived resolve, got (%v,%v)", ok, conflict)
	}
	if skipN < 1 {
		t.Fatal("expected at least one failed --skip")
	}
	if len(*got) != 2 {
		t.Fatalf("expected two regenerations (first commit + post-skip commit), got %d", len(*got))
	}
}

func TestRegenerateDerivedArtifact_RunFails_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("invokes the go toolchain")
	}
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	// worktree/go has no module → `go run ./cmd/evolve` fails → error surfaced.
	if err := regenerateDerivedArtifact(context.Background(), wt, cflags); err == nil {
		t.Fatal("want an error when `go run` fails in a worktree with no module")
	}
}

func TestIsDerivedArtifact(t *testing.T) {
	if !isDerivedArtifact(cflags) {
		t.Fatalf("%s must be classified as a derived artifact", cflags)
	}
	for _, p := range []string{
		"go/internal/flagregistry/registry_table.go",
		"go/internal/core/orchestrator.go",
		"README.md",
		"",
	} {
		if isDerivedArtifact(p) {
			t.Fatalf("%q must NOT be a derived artifact", p)
		}
	}
}

// TestDerivedArtifacts_MapIntegrity locks the single classifier: every registered
// derived artifact must exist on disk and carry a GENERATED marker pair (so the
// path is really a projection, not a hand-written file), its regen command must be
// non-empty, and its ssotPrefix must be a real on-disk source path.
func TestDerivedArtifacts_MapIntegrity(t *testing.T) {
	root := repoRootForTest(t)
	if len(derivedArtifacts) == 0 {
		t.Fatal("derivedArtifacts must register at least control-flags.md")
	}
	for rel, spec := range derivedArtifacts {
		if len(spec.regenArgs) == 0 {
			t.Fatalf("%s has an empty regen command", rel)
		}
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("registered derived artifact %s not readable: %v", rel, err)
		}
		if !strings.Contains(string(b), "<!-- GENERATED:") {
			t.Fatalf("%s has no GENERATED marker — not a projection; do not register it", rel)
		}
		if spec.ssotPrefix == "" {
			t.Fatalf("%s has an empty ssotPrefix", rel)
		}
		if _, err := os.Stat(filepath.Join(root, spec.ssotPrefix)); err != nil {
			t.Fatalf("%s ssotPrefix %q does not exist on disk: %v", rel, spec.ssotPrefix, err)
		}
	}
}

func TestDerivedProjectionsForChanges(t *testing.T) {
	// A change under the registry SSOT prefix marks control-flags.md stale.
	got := derivedProjectionsForChanges([]string{
		"go/internal/flagregistry/registry_table.go",
		"go/internal/core/foo.go",
	})
	if len(got) != 1 || got[0] != cflags {
		t.Fatalf("registry change must flag %s stale, got %v", cflags, got)
	}
	// No SSOT change → no projection to regenerate (so non-flag cycles pay no cost).
	if g := derivedProjectionsForChanges([]string{"go/internal/core/foo.go", "README.md"}); len(g) != 0 {
		t.Fatalf("non-SSOT changes must trigger no regen, got %v", g)
	}
	if g := derivedProjectionsForChanges(nil); len(g) != 0 {
		t.Fatalf("no changes → no regen, got %v", g)
	}
}

func TestRegenStaleProjections_RegeneratesAndStages(t *testing.T) {
	var regened, staged []string
	regen := func(_ context.Context, _, rel string) error { regened = append(regened, rel); return nil }
	stage := func(_ context.Context, _, rel string) error { staged = append(staged, rel); return nil }
	done := regenStaleProjections(context.Background(), "/wt",
		[]string{"go/internal/flagregistry/registry_table.go", "go/internal/core/foo.go"}, regen, stage)
	if len(done) != 1 || done[0] != cflags {
		t.Fatalf("done = %v, want [%s]", done, cflags)
	}
	if len(regened) != 1 || regened[0] != cflags {
		t.Fatalf("regened = %v", regened)
	}
	if len(staged) != 1 || staged[0] != cflags {
		t.Fatalf("staged = %v", staged)
	}
}

func TestRegenStaleProjections_NoSSOTChange_NoOp(t *testing.T) {
	called := false
	regen := func(_ context.Context, _, _ string) error { called = true; return nil }
	done := regenStaleProjections(context.Background(), "/wt",
		[]string{"go/internal/core/foo.go", "README.md"}, regen, func(context.Context, string, string) error { return nil })
	if len(done) != 0 || called {
		t.Fatalf("no SSOT change must be a no-op (done=%v called=%v)", done, called)
	}
}

func TestRegenStaleProjections_RegenFails_SkipsWithoutStaging(t *testing.T) {
	staged := false
	regen := func(_ context.Context, _, _ string) error { return errors.New("regen boom") }
	done := regenStaleProjections(context.Background(), "/wt",
		[]string{"go/internal/flagregistry/registry_table.go"}, regen, func(context.Context, string, string) error { staged = true; return nil })
	if len(done) != 0 {
		t.Fatalf("regen failure → not done, got %v", done)
	}
	if staged {
		t.Fatal("must NOT stage when regen failed")
	}
}

func TestRegenStaleProjections_StageFails_Skips(t *testing.T) {
	regen := func(_ context.Context, _, _ string) error { return nil }
	done := regenStaleProjections(context.Background(), "/wt",
		[]string{"go/internal/flagregistry/registry_table.go"}, regen, func(context.Context, string, string) error { return errors.New("add boom") })
	if len(done) != 0 {
		t.Fatalf("stage failure → not done, got %v", done)
	}
}

// initTempRepo creates an empty temp git repo on branch main with a test identity
// and returns its dir plus a git runner that fails the test on any non-zero git.
func initTempRepo(t *testing.T) (string, func(...string)) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if _, code, err := gitCapture(context.Background(), dir, args...); err != nil || code != 0 {
			t.Fatalf("git %v: code=%d err=%v", args, code, err)
		}
	}
	git("init")
	git("checkout", "-b", "main")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "test")
	return dir, git
}

func TestStageWorktreePath_RealGit(t *testing.T) {
	dir, _ := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := stageWorktreePath(context.Background(), dir, "f.txt"); err != nil {
		t.Fatalf("stageWorktreePath: %v", err)
	}
	out, _, _ := gitCapture(context.Background(), dir, "diff", "--cached", "--name-only")
	if strings.TrimSpace(out) != "f.txt" {
		t.Fatalf("f.txt should be staged, got %q", out)
	}
}

func TestStageWorktreePath_NonRepo_Errors(t *testing.T) {
	// A non-git dir makes `git add` exit non-zero → stageWorktreePath returns the
	// command-failure error (the code!=0 branch).
	if err := stageWorktreePath(context.Background(), t.TempDir(), "nope.txt"); err == nil {
		t.Fatal("staging in a non-repo must return an error")
	}
}

func TestNormalizeDerivedProjections_EmptyWorktree_NoOp(t *testing.T) {
	o := &Orchestrator{}
	o.normalizeDerivedProjections(context.Background(), "") // guard: must not panic
}

func TestNormalizeDerivedProjections_CleanWorktree_NoRegen(t *testing.T) {
	// A clean repo (no changed paths) → no stale projection → no `go run`, no error.
	dir, git := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "base")
	o := &Orchestrator{}
	o.normalizeDerivedProjections(context.Background(), dir) // clean tree → no-op, no panic
}

func TestChangedWorktreePaths_RealGit(t *testing.T) {
	dir, git := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "base")
	// A tracked edit AND an untracked new file must both be reported (the untracked
	// case is the H1-class gap: a cycle ADDING a registry file must still regen).
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := changedWorktreePaths(context.Background(), dir)
	want := map[string]bool{"f.txt": false, "new.txt": false}
	for _, p := range got {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	if !want["f.txt"] || !want["new.txt"] {
		t.Fatalf("changedWorktreePaths must report tracked AND untracked changes, got %v", got)
	}
	// A clean tree yields no paths.
	git("checkout", "--", "f.txt")
	_ = os.Remove(filepath.Join(dir, "new.txt"))
	if g := changedWorktreePaths(context.Background(), dir); len(g) != 0 {
		t.Fatalf("clean tree must yield no changed paths, got %v", g)
	}
}

// repoRootForTest walks up from the test's working directory (go/internal/core)
// to the repo root (the dir containing go/ and docs/).
func repoRootForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go", "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root from %s", wd)
	return ""
}
