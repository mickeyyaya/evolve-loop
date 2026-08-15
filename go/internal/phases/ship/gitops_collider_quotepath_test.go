// gitops_collider_quotepath_test.go — RED contract for cycle-1469 top_n task
// `gitstage-collider-quotepath` (the collider-side hole left by cycle-1108).
//
// Cycle-1108 taught every path-classifying git reader in this package the
// quote-path contract — `-c core.quotePath=false` on the read (rawPathRead,
// gitops.go) plus unquoteGitPath (manifest.go) for the residue that flag does
// not suppress. detectColliders (gitops.go:77) was never enrolled. It reads
// BOTH `git diff --name-only` and `git status --porcelain` with a bare
// captureGitOutputAtDir, and its only decoding is a naive strip of the wrapping
// quotes:
//
//	if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
//	    path = path[1 : len(path)-1]
//	}
//
// Verified against real git 2.50.1 (2026-08-15, `git status --porcelain` on a
// tree holding all three input classes):
//
//	?? "caf\303\251.txt"       # non-ASCII → octal-escaped, quoted
//	?? "we\"ird.txt"           # embedded quote → backslash-escaped, quoted
//	?? "we -> ird.txt"         # space → quoted, NOT escaped
//
// So the strip yields the 15-byte literal `caf\303\251.txt` and the 11-byte
// literal `we\"ird.txt` — paths that exist on no disk. detectColliders then
// os.Stat()s them, gets ENOENT, and `continue`s: the real untracked main-side
// collider is INVISIBLE to both the pre-flight and the repair ladder, and the
// ff-merge hits the file git refuses to overwrite. The `git diff --name-only`
// stream is worse — it gets no decoding at all, not even the strip.
//
// Contract pinned here:
//  1. A quoted porcelain entry (non-ASCII, embedded quote, backslash) is
//     compared as its LITERAL repo-relative path, so a real collider is found.
//  2. A quoted `git diff --name-only` entry decodes the same way.
//  3. Negative/anti-no-op: a path that is not a collider — absent on the main
//     side, or present but TRACKED there — is never reported, and a deleted
//     porcelain entry stays skipped.
//  4. Both classification reads carry `-c core.quotePath=false` BEFORE the
//     subcommand, the zero-parsing half of the same contract every other reader
//     in this package already honours.
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// colliderStub is a CmdRunner standing in for the three git reads
// detectColliders issues: `diff --name-only`, `status --porcelain`, and the
// per-candidate `ls-files` tracking probe. Canned output is emitted verbatim —
// quoted exactly as git writes it — so the parser is exercised against real
// git output shape rather than a pre-normalised convenience value.
type colliderStub struct {
	diff    string   // `git diff --name-only` stdout
	status  string   // `git status --porcelain` stdout
	tracked []string // literal paths `git ls-files <p>` reports as tracked in main
	calls   [][]string
}

func (c *colliderStub) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, stdout, _ io.Writer) (int, error) {
		c.calls = append(c.calls, append([]string{name}, args...))
		if name != "git" {
			return 0, nil
		}
		write := func(s string) (int, error) {
			if stdout != nil && s != "" {
				_, _ = io.WriteString(stdout, s)
			}
			return 0, nil
		}
		switch {
		case slices.Contains(args, "diff"):
			return write(c.diff)
		case slices.Contains(args, "status"):
			return write(c.status)
		case slices.Contains(args, "ls-files"):
			// git ls-files echoes the pathspec only when it is tracked.
			p := args[len(args)-1]
			if slices.Contains(c.tracked, p) {
				return write(p + "\n")
			}
			return 0, nil
		}
		return 0, nil
	}
}

// gitCallWith returns the first recorded git argv containing sub, or nil.
func (c *colliderStub) gitCallWith(sub string) []string {
	for _, call := range c.calls {
		if len(call) > 0 && call[0] == "git" && slices.Contains(call[1:], sub) {
			return call
		}
	}
	return nil
}

// colliderTrees materialises the two sides detectColliders os.Stat()s: the
// cycle worktree and the main project root. Every name is written to BOTH
// unless listed in mainOnlyAbsent, because a collider is by definition a file
// that exists on both sides and is untracked on the main side.
func colliderTrees(t *testing.T, names []string, mainOnlyAbsent ...string) (root, worktree string) {
	t.Helper()
	root, worktree = filepath.Join(t.TempDir(), "main"), filepath.Join(t.TempDir(), "wt")
	for _, dir := range []string{root, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(worktree, n), []byte("incoming\n"), 0o644); err != nil {
			t.Fatalf("write worktree %q: %v", n, err)
		}
		if slices.Contains(mainOnlyAbsent, n) {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, n), []byte("main side\n"), 0o644); err != nil {
			t.Fatalf("write main %q: %v", n, err)
		}
	}
	return root, worktree
}

// TestDetectColliders_QuotePathDecodesPorcelainEntries — AC1. Each quoted
// porcelain class must be reported under its LITERAL name. Today the naive
// quote-strip hands back the escaped text, os.Stat fails, and the collider is
// dropped on the floor.
func TestDetectColliders_QuotePathDecodesPorcelainEntries(t *testing.T) {
	tests := []struct {
		name   string
		status string // exactly as git writes it
		want   string // the literal on-disk path
	}{
		{
			name:   "octal-escaped non-ascii",
			status: `?? "caf\303\251.txt"` + "\n",
			want:   "café.txt",
		},
		{
			name:   "embedded quote",
			status: `?? "we\"ird.txt"` + "\n",
			want:   `we"ird.txt`,
		},
		{
			name:   "embedded backslash",
			status: `?? "back\\slash.txt"` + "\n",
			want:   `back\slash.txt`,
		},
		{
			name:   "quoted-but-unescaped space",
			status: `?? "with space.txt"` + "\n",
			want:   "with space.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, wt := colliderTrees(t, []string{tc.want})
			stub := &colliderStub{status: tc.status}
			opts := &Options{ProjectRoot: root, Runner: stub.runner(), Stderr: io.Discard}

			got, err := detectColliders(context.Background(), opts, wt, "main", "cycle-1469")
			if err != nil {
				t.Fatalf("detectColliders: %v", err)
			}
			if !slices.Equal(got, []string{tc.want}) {
				t.Errorf("detectColliders(status=%q) = %q, want [%q] — a C-quoted incoming filename must compare as its literal repo-relative path, or the ff-merge collides with a file the pre-flight never saw", tc.status, got, tc.want)
			}
		})
	}
}

