package gc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type DevAction string

const (
	DevActionArchive DevAction = "archive"
	DevActionKeep    DevAction = "keep"
)

const devArchiveLabel = "gc: archived idle dev worktree"

var ErrPRLookupUnavailable = errors.New("gc: pull request lookup unavailable")

type DevItem struct {
	Path     string    `json:"path"`
	Branch   string    `json:"branch,omitempty"`
	Head     string    `json:"head,omitempty"`
	Action   DevAction `json:"action"`
	IdleDays float64   `json:"idle_days"`
	Dirty    bool      `json:"dirty,omitempty"`
	Failed   bool      `json:"failed,omitempty"`
	Reason   string    `json:"reason"`
}

type DevManifest struct {
	Items   []DevItem `json:"items"`
	Notes   []string  `json:"notes,omitempty"`
	Skipped string    `json:"skipped,omitempty"`
}

type OpenPRLookup func(ctx context.Context, dir string) (map[string][]time.Time, error)

type DevSaveFunc func(ctx context.Context, w wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error)

type DevOptions struct {
	ProjectRoot string
	EvolveDir   string
	Cwd         string
	Policy      DevWorktreesPolicy
	Now         func() time.Time
	Exec        sysexec.RunFunc
	Processes   func(ctx context.Context) ([]Process, error)
	OpenPRs     OpenPRLookup
	Save        DevSaveFunc
}

func (o DevOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o DevOptions) processes(ctx context.Context) ([]Process, error) {
	if o.Processes != nil {
		return o.Processes(ctx)
	}
	return listCwdProcesses(ctx, o.Exec, o.ProjectRoot)
}

func (o DevOptions) save(ctx context.Context, w wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
	if o.Save != nil {
		return o.Save(ctx, w)
	}
	return wtcheckpoint.Save(ctx, w, wtcheckpoint.SaveOptions{Label: devArchiveLabel, Now: o.Now})
}

type devSweep struct {
	o         DevOptions
	hub       wtcheckpoint.Hub
	store     gitexec.Git
	procs     []Process
	now       time.Time
	limit     time.Duration
	notes     []string
	prs       map[string][]time.Time
	prsErr    error
	prsLoaded bool
	strictPR  bool
}

func newDevSweep(ctx context.Context, o DevOptions) (*devSweep, string, error) {
	if o.Exec == nil {
		return nil, "", errors.New("gc: the dev worktree sweep requires Exec")
	}
	hub, err := wtcheckpoint.ResolveHub(o.ProjectRoot)
	if err != nil {
		return nil, err.Error(), nil
	}
	procs, err := o.processes(ctx)
	if err != nil {
		return nil, "", err
	}
	return &devSweep{o: o, hub: hub, store: gitexec.Git{Dir: hub.Store, Exec: o.Exec}, procs: procs, now: o.now(), limit: o.Policy.ArchiveAfter()}, "", nil
}

