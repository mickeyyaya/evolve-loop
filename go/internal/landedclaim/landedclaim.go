// Package landedclaim proves that a commit delivered an inbox item, and fails closed.
// See docs/architecture/packages/internal-landedclaim.md.
package landedclaim

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type Check func(sha string, paths []string) bool

var commitID = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func OnMain(git gitexec.Git) Check {
	return func(sha string, paths []string) bool {
		if !commitID.MatchString(sha) || len(paths) == 0 {
			return false
		}
		ctx := context.Background()
		if _, _, code, err := git.Capture(ctx, "merge-base", "--is-ancestor", sha, "main"); err != nil || code != 0 {
			return false
		}
		out, err := git.Output(ctx, "-c", "core.quotepath=false", "log", "-1", "--no-color", "--first-parent", "-m", "--name-only", "--format=", sha)
		return err == nil && touchesAny(strings.Split(out, "\n"), paths)
	}
}

func touchesAny(changed, declared []string) bool {
	for _, c := range changed {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		c = path.Clean(c)
		for _, d := range declared {
			if d = path.Clean(d); c == d || strings.HasPrefix(c, d+"/") {
				return true
			}
		}
	}
	return false
}
