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

func (o Options) reviewWaiver(ctx context.Context) (waived, refused string) {
	paths, err := o.changeListing(ctx)
	if err != nil {
		return "", "the change could not be listed"
	}
	proven := 0
	for _, path := range paths {
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