func PlanDevWorktrees(ctx context.Context, o DevOptions) (DevManifest, error) {
	s, skipped, err := newDevSweep(ctx, o)
	if err != nil || skipped != "" {
		return DevManifest{Skipped: skipped}, err
	}
	trees, err := s.hub.DevWorktrees(ctx)
	if err != nil {
		return DevManifest{}, err
	}
	items := make([]DevItem, 0, len(trees))
	for _, wt := range trees {
		items = append(items, s.judge(ctx, wt))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return DevManifest{Items: items, Notes: s.notes}, nil
}

type devGate func(ctx context.Context, it *DevItem) (string, error)

func (s *devSweep) judge(ctx context.Context, wt wtcheckpoint.Worktree) DevItem {
	it := DevItem{Path: wt.Dir, Action: DevActionKeep}
	tree := gitexec.Git{Dir: wt.Dir, Exec: s.o.Exec}
	gates := []devGate{
		func(ctx context.Context, it *DevItem) (string, error) { return s.measure(ctx, tree, it) },
		s.gateProtected,
		s.gateOccupied,
		s.gateBranch,
		s.gateIdle,
		s.gateOpenPR,
		s.gateLeftovers,
	}
	for _, gate := range gates {
		reason, err := gate(ctx, &it)
		if err != nil {
			it.Failed, it.Reason = true, "cannot judge: "+err.Error()
			return it
		}
		if reason != "" {
			it.Reason = reason
			return it
		}
	}
	return s.finish(ctx, tree, it)
}

func (s *devSweep) measure(ctx context.Context, tree gitexec.Git, it *DevItem) (string, error) {
	head, err := tree.HEAD(ctx)
	if err != nil {
		return "", err
	}
	it.Head = head
	branch, stderr, code, err := tree.Capture(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || code > 1 {
		return "", fmt.Errorf("git symbolic-ref exit %d err=%v: %s", code, err, strings.TrimSpace(stderr))
	}
	if code == 0 {
		it.Branch = strings.TrimSpace(branch)
	}
	last, err := lastActivity(ctx, tree)
	if err != nil {
		return "", err
	}
	it.IdleDays = s.now.Sub(last).Hours() / 24
	return "", nil
}

func (s *devSweep) gateProtected(_ context.Context, it *DevItem) (string, error) {
	switch {
	case samePathResolved(it.Path, s.o.ProjectRoot):
		return "the project root of this run", nil
	case isPlaneWorktree(s.hub.Root, it.Path):
		return "a plane worktree (runtime or console)", nil
	}
	return "", nil
}

func (s *devSweep) gateOccupied(_ context.Context, it *DevItem) (string, error) {
	if s.o.Cwd != "" && pathWithin(it.Path, s.o.Cwd) {
		return "the caller works in this directory", nil
	}
	for _, p := range s.procs {
		if pathWithin(it.Path, p.Cwd) {
			return fmt.Sprintf("process %d runs inside (cwd %s)", p.Pid, p.Cwd), nil
		}
	}
	return "", nil
}

func (s *devSweep) gateBranch(_ context.Context, it *DevItem) (string, error) {
	if it.Branch == "" {
		return "detached HEAD; no branch keeps its commits", nil
	}
	return "", nil
}

func (s *devSweep) gateIdle(_ context.Context, it *DevItem) (string, error) {
	if time.Duration(it.IdleDays*24*float64(time.Hour)) < s.limit {
		return fmt.Sprintf("active within the %d-day limit", int(s.limit.Hours()/24)), nil
	}
	return "", nil
}

func (s *devSweep) gateOpenPR(ctx context.Context, it *DevItem) (string, error) {
	prs, err := s.openPRs(ctx)
	if err != nil {
		return "", err
	}
	for _, updated := range prs[it.Branch] {
		if age := s.now.Sub(updated); age < s.limit {
			return fmt.Sprintf("open pull request updated %.1f days ago", age.Hours()/24), nil
		}
	}
	return "", nil
}

func (s *devSweep) openPRs(ctx context.Context) (map[string][]time.Time, error) {
	if s.prsLoaded {
		return s.prs, s.prsErr
	}
	s.prsLoaded = true
	if s.o.OpenPRs == nil && s.strictPR {
		s.prsErr = errors.New("no pull request lookup is configured, and an archive must not skip the open pull request rule")
		return nil, s.prsErr
	}
	if s.o.OpenPRs == nil {
		s.notes = append(s.notes, "no pull request lookup is configured: the open pull request check was skipped")
		return nil, nil
	}
	prs, err := s.o.OpenPRs(ctx, s.o.ProjectRoot)
	if errors.Is(err, ErrPRLookupUnavailable) && s.strictPR {
		s.prsErr = fmt.Errorf("gh is unavailable, and an archive must not skip the open pull request rule: %w", err)
		return nil, s.prsErr
	}
	if errors.Is(err, ErrPRLookupUnavailable) {
		s.notes = append(s.notes, "gh is unavailable: the open pull request check was skipped")
		return nil, nil
	}
	s.prs, s.prsErr = prs, err
	return s.prs, s.prsErr
}

func (s *devSweep) finish(ctx context.Context, tree gitexec.Git, it DevItem) DevItem {
	dirty, err := tree.DirtyPaths(ctx)
	if err == nil {
		it.Dirty = len(dirty) > 0
	}
	landed := ""
	if err == nil {
		landed, err = s.landed(ctx, it)
	}
	if err != nil {
		it.Failed, it.Reason = true, "cannot judge: "+err.Error()
		return it
	}
	it.Action = DevActionArchive
	it.Reason = archiveReason(it, landed, s.limit)
	return it
}

func archiveReason(it DevItem, landed string, limit time.Duration) string {
	reason := fmt.Sprintf("idle %.1f days (limit %d); branch %s %s and stays", it.IdleDays, int(limit.Hours()/24), it.Branch, landed)
	if it.Dirty {
		reason += "; uncommitted work is saved as a checkpoint first"
	}
	return reason
}

func (s *devSweep) landed(ctx context.Context, it DevItem) (string, error) {
	_, stderr, code, err := s.store.Capture(ctx, "merge-base", "--is-ancestor", it.Head, plane.OriginMainRef)
	switch {
	case err != nil || code > 1:
		return "", fmt.Errorf("git merge-base --is-ancestor exit %d err=%v: %s", code, err, strings.TrimSpace(stderr))
	case code == 0:
		return "is in main", nil
	}
	res, err := branchLandedBySquash(ctx, s.store, plane.OriginMainRef, it.Branch)
	switch {
	case err != nil:
		return "", err
	case res.NoNet:
		return fmt.Sprintf("has no net changes against main (%d commit(s) not in main)", res.Ahead), nil
	case res.Landed:
		return "is in main", nil
	}
	return "is not in main", nil
}

func samePathResolved(a, b string) bool {
	return resolvePath(a) == resolvePath(b)
}

func isPlaneWorktree(hubRoot, dir string) bool {
	for _, name := range []string{"runtime", "console"} {
		if samePathResolved(dir, filepath.Join(hubRoot, name)) {
			return true
		}
	}
	return false
}

var devEvalSymlinks = filepath.EvalSymlinks

func resolveDeep(p string) string {
	p = filepath.Clean(p)
	for tail := ""; ; {
		if r, err := devEvalSymlinks(p); err == nil {
			return filepath.Join(r, tail)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(p, tail)
		}
		tail, p = filepath.Join(filepath.Base(p), tail), parent
	}
}

func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(resolveDeep(root), resolveDeep(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
