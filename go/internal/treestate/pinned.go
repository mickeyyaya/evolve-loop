package treestate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const hunkMarker = "@@"

var pinnedPrefix = []string{
	"-c", "core.quotePath=false",
	"-c", "diff.suppressBlankEmpty=false",
	"-c", "core.bigFileThreshold=512m",
	"diff",
	"--full-index",
	"--no-ext-diff",
	"--no-textconv",
	"--no-color",
	"--no-renames",
	"--no-relative",
	"--src-prefix=a/",
	"--dst-prefix=b/",
	"--diff-algorithm=myers",
	"--indent-heuristic",
	"-U3",
	"--inter-hunk-context=0",
	"--ignore-submodules=dirty",
	"--submodule=short",
	"-O/dev/null",
	"--end-of-options",
}

func DiffArgs(revs ...string) []string {
	args := make([]string, 0, len(pinnedPrefix)+len(revs))
	args = append(args, pinnedPrefix...)
	return append(args, revs...)
}

func Pinned(ctx context.Context, run sysexec.RunFunc, dir string, env []string) (string, error) {
	return pinnedSHA(ctx, run, dir, env, "HEAD")
}

func PinnedBetween(ctx context.Context, run sysexec.RunFunc, dir string, env []string, from, to string) (string, error) {
	return pinnedSHA(ctx, run, dir, env, from, to)
}

func pinnedSHA(ctx context.Context, run sysexec.RunFunc, dir string, env []string, revs ...string) (string, error) {
	args := DiffArgs(revs...)
	text, err := runDiff(ctx, run, dir, scrubbedEnv(env), args, RunError{Args: args})
	if err != nil {
		return "", err
	}
	return digest(stripHunkContext(text)), nil
}

func scrubbedEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "GIT_DIFF_OPTS=") {
			kept = append(kept, kv)
		}
	}
	return kept
}

func stripHunkContext(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, hunkMarker+" ") {
			if end := strings.Index(line[len(hunkMarker)+1:], " "+hunkMarker); end >= 0 {
				lines[i] = line[:len(hunkMarker)+1+end+1+len(hunkMarker)]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