// TestDetectColliders_QuotePathDecodesDiffNameOnly — AC2. The
// `git diff --name-only` stream gets no decoding at all today, not even the
// naive strip, so a quote-bearing file changed in branch..cycleBranch is
// invisible even when it collides.
func TestDetectColliders_QuotePathDecodesDiffNameOnly(t *testing.T) {
	const want = "café.txt"
	root, wt := colliderTrees(t, []string{want})
	stub := &colliderStub{diff: `"caf\303\251.txt"` + "\n"}
	opts := &Options{ProjectRoot: root, Runner: stub.runner(), Stderr: io.Discard}

	got, err := detectColliders(context.Background(), opts, wt, "main", "cycle-1469")
	if err != nil {
		t.Fatalf("detectColliders: %v", err)
	}
	if !slices.Equal(got, []string{want}) {
		t.Errorf("detectColliders(diff=quoted) = %q, want [%q] — the diff --name-only reader must decode C-quoted paths exactly as the porcelain reader does", got, want)
	}
}

// TestDetectColliders_QuotePathNonCollidersStaySilent — AC3, the anti-no-op
// half. Decoding must not turn every quoted incoming path into a collider: a
// quoted path absent on the main side, a quoted path TRACKED on the main side,
// and a quoted DELETION are all non-colliders and must stay out of the list.
// A change that reports these would quarantine operator files that a ff-merge
// would have accepted.
func TestDetectColliders_QuotePathNonCollidersStaySilent(t *testing.T) {
	const (
		real     = "café.txt"     // untracked on main → the one true collider
		absent   = "absent é.txt" // worktree-only → not a collider
		trackedP = "tracked é.txt"
		deleted  = "gone é.txt"
	)
	root, wt := colliderTrees(t, []string{real, absent, trackedP, deleted}, absent)
	stub := &colliderStub{
		status: `?? "caf\303\251.txt"` + "\n" +
			`?? "absent \303\251.txt"` + "\n" +
			`?? "tracked \303\251.txt"` + "\n" +
			` D "gone \303\251.txt"` + "\n",
		tracked: []string{trackedP},
	}
	opts := &Options{ProjectRoot: root, Runner: stub.runner(), Stderr: io.Discard}

	got, err := detectColliders(context.Background(), opts, wt, "main", "cycle-1469")
	if err != nil {
		t.Fatalf("detectColliders: %v", err)
	}
	if !slices.Equal(got, []string{real}) {
		t.Errorf("detectColliders = %q, want exactly [%q] — decoding must not widen the collider set: %q is worktree-only, %q is tracked on main, %q is a deletion", got, real, absent, trackedP, deleted)
	}
}

// TestDetectColliders_QuotePathDisabledOnGitReads — AC4. Both classification
// reads must carry `-c core.quotePath=false` before the subcommand, so the
// common non-ASCII case never reaches the parser escaped at all (git only
// accepts config args before the subcommand). This is the same argv contract
// TestStageExplicitPaths_QuotePathDisabledOnGitReads pins for the staging
// reads; detectColliders was simply never enrolled.
func TestDetectColliders_QuotePathDisabledOnGitReads(t *testing.T) {
	root, wt := colliderTrees(t, []string{"plain.txt"})
	stub := &colliderStub{status: "?? plain.txt\n", diff: "plain.txt\n"}
	opts := &Options{ProjectRoot: root, Runner: stub.runner(), Stderr: io.Discard}

	if _, err := detectColliders(context.Background(), opts, wt, "main", "cycle-1469"); err != nil {
		t.Fatalf("detectColliders: %v", err)
	}
	for _, sub := range []string{"diff", "status"} {
		call := stub.gitCallWith(sub)
		if call == nil {
			t.Fatalf("detectColliders never invoked `git %s`; calls=%v", sub, stub.calls)
		}
		args := call[1:]
		cfgAt := configArgIndex(args, "core.quotePath=false")
		if cfgAt < 0 {
			t.Errorf("`git %s` argv %v lacks `-c core.quotePath=false` — the collider readers must honour the cycle-1108 quote-path contract every other path reader in this package already does", sub, args)
			continue
		}
		if subAt := slices.Index(args, sub); subAt >= 0 && cfgAt > subAt {
			t.Errorf("`git %s` argv %v places `-c core.quotePath=false` (index %d) after the subcommand (index %d) — git only accepts config args before the subcommand", sub, args, cfgAt, subAt)
		}
	}
	// Guard the decode half stays wired too: quotePath=false does NOT suppress
	// quoting for embedded quotes/backslashes/spaces (verified against git
	// 2.50.1), so the flag alone is not the whole fix.
	if !strings.Contains(strings.Join(stub.gitCallWith("status"), " "), "porcelain") {
		t.Errorf("status read is no longer `--porcelain`: %v", stub.gitCallWith("status"))
	}
}
