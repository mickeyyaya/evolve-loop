package integration

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
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
		if err != nil || code != 0 {
			return "", &GitError{Args: args, Code: code, Stdout: out.String(), Stderr: errOut.String(), Err: err}
		}
		return out.String(), nil
	}
}

type GitError struct {
	Args   []string
	Code   int
	Stdout string
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	return fmt.Sprintf("integration: git %s exit=%d: %s %v", strings.Join(e.Args, " "), e.Code, strings.TrimSpace(e.Stderr), e.Err)
}

func fullObjectID(sha string) error {
	if !objectID.MatchString(sha) {
		return fmt.Errorf("integration: %q is not a full object id", sha)
	}
	return nil
}

type Repo struct {
	Dir string
	Git GitFunc
}

func (r Repo) PatchID(ctx context.Context, sha string) (string, error) {
	if err := fullObjectID(sha); err != nil {
		return "", err
	}
	diff, err := r.Git(ctx, r.Dir, "", "diff-tree", "-p", "-U1", "--root", "--end-of-options", sha)
	if err != nil {
		return "", fmt.Errorf("integration: read the diff of %s: %w", sha, err)
	}
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("integration: commit %s has no patch", sha)
	}
	out, err := r.Git(ctx, r.Dir, diff, "patch-id", "--verbatim")
	if err != nil {
		return "", fmt.Errorf("integration: patch-id of %s: %w", sha, err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("integration: patch-id of %s printed nothing", sha)
	}
	return fields[0], nil
}

func (r Repo) LandedOnRef(ctx context.Context, rec DeliverableRecord, ref string) bool {
	if len(rec.Commits) == 0 || !r.patchesAreOwn(ctx, rec) {
		return false
	}
	log, err := r.Git(ctx, r.Dir, "", "log", "-z", "--format=%H%n%B", "--end-of-options", rec.Commits[0].BaseSHA+".."+ref)
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
	return firstRecord(root, strconv.Itoa(cycle)+".json", func(r DeliverableRecord) bool { return r.Cycle == cycle })
}

func FindRecordByCommit(root, sha string) (DeliverableRecord, bool) {
	return firstRecord(root, "*.json", func(r DeliverableRecord) bool { return r.HoldsCommit(sha) })
}

func firstRecord(root, name string, match func(DeliverableRecord) bool) (DeliverableRecord, bool) {
	paths, err := filepath.Glob(filepath.Join(WaveDir(root, "*"), recordPrefix+name))
	if err != nil {
		return DeliverableRecord{}, false
	}
	for _, p := range paths {
		if r, err := ReadRecord(p); err == nil && match(r) {
			return r, true
		}
	}
	return DeliverableRecord{}, false
}
