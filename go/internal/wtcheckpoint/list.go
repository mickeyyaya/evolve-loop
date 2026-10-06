package wtcheckpoint

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type Entry struct {
	Ref      string    `json:"ref"`
	Worktree string    `json:"worktree"`
	Time     time.Time `json:"time"`
	Label    string    `json:"label,omitempty"`
	Base     string    `json:"base"`
	Files    int       `json:"files"`
	Added    int       `json:"added"`
	Deleted  int       `json:"deleted"`
}

func List(ctx context.Context, h Hub, worktree string) ([]Entry, error) {
	pattern := refPrefix
	if worktree != "" {
		pattern = refPrefix + worktreeAt(filepath.Clean(worktree)).Name + "/"
	}
	refs, err := listRefs(ctx, h.git(), pattern)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(refs))
	for _, r := range refs {
		e, err := describe(ctx, h.git(), r)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func describe(ctx context.Context, g gitexec.Git, r refEntry) (Entry, error) {
	at, err := time.Parse(stampLayout, r.stamp())
	if err != nil {
		return Entry{}, fmt.Errorf("wtcheckpoint: %s does not end in a %s stamp: %w", r.name, stampLayout, err)
	}
	e := Entry{Ref: r.name, Worktree: r.worktree(), Time: at}
	if e.Base, err = g.Output(ctx, "rev-parse", "--verify", r.oid+"~2"); err != nil {
		return Entry{}, fmt.Errorf("wtcheckpoint: %s has no base commit: %w", r.name, err)
	}
	body, err := g.Output(ctx, "show", "-s", "--format=%b", r.oid)
	if err != nil {
		return Entry{}, err
	}
	e.Label = labelOf(body)
	numstat, err := g.Output(ctx, "diff", "--numstat", "--no-renames", e.Base, r.oid)
	if err != nil {
		return Entry{}, err
	}
	e.Files, e.Added, e.Deleted, err = sumNumstat(numstat)
	return e, err
}

func labelOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if label, ok := strings.CutPrefix(strings.TrimSpace(line), labelPrefix); ok {
			return label
		}
	}
	return ""
}

func sumNumstat(numstat string) (files, added, deleted int, err error) {
	for _, line := range strings.Split(numstat, "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			continue
		}
		files++
		if f[0] == binaryNumstat {
			continue
		}
		a, aErr := strconv.Atoi(f[0])
		d, dErr := strconv.Atoi(f[1])
		if aErr != nil || dErr != nil {
			return 0, 0, 0, fmt.Errorf("wtcheckpoint: unreadable numstat line %q", line)
		}
		added, deleted = added+a, deleted+d
	}
	return files, added, deleted, nil
}
