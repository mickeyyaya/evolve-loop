package publishmirror

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/gitexec"
	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

// DefaultRemote is the public open-source mirror. Pushes always go here by URL,
// never to the private repo's origin remote.
const DefaultRemote = "https://github.com/mickeyyaya/evolveloop.git"

// orphanBranch is the throwaway local branch the mirror snapshot is built on. It
// has no parent (severs all private history) and is deleted after each run.
const orphanBranch = "mirror-publish"

// Options configures a publish-mirror run. The zero value is unusable; at minimum
// RepoDir should point at the private repo (defaults to the current directory).
type Options struct {
	RepoDir      string          // private repo root (default: cwd)
	Ref          string          // release commit/ref to snapshot (default: HEAD)
	Remote       string          // public mirror URL (default: DefaultRemote)
	Tag          string          // optional tag to create on the mirror
	Message      string          // commit message (default: "Release <tag-or-short-sha>")
	PublicReadme string          // optional path to a README.md to swap in (defers the B1c decision)
	Denylist     []string        // PII substrings that must not appear (e.g. operator username, email)
	AllowFiles   []string        // staged paths exempt from sanitizer violations (known-safe test/example fixtures); exemptions are logged, never silent
	Push         bool            // false = dry-run (build + sanitize, never push)
	ScratchDir   string          // worktree dir (default: sibling "evolveloop-mirror-scratch")
	Exec         sysexec.RunFunc // git execution seam (default: sysexec.DefaultRunner)
	Stderr       io.Writer       // progress log sink (default: discard)
}

// Result reports what a run did (or, in dry-run, would do).
type Result struct {
	ReleaseCommit string      // resolved 40-char commit the snapshot was taken from
	ScratchDir    string      // worktree the snapshot was built in (removed on success)
	StagedFiles   int         // number of files staged into the mirror tree
	Dropped       []string    // index paths removed (the tracked binary)
	Violations    []Violation // REAL sanitizer findings after allowlist filtering (non-empty ⇒ hard stop, no push)
	Exempted      int         // count of violations suppressed by AllowFiles (logged, for transparency)
	Pushed        bool        // whether the mirror was actually pushed
	PublicRef     string      // the pushed branch ref on the mirror ("main")
	Tag           string      // the tag created/pushed, if any
	Message       string      // the commit message used
}

