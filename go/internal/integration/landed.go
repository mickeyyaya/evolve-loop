package integration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
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
	if !objectID.MatchString(sha) || strings.Trim(sha, "0") == "" {
		return fmt.Errorf("integration: %q is not a full object id", sha)
	}
	return nil
}

type Repo struct {
	Dir string
	Git GitFunc
}

var errNoPatch = errors.New("empty diff")

func (r Repo) PatchID(ctx context.Context, sha string) (string, error) {
	if err := fullObjectID(sha); err != nil {
		return "", err
	}
	diff, err := r.Git(ctx, r.Dir, "", "diff-tree", "-p", "-U1", "--root", "--end-of-options", sha)
	if err != nil {
		return "", fmt.Errorf("integration: read the diff of %s: %w", sha, err)
	}
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("integration: commit %s has no patch: %w", sha, errNoPatch)
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
	landed, err := r.CheckLanded(ctx, rec, ref)
	return landed && err == nil
}

func (r Repo) CheckLanded(ctx context.Context, rec DeliverableRecord, ref string) (bool, error) {
	if len(rec.Commits) == 0 {
		return false, nil
	}
	if own, err := r.patchesAreOwn(ctx, rec); err != nil || !own {
		return false, err
	}
	entries, err := r.ownedEntries(ctx, rec, ref)
	if err != nil {
		return false, err
	}
	if rec.Console() {
		return r.consoleLanded(ctx, rec, entries)
	}
	return r.laneLanded(ctx, rec, entries)
}

type logEntry struct {
	sha     string
	message string
}

func (r Repo) ownedEntries(ctx context.Context, rec DeliverableRecord, ref string) ([]logEntry, error) {
	log, err := r.Git(ctx, r.Dir, "", "log", "-z", "--format=%H%n%B", "--end-of-options", rec.Commits[0].BaseSHA+".."+ref)
	if err != nil {
		return nil, fmt.Errorf("integration: list the commits that landed on %s: %w", ref, err)
	}
	var owned []logEntry
	for _, entry := range strings.Split(log, logEntrySep) {
		sha, message, ok := strings.Cut(strings.TrimLeft(entry, "\n"), "\n")
		if ok && rec.OwnsMessage(message) {
			owned = append(owned, logEntry{sha: sha, message: message})
		}
	}
	return owned, nil
}

func (r Repo) laneLanded(ctx context.Context, rec DeliverableRecord, entries []logEntry) (bool, error) {
	var candidates []LandedCommit
	for _, e := range entries {
		patch, err := r.PatchID(ctx, e.sha)
		if err != nil {
			return false, err
		}
		candidates = append(candidates, LandedCommit{SHA: e.sha, Message: e.message, PatchID: patch})
	}
	return FindLanded(candidates, rec), nil
}

func (r Repo) consoleLanded(ctx context.Context, rec DeliverableRecord, entries []logEntry) (bool, error) {
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		id, err := r.PatchID(ctx, e.sha)
		if errors.Is(err, errNoPatch) {
			continue
		}
		if err != nil {
			return false, err
		}
		present[id] = true
	}
	for _, c := range rec.Commits {
		id, err := r.PatchID(ctx, c.SHA)
		if err != nil || !present[id] {
			return false, err
		}
	}
	return true, nil
}

func (r Repo) patchesAreOwn(ctx context.Context, rec DeliverableRecord) (bool, error) {
	for _, c := range rec.Commits {
		patch, err := r.PatchID(ctx, c.SHA)
		if err != nil {
			return false, err
		}
		if patch != c.PatchID {
			return false, nil
		}
	}
	return true, nil
}

func (r Repo) IsAncestor(ctx context.Context, sha, ref string) (bool, error) {
	if err := fullObjectID(sha); err != nil {
		return false, err
	}
	_, err := r.Git(ctx, r.Dir, "", "merge-base", "--is-ancestor", "--end-of-options", sha, ref)
	if err == nil {
		return true, nil
	}
	var ge *GitError
	if errors.As(err, &ge) && ge.Code == 1 && ge.Err == nil {
		return false, nil
	}
	return false, fmt.Errorf("integration: is %s an ancestor of %s: %w", sha, ref, err)
}

func FindRecordByCommit(root, sha string) (DeliverableRecord, bool) {
	return firstRecord(root, "*.json", func(r DeliverableRecord) bool { return r.HoldsCommit(sha) })
}

func RecordForCommit(root string, cycle int, sha string) (DeliverableRecord, bool) {
	return firstRecord(root, "*.json", func(r DeliverableRecord) bool { return r.Cycle == cycle && r.HoldsCommit(sha) })
}

func firstRecord(root, name string, match func(DeliverableRecord) bool) (DeliverableRecord, bool) {
	paths, err := filepath.Glob(filepath.Join(WaveDir(root, "*"), recordPrefix+name))
	if err != nil {
		return DeliverableRecord{}, false
	}
	slices.SortStableFunc(paths, compareWaveDirs)
	for _, p := range paths {
		if r, err := ReadRecord(p); err == nil && match(r) {
			return r, true
		}
	}
	return DeliverableRecord{}, false
}

func compareWaveDirs(a, b string) int {
	an, aok := poolNumber(a)
	bn, bok := poolNumber(b)
	switch {
	case aok && bok && an != bn:
		return cmp.Compare(an, bn)
	case aok != bok:
		if aok {
			return -1
		}
		return 1
	}
	return cmp.Compare(a, b)
}

func poolNumber(recordPath string) (int, bool) {
	name := strings.TrimPrefix(filepath.Base(filepath.Dir(recordPath)), wavePrefix)
	n, err := strconv.Atoi(name)
	return n, err == nil && n >= 0
}
