package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/gitexec"
	"github.com/mickeyyaya/evolveloop/go/internal/publishmirror"
	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

// runPublishMirrorCmd implements `evolve publish-mirror` — build (and optionally
// push) the public open-source mirror from the private tree, applying the
// residual publish transform and a deterministic PII/secret sanitizer gate. It
// automates docs/operations/public-release.md.
//
// Usage:
//
//	evolve publish-mirror                       # dry-run: build + sanitize, no push
//	evolve publish-mirror --push --tag v1.2.3   # publish a tagged release to the mirror
//	evolve publish-mirror --public-readme PATH  # swap in a condensed public README (B1c)
//
// Flags:
//
//	--ref REF            release commit/ref to snapshot (default: HEAD)
//	--remote URL         public mirror URL (default: the evolveloop repo)
//	--tag TAG            tag to create on the mirror
//	--message MSG        commit message (default: "Release <tag-or-short-sha>")
//	--public-readme PATH README.md to swap in for the public tree
//	--repo DIR           private repo root (default: cwd)
//	--push               actually push (without it, dry-run)
//
// Exit codes:
//
//	0  — dry-run clean, or pushed
//	1  — runtime failure (bad args, git failure)
//	2  — sanitizer found PII/secret violations (no push)
func runPublishMirrorCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve publish-mirror", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts publishmirror.Options
	fs.StringVar(&opts.Ref, "ref", "HEAD", "release commit/ref to snapshot")
	fs.StringVar(&opts.Remote, "remote", publishmirror.DefaultRemote, "public mirror URL")
	fs.StringVar(&opts.Tag, "tag", "", "tag to create on the mirror")
	fs.StringVar(&opts.Message, "message", "", `commit message (default: "Release <tag-or-short-sha>")`)
	fs.StringVar(&opts.PublicReadme, "public-readme", "", "path to a README.md to swap into the public tree")
	fs.StringVar(&opts.RepoDir, "repo", "", "private repo root (default: cwd)")
	fs.StringVar(&opts.ScratchDir, "scratch-dir", "", "worktree dir (default: sibling evolveloop-mirror-scratch)")
	var allowFrom string
	fs.StringVar(&allowFrom, "allow-from", "docs/operations/mirror-sanitizer-allowlist.txt", "file listing staged paths exempt from the sanitizer (one per line; # comments)")
	fs.BoolVar(&opts.Push, "push", false, "actually push to the mirror (without it, dry-run)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	opts.Exec = sysexec.DefaultRunner
	opts.Stderr = stderr
	opts.Denylist = operatorDenylist(opts.RepoDir)
	opts.AllowFiles = loadAllowlist(opts.RepoDir, allowFrom, stderr)

	res, err := publishmirror.Run(context.Background(), opts)
	if res != nil && len(res.Violations) > 0 {
		fmt.Fprintf(stderr, "publish-mirror: SANITIZER FAILED — %d violation(s):\n", len(res.Violations))
		for _, v := range res.Violations {
			fmt.Fprintf(stderr, "  %s:%d  [%s]  %s\n", v.File, v.Line, v.Rule, v.Match)
		}
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "publish-mirror: %v\n", err)
		return 1
	}
	if res.Pushed {
		tagPart := ""
		if res.Tag != "" {
			tagPart = ", tag " + res.Tag
		}
		fmt.Fprintf(stdout, "publish-mirror: pushed %d files to %s (ref %s%s) from %s\n",
			res.StagedFiles, opts.Remote, res.PublicRef, tagPart, res.ReleaseCommit)
	} else {
		fmt.Fprintf(stdout, "publish-mirror: DRY-RUN ok — %d files, sanitizer clean, dropped %v. Re-run with --push to publish to %s.\n",
			res.StagedFiles, res.Dropped, opts.Remote)
	}
	return 0
}

// operatorDenylist auto-derives the PII terms most likely to leak: the login
// name of whoever runs the publish, and their configured git email/name. These
// are exactly the strings the convergence scrub replaced, so they double as a
// regression net. The login name comes from os/user (not os.Getenv("USER")) so
// it is not an operator-dial env read subject to the EVOLVE_ anti-rename gate.
func operatorDenylist(repoDir string) []string {
	var out []string
	if u, err := user.Current(); err == nil {
		if name := strings.TrimSpace(u.Username); name != "" {
			out = append(out, name)
		}
	}
	dir := repoDir
	if dir == "" {
		dir = "."
	}
	g := gitexec.Git{Dir: dir, Exec: sysexec.DefaultRunner}
	for _, key := range []string{"user.email", "user.name"} {
		if v, err := g.Output(context.Background(), "config", key); err == nil {
			if t := strings.TrimSpace(v); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// loadAllowlist reads the sanitizer allowlist (one staged path per line; blank
// lines and `#` comments ignored). A missing file is not an error — it just
// means no exemptions. The file is tracked (auditable) so the public mirror's
// exemptions are version-controlled.
func loadAllowlist(repoDir, path string, stderr io.Writer) []string {
	if path == "" {
		return nil
	}
	full := path
	if !filepath.IsAbs(path) {
		dir := repoDir
		if dir == "" {
			dir = "."
		}
		full = filepath.Join(dir, path)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "publish-mirror: warning: cannot read allowlist %s: %v\n", full, err)
		}
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	// Audit trail: which allowlist (and how many entries) governed this run.
	fmt.Fprintf(stderr, "publish-mirror: sanitizer allowlist: %d entries from %s\n", len(out), full)
	return out
}
