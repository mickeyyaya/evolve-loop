package inboxcurator

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
	logFormat = "--format=" + recordSep + "%H" + fieldSep + "%cI" + fieldSep + "%B" + fieldSep
)

var revisionRE = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

var ErrBadRevision = errors.New("inboxcurator: a revision must be a hex commit id")

func IsCommitID(rev string) bool { return revisionRE.MatchString(rev) }

func Landed(ctx context.Context, git gitexec.Git, from, to string) ([]Change, error) {
	if !revisionRE.MatchString(from) {
		return nil, fmt.Errorf("%w: %q", ErrBadRevision, from)
	}
	return readLog(ctx, git, from+".."+to)
}

func Commit(ctx context.Context, git gitexec.Git, sha string) ([]Change, error) {
	if !revisionRE.MatchString(sha) {
		return nil, fmt.Errorf("%w: %q", ErrBadRevision, sha)
	}
	return readLog(ctx, git, "-1", sha)
}

func pathLines(block string) []string {
	var paths []string
	for _, line := range strings.Split(block, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func readLog(ctx context.Context, git gitexec.Git, rev ...string) ([]Change, error) {
	out, err := git.Output(ctx, append([]string{"-c", "core.quotepath=false", "log", "--no-color", "--first-parent", "-m", "--name-only", logFormat}, rev...)...)
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: read the landed commits %s: %w", strings.Join(rev, " "), err)
	}
	var changes []Change
	for _, record := range strings.Split(out, recordSep)[1:] {
		parts := strings.SplitN(record, fieldSep, 4)
		when, _ := time.Parse(time.RFC3339, parts[1])
		c := Change{Commit: parts[0], When: when, Text: strings.TrimSpace(parts[2]), Paths: pathLines(parts[3])}
		if c.Tests, err = definedTests(ctx, git, c); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, nil
}

func definedTests(ctx context.Context, git gitexec.Git, c Change) ([]string, error) {
	testFiles := slices.DeleteFunc(slices.Clone(c.Paths), func(p string) bool { return !strings.HasSuffix(p, "_test.go") })
	if len(testFiles) == 0 {
		return nil, nil
	}
	out, _, code, err := git.Capture(ctx, append([]string{"grep", "-h", "-E", "^func (Test|Benchmark|Fuzz)", c.Commit, "--"}, testFiles...)...)
	if err != nil || code > 1 {
		return nil, fmt.Errorf("inboxcurator: read the tests of commit %s: exit %d: %w", c.Commit, code, err)
	}
	var names []string
	for _, m := range testFuncRE.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names, nil
}
