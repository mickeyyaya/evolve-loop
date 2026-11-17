package inboxcurator

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
	logFormat = "--format=" + recordSep + "%H" + fieldSep + "%P" + fieldSep + "%cI" + fieldSep + "%B" + fieldSep
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

func ClosedHistory(ctx context.Context, git gitexec.Git, since time.Time) ([]Change, error) {
	out, err := git.Output(ctx, "log", "--no-color", "--since="+since.UTC().Format(time.RFC3339), "--grep=Closes-Inbox:", "--format=%H", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: find the commits that close an item: %w", err)
	}
	shas := strings.Fields(out)
	if len(shas) == 0 {
		return nil, nil
	}
	return readChanges(ctx, git, closesLogArgs, append([]string{"--no-walk=unsorted"}, shas...)...)
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
	return readChanges(ctx, git, fullLogArgs, rev...)
}

var (
	fullLogArgs   = []string{"-c", "core.quotepath=false", "log", "--no-color", "--first-parent", "-m", "--name-only", logFormat}
	closesLogArgs = []string{"log", "--no-color", "--first-parent", logFormat}
)

func readChanges(ctx context.Context, git gitexec.Git, args []string, rev ...string) ([]Change, error) {
	out, err := git.Output(ctx, slices.Concat(args, rev)...)
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: read the landed commits %s: %w", strings.Join(rev, " "), err)
	}
	var changes []Change
	for _, record := range strings.Split(out, recordSep)[1:] {
		parts := strings.SplitN(record, fieldSep, 5)
		when, _ := time.Parse(time.RFC3339, parts[2])
		c := Change{Commit: parts[0], When: when, Text: strings.TrimSpace(parts[3]), Paths: pathLines(parts[4])}
		if c.Closes, err = closesOf(ctx, git, c, strings.Fields(parts[1])); err != nil {
			return nil, err
		}
		if c.Tests, err = definedTests(ctx, git, c); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, nil
}

func closesOf(ctx context.Context, git gitexec.Git, c Change, parents []string) ([]string, error) {
	if len(parents) < 2 {
		return closesinbox.IDs([]byte(c.Text)), nil
	}
	out, err := git.Output(ctx, "rev-list", "--no-merges", "--format=%B", parents[0]+".."+c.Commit)
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: read the merged branch of commit %s: %w", c.Commit, err)
	}
	return closesinbox.IDs([]byte(out)), nil
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
