package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type GitFunc func(ctx context.Context, dir, stdin string, args ...string) (string, error)

const (
	gitBin      = "git"
	logEntrySep = "\x00"
)

var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func SysGit() GitFunc {
	return func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		var out, errOut strings.Builder
		var in io.Reader
		if stdin != "" {
			in = strings.NewReader(stdin)
		}
		code, err := sysexec.DefaultRunner(ctx, gitBin, dir, args, nil, in, &out, &errOut)
		if err != nil {
			return "", fmt.Errorf("integration: git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(errOut.String()), err)
		}
		if code != 0 {
			return "", fmt.Errorf("integration: git %s exit=%d: %s", strings.Join(args, " "), code, strings.TrimSpace(errOut.String()))
		}
		return out.String(), nil
	}
}

type Repo struct {
	Dir string
	Git GitFunc
}

func (r Repo) PatchID(ctx context.Context, sha string) (string, error) {
	if !objectID.MatchString(sha) {
		return "", fmt.Errorf("integration: %q is not a full object id", sha)
	}
	diff, err := r.Git(ctx, r.Dir, "", "diff-tree", "-p", "--root", "--end-of-options", sha)
	if err != nil {
		return "", fmt.Errorf("integration: read the diff of %s: %w", sha, err)
	}
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("%w: %s is a merge or an empty commit", ErrNoPatch, sha)
	}
	id, err := r.DiffPatchID(ctx, diff)
	if err != nil {
		return "", fmt.Errorf("integration: patch-id of %s: %w", sha, err)
	}
	return id, nil
}

func (r Repo) DiffPatchID(ctx context.Context, diff string) (string, error) {
	out, err := r.Git(ctx, r.Dir, diff, "patch-id", "--stable")
	if err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w", err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", errors.New("git patch-id --stable: empty output (empty diff?)")
	}
	return fields[0], nil
}

func (r Repo) LandedOnRef(ctx context.Context, rec DeliverableRecord, ref string) bool {
	return r.landedIn(ctx, rec, ref)
}

func (r Repo) LandedSince(ctx context.Context, rec DeliverableRecord, ref, since string) bool {
	if !objectID.MatchString(since) {
		return false
	}
	return r.landedIn(ctx, rec, ref, "^"+since)
}

func (r Repo) landedIn(ctx context.Context, rec DeliverableRecord, ref string, exclude ...string) bool {
	if len(rec.Commits) == 0 || !r.patchesAreOwn(ctx, rec) {
		return false
	}
	args := append([]string{"log", "-z", "--format=%H%n%B", "--end-of-options", rec.Commits[0].BaseSHA + ".." + ref}, exclude...)
	log, err := r.Git(ctx, r.Dir, "", args...)
	if err != nil {
		return false
	}
	var candidates []LandedCommit
	for _, entry := range strings.Split(log, logEntrySep) {
		sha, message, ok := strings.Cut(strings.TrimLeft(entry, "\n"), "\n")
		if !ok || !HasCycleTrailer(message, rec.Cycle) {
			continue
		}
		patch, err := r.PatchID(ctx, sha)
		if err != nil {
			continue
		}
		candidates = append(candidates, LandedCommit{SHA: sha, Message: message, PatchID: patch})
	}
	return FindLanded(candidates, rec)
}

func (r Repo) patchesAreOwn(ctx context.Context, rec DeliverableRecord) bool {
	for _, c := range rec.Commits {
		if patch, err := r.PatchID(ctx, c.SHA); err != nil || patch != c.PatchID {
			return false
		}
	}
	return true
}

func FindRecord(root string, cycle int) (DeliverableRecord, bool) {
	paths, err := filepath.Glob(RecordPath(filepath.Join(wavesDir(root), waveDirPrefix+"*"), cycle))
	if err != nil {
		return DeliverableRecord{}, false
	}
	return firstRecord(paths, func(r DeliverableRecord) bool { return r.Cycle == cycle })
}

func FindRecordByCommit(root, sha string) (DeliverableRecord, bool) {
	if !objectID.MatchString(sha) {
		return DeliverableRecord{}, false
	}
	entries, err := filepath.Glob(filepath.Join(wavesDir(root), waveDirPrefix+"*", commitIndexDir, sha+indexSep+"*"))
	if err != nil {
		return DeliverableRecord{}, false
	}
	var paths []string
	for _, e := range entries {
		if cycle, ok := indexedCycle(e, sha); ok {
			paths = append(paths, RecordPath(filepath.Dir(filepath.Dir(e)), cycle))
		}
	}
	return firstRecord(paths, func(r DeliverableRecord) bool { return r.HoldsCommit(sha) })
}

func firstRecord(paths []string, match func(DeliverableRecord) bool) (DeliverableRecord, bool) {
	for _, p := range paths {
		if r, err := ReadRecord(p); err == nil && match(r) {
			return r, true
		}
	}
	return DeliverableRecord{}, false
}
