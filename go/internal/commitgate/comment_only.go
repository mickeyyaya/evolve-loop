package commitgate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// reviewWaiver returns why the change needs no reviewer, or why it still does.
// Only a comment removal qualifies: at least one Go file proven comment-only
// against HEAD, and nothing else but Markdown under docs/. It lists the whole
// change itself with renames split, so neither a --files selection nor a
// rename can hide a path from it. See docs/conventions/code-comments.md.
func (o Options) reviewWaiver(ctx context.Context) (waived, refused string) {
	out, _, code, err := sysexec.Capture(ctx, o.Runner, o.RepoRoot, "git", "diff", "--name-only", "--no-renames", "HEAD")
	if err != nil || code > 1 {
		return "", "the change could not be listed"
	}
	proven := 0
	for _, line := range strings.Split(out, "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		if reason := o.notWaivable(ctx, path); reason != "" {
			return "", path + ": " + reason
		}
		if strings.HasSuffix(path, ".go") {
			proven++
		}
	}
	if proven == 0 {
		return "", "no Go file proven comment-only"
	}
	return fmt.Sprintf("%d Go file(s) proven comment-only by commentaudit; every other file is docs/**.md", proven), ""
}

func (o Options) notWaivable(ctx context.Context, path string) string {
	switch {
	case !o.isRegularFile(path):
		return "not a regular file"
	case strings.Contains("/"+path, "/testdata/"):
		return "testdata fixtures are read by tests"
	case strings.HasSuffix(path, ".go"):
		return o.notCommentOnly(ctx, path)
	case strings.HasPrefix(path, "docs/") && strings.HasSuffix(path, ".md"):
		return ""
	}
	return "neither Go nor Markdown under docs/"
}

// isRegularFile refuses symlinks and deletions: a symlink would let a path that
// looks like docs or Go point at content the proof never saw.
func (o Options) isRegularFile(path string) bool {
	info, err := os.Lstat(filepath.Join(o.RepoRoot, path))
	return err == nil && info.Mode().IsRegular()
}

func (o Options) notCommentOnly(ctx context.Context, path string) string {
	before, _, code, err := sysexec.Capture(ctx, o.Runner, o.RepoRoot, "git", "show", "HEAD:"+path)
	if err != nil || code != 0 {
		return "not at HEAD"
	}
	after, err := os.ReadFile(filepath.Join(o.RepoRoot, path))
	if err != nil {
		return err.Error()
	}
	ok, reason, err := commentaudit.Equivalent(path, []byte(before), after)
	switch {
	case err != nil:
		return err.Error()
	case !ok:
		return reason
	}
	return ""
}
