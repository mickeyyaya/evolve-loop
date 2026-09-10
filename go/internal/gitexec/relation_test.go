package gitexec

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func relGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func relCommit(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	relGit(t, dir, "add", name)
	relGit(t, dir, "commit", "-q", "-m", "add "+name)
}

// relationFixture: a bare origin with one commit on main, a seed clone that
// can advance origin, and the checkout under test.
func relationFixture(t *testing.T) (seed, checkout string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	seed = t.TempDir()
	checkout = filepath.Join(t.TempDir(), "checkout")
	relGit(t, "", "init", "-q", "--bare", "-b", "main", origin)
	relGit(t, seed, "init", "-q", "-b", "main")
	relGit(t, seed, "config", "user.email", "t@example.com")
	relGit(t, seed, "config", "user.name", "t")
	relCommit(t, seed, "base.txt")
	relGit(t, seed, "remote", "add", "origin", origin)
	relGit(t, seed, "push", "-q", "origin", "main")
	relGit(t, "", "clone", "-q", origin, checkout)
	relGit(t, checkout, "config", "user.email", "t@example.com")
	relGit(t, checkout, "config", "user.name", "t")
	return seed, checkout
}

// TestRelationToRemote_FourKinds pins the single main-relation resolver the
// wave boundary and the lane base share (2026-09-09 token-waste root cause #3).
func TestRelationToRemote_FourKinds(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		local  bool
		remote bool
		want   RelationKind
		ahead  int
		behind int
	}{
		{"current", false, false, RelationCurrent, 0, 0},
		{"behind", false, true, RelationBehind, 0, 1},
		{"ahead", true, false, RelationAhead, 1, 0},
		{"diverged", true, true, RelationDiverged, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed, checkout := relationFixture(t)
			if tc.local {
				relCommit(t, checkout, "local.txt")
			}
			if tc.remote {
				relCommit(t, seed, "remote.txt")
				relGit(t, seed, "push", "-q", "origin", "main")
				relGit(t, checkout, "fetch", "-q", "origin")
			}
			rel, err := Git{Dir: checkout, Exec: sysexec.DefaultRunner}.RelationToRemote(ctx, "origin/main")
			if err != nil {
				t.Fatal(err)
			}
			if rel.Kind != tc.want || rel.Ahead != tc.ahead || rel.Behind != tc.behind {
				t.Fatalf("got %+v, want kind=%s ahead=%d behind=%d", rel, tc.want, tc.ahead, tc.behind)
			}
			if s := rel.String(); !strings.Contains(strings.ToLower(s), string(tc.want)) {
				t.Fatalf("String() must name the kind: %q", s)
			}
		})
	}
}

// TestRelationToRemote_UnknownRefIsAnError: no guessing — an unresolvable
// remote ref is reported, never classified.
func TestRelationToRemote_UnknownRefIsAnError(t *testing.T) {
	_, checkout := relationFixture(t)
	if _, err := (Git{Dir: checkout, Exec: sysexec.DefaultRunner}).RelationToRemote(context.Background(), "origin/nope"); err == nil {
		t.Fatal("an unknown remote ref must be an error")
	}
}

// TestMainRelation_ZeroValueString: the rendered sentence never panics or
// lies on an unresolved relation.
func TestMainRelation_ZeroValueString(t *testing.T) {
	var rel MainRelation
	if got := rel.String(); !strings.Contains(got, "unknown") {
		t.Fatalf("zero-value MainRelation must render as unknown; got %q", got)
	}
}

// TestRelationToRemote_UnparseableCountsAreAnError: "nothing here guesses a
// state" — a rev-list count that is not two integers is an error, never a
// silent zero that could reclassify AHEAD as BEHIND.
func TestRelationToRemote_UnparseableCountsAreAnError(t *testing.T) {
	fake := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		switch {
		case len(args) >= 2 && args[0] == "rev-parse" && args[1] == "HEAD":
			_, _ = io.WriteString(stdout, "aaaa\n")
		case len(args) >= 1 && args[0] == "rev-parse":
			_, _ = io.WriteString(stdout, "bbbb\n")
		case len(args) >= 1 && args[0] == "rev-list":
			_, _ = io.WriteString(stdout, "x y\n")
		}
		return 0, nil
	}
	if _, err := (Git{Dir: t.TempDir(), Exec: fake}).RelationToRemote(context.Background(), "origin/main"); err == nil {
		t.Fatal("non-integer rev-list counts must be an error, not a guessed relation")
	}
}