// Run builds the public mirror snapshot from the private tree and, when
// opts.Push is set and the sanitizer is clean, pushes it to the mirror by URL.
// Without opts.Push it is a dry-run: it builds and sanitizes the snapshot, then
// tears it down, reporting what would be published. A non-empty Result.Violations
// is always a hard stop (returned as an error) — no push happens.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Exec == nil {
		opts.Exec = sysexec.DefaultRunner
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.RepoDir == "" {
		opts.RepoDir = "."
	}
	if opts.Ref == "" {
		opts.Ref = "HEAD"
	}
	if opts.Remote == "" {
		opts.Remote = DefaultRemote
	}
	absRepo, err := filepath.Abs(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve repo dir: %w", err)
	}
	if opts.ScratchDir == "" {
		opts.ScratchDir = filepath.Join(filepath.Dir(absRepo), "evolveloop-mirror-scratch")
	}
	// The scratch worktree inherits the private repo's remotes (including
	// origin). A bare remote NAME would resolve against that config and could
	// push the snapshot to the private repo. Require an explicit URL or path
	// (anything with a scheme, host, or path separator) so the push target is
	// unambiguous and can never be origin.
	if !strings.ContainsAny(opts.Remote, "/:@") {
		return nil, fmt.Errorf("--remote %q looks like a bare remote name; pass a URL or path so the push never resolves against the private repo's remotes", opts.Remote)
	}

	repo := gitexec.Git{Dir: absRepo, Exec: opts.Exec}
	logf := func(format string, a ...any) { fmt.Fprintf(opts.Stderr, "[publish-mirror] "+format+"\n", a...) }

	commit, err := repo.Output(ctx, "rev-parse", "--verify", opts.Ref+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("resolve ref %q: %w", opts.Ref, err)
	}
	commit = strings.TrimSpace(commit)
	res := &Result{ReleaseCommit: commit, ScratchDir: opts.ScratchDir, Message: opts.Message}
	logf("release commit %s", commit)

	// Fresh scratch worktree off the release commit.
	cleanupWorktree(ctx, repo, opts.ScratchDir, logf)
	if err := repo.Run(ctx, "worktree", "add", "--detach", opts.ScratchDir, commit); err != nil {
		return res, fmt.Errorf("worktree add: %w", err)
	}
	defer cleanupWorktree(ctx, repo, opts.ScratchDir, logf)

	scratch := gitexec.Git{Dir: opts.ScratchDir, Exec: opts.Exec}

	if err := applyTransforms(opts); err != nil {
		return res, err
	}

	// Orphan branch (no parent) + stage everything + drop the tracked binary.
	if err := scratch.Run(ctx, "checkout", "--orphan", orphanBranch); err != nil {
		return res, fmt.Errorf("checkout --orphan: %w", err)
	}
	if err := scratch.Run(ctx, "add", "-A"); err != nil {
		return res, fmt.Errorf("git add -A: %w", err)
	}
	// Drop the tracked binary from the index. git exits 128 when the path is not
	// in the index (publishing from a ref before the binary was tracked) — that
	// is benign. Any OTHER non-zero code is unexpected (e.g. a locked/corrupt
	// index) and must not be silently swallowed, or the binary could ship.
	if _, _, code, err := scratch.Capture(ctx, "rm", "--cached", trackedBinary); err != nil {
		return res, fmt.Errorf("rm --cached %s: %w", trackedBinary, err)
	} else {
		switch code {
		case 0:
			res.Dropped = append(res.Dropped, trackedBinary)
		case 128:
			// not tracked in this snapshot — nothing to drop
		default:
			return res, fmt.Errorf("rm --cached %s: unexpected git exit %d", trackedBinary, code)
		}
	}

	staged, err := scratch.Output(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return res, fmt.Errorf("list staged: %w", err)
	}
	stagedPaths := splitLines(staged)
	res.StagedFiles = len(stagedPaths)
	logf("staged %d files; dropped %v", res.StagedFiles, res.Dropped)

	files, skipped, err := readStagedFiles(opts.ScratchDir, stagedPaths)
	if err != nil {
		return res, err
	}
	if len(skipped) > 0 {
		logf("not text-scanned (binary): %d file(s): %v", len(skipped), skipped)
	}
	real, exempted := partitionViolations(Scan(files, opts.Denylist), opts.AllowFiles)
	res.Violations = real
	res.Exempted = exempted
	if exempted > 0 {
		logf("allowlist: exempted %d violation(s) (allowlist: %d entries)", exempted, len(opts.AllowFiles))
	}
	if len(res.Violations) > 0 {
		logf("SANITIZER FAIL: %d violation(s)", len(res.Violations))
		return res, fmt.Errorf("sanitizer found %d violation(s) — refusing to publish", len(res.Violations))
	}
	logf("sanitizer clean")

	if !opts.Push {
		logf("dry-run: would publish %d files to %s (run with --push)", res.StagedFiles, opts.Remote)
		return res, nil
	}

	msg := opts.Message
	if msg == "" {
		if opts.Tag != "" {
			msg = "Release " + opts.Tag
		} else {
			msg = "Release " + short(commit)
		}
	}
	res.Message = msg
	if err := scratch.Run(ctx, "commit", "-m", msg); err != nil {
		return res, fmt.Errorf("commit: %w", err)
	}
	if opts.Tag != "" {
		if err := scratch.Run(ctx, "tag", opts.Tag); err != nil {
			return res, fmt.Errorf("tag %s: %w", opts.Tag, err)
		}
	}
	if err := scratch.Run(ctx, "push", opts.Remote, orphanBranch+":main"); err != nil {
		return res, fmt.Errorf("push to mirror: %w", err)
	}
	res.Pushed = true
	res.PublicRef = "main"
	if opts.Tag != "" {
		if err := scratch.Run(ctx, "push", opts.Remote, opts.Tag); err != nil {
			return res, fmt.Errorf("push tag %s: %w", opts.Tag, err)
		}
		res.Tag = opts.Tag
	}
	logf("pushed %s -> %s main%s", short(commit), opts.Remote, tagSuffix(opts.Tag))
	return res, nil
}

// applyTransforms mutates the scratch tree in place: removes the chore(build)
// commit-prefix entry and (when configured) swaps in the public README.
func applyTransforms(opts Options) error {
	scopePath := filepath.Join(opts.ScratchDir, commitPrefixScopePath)
	if b, err := os.ReadFile(scopePath); err == nil {
		out, rerr := removeBuildPrefix(string(b))
		if rerr != nil {
			return rerr
		}
		if err := os.WriteFile(scopePath, []byte(out), 0o644); err != nil {
			return fmt.Errorf("write commit-prefix-scope: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read commit-prefix-scope: %w", err)
	}

	if opts.PublicReadme != "" {
		b, err := os.ReadFile(opts.PublicReadme)
		if err != nil {
			return fmt.Errorf("read public README %q: %w", opts.PublicReadme, err)
		}
		if err := os.WriteFile(filepath.Join(opts.ScratchDir, "README.md"), b, 0o644); err != nil {
			return fmt.Errorf("swap README: %w", err)
		}
	}
	return nil
}

// cleanupWorktree tears down the scratch worktree and the throwaway orphan
// branch. Order matters: remove the worktree (which releases the checked-out
// orphan branch) BEFORE deleting the directory, so git can drop its admin entry
// cleanly; only then force-delete the branch (which otherwise lingers in the
// private repo holding a full-tree snapshot). worktree-remove failures are
// logged; a missing orphan branch (the dry-run case, never committed) is normal
// and stays quiet.
func cleanupWorktree(ctx context.Context, repo gitexec.Git, dir string, logf func(string, ...any)) {
	if _, _, code, err := repo.Capture(ctx, "worktree", "remove", "--force", dir); err != nil || (code != 0 && code != 128) {
		logf("cleanup: worktree remove %q exit=%d err=%v", dir, code, err)
	}
	_ = os.RemoveAll(dir)
	_, _, _, _ = repo.Capture(ctx, "branch", "-D", orphanBranch)
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func tagSuffix(tag string) string {
	if tag == "" {
		return ""
	}
	return " + " + tag
}
