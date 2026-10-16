package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

const (
	completionArtifact         = core.CompletionArtifact
	completionStdout           = core.CompletionStdout
	completionGit              = core.CompletionGit
	completionWorktreeEvidence = core.CompletionWorktreeEvidence
)

func completionContractName(mode core.CompletionContract) core.CompletionContract {
	if mode == "" {
		return completionArtifact
	}
	return mode
}

const stdoutIdlePolls = 3

const artifactStableTicks = 2

const finalPollGrace = 5 * time.Second

type finalPollCtxKey struct{}

func withFinalPoll(ctx context.Context) (context.Context, context.CancelFunc) {
	live := context.WithValue(context.WithoutCancel(ctx), finalPollCtxKey{}, true)
	return context.WithTimeout(live, finalPollGrace)
}

func isFinalPoll(ctx context.Context) bool {
	final, _ := ctx.Value(finalPollCtxKey{}).(bool)
	return final
}

type completionEvidence struct {
	CommitSHA          string
	CarriedDeliverable string
	WorktreeChanges    []string
}

type completionDetector interface {
	poll(ctx context.Context) (ready bool, evidence completionEvidence, note string, err error)
}

func newCompletionDetector(mode core.CompletionContract, cfg *Config, deps Deps, lp tmuxLaunch, base dispatchBaseline) completionDetector {
	switch mode {
	case completionStdout:
		return &stdoutDetector{cfg: cfg, deps: deps, lp: lp, threshold: stdoutIdlePolls}
	case completionGit:
		return newGitEvidenceDetector(cfg, deps)
	case completionWorktreeEvidence:
		return &worktreeEvidenceDetector{artifactDetector: &artifactDetector{cfg: cfg, baseline: base}, deps: deps}
	default:
		return &artifactDetector{cfg: cfg, baseline: base}
	}
}

type dispatchBaseline struct {
	entries  map[string]baselineEntry
	worktree worktreeSnapshot
}

type baselineEntry struct {
	size    int64
	modTime time.Time
}

func captureArtifactBaseline(cfg *Config) dispatchBaseline {
	var b dispatchBaseline
	for _, path := range artifactCandidatePaths(cfg) {
		fi, err := os.Lstat(path)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
			continue
		}
		if b.entries == nil {
			b.entries = map[string]baselineEntry{}
		}
		b.entries[path] = baselineEntry{size: fi.Size(), modTime: fi.ModTime()}
	}
	return b
}

func (b dispatchBaseline) matches(path string, fi os.FileInfo) bool {
	e, ok := b.entries[path]
	return ok && fi.Size() == e.size && fi.ModTime().Equal(e.modTime)
}

type gitEvidenceDetector struct {
	phase       string
	expectedTok string
	gitCmd      func(ctx context.Context, args ...string) (string, error)

	baseline     string
	haveBaseline bool
}

func newGitEvidenceDetector(cfg *Config, deps Deps) *gitEvidenceDetector {
	tok := ""
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "challenge-token.txt")); err == nil {
		tok = strings.TrimSpace(string(b))
	}
	if tok == "" && deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[git-evidence] WARN: challenge-token.txt missing/empty in %s — completion will never verify\n", cfg.Workspace)
	}
	worktree := cfg.Worktree
	return &gitEvidenceDetector{
		phase:       cfg.Agent,
		expectedTok: tok,
		gitCmd: func(ctx context.Context, args ...string) (string, error) {
			var out strings.Builder
			full := append([]string{"-C", worktree}, args...)
			_, err := deps.Runner(ctx, "git", "", full, driverEnv(deps, nil), nil, &out, io.Discard)
			return strings.TrimSpace(out.String()), err
		},
	}
}

func (d *gitEvidenceDetector) poll(ctx context.Context) (bool, completionEvidence, string, error) {
	head, err := d.gitCmd(ctx, "rev-parse", "HEAD")
	if err != nil || head == "" {
		return false, completionEvidence{}, "", nil
	}
	if !d.haveBaseline {
		d.baseline, d.haveBaseline = head, true
		return false, completionEvidence{}, "", nil
	}
	if head == d.baseline {
		return false, completionEvidence{}, "", nil
	}
	revList, err := d.gitCmd(ctx, "rev-list", d.baseline+"..HEAD")
	if err != nil {
		return false, completionEvidence{}, "", nil
	}
	for _, sha := range strings.Fields(revList) {
		msg, err := d.gitCmd(ctx, "log", "-1", "--format=%B", sha)
		if err != nil {
			continue
		}
		if evidence.Parse(msg).Verify(d.phase, d.expectedTok) {
			return true, completionEvidence{CommitSHA: sha},
				fmt.Sprintf("git-evidence: %s commit %s verified", d.phase, shortSHA(sha)), nil
		}
	}
	d.baseline = head
	return false, completionEvidence{}, "", nil
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

type artifactDetector struct {
	cfg      *Config
	baseline dispatchBaseline

	haveLast    bool
	lastPath    string
	lastSize    int64
	lastModTime time.Time
	stable      int
}

var statArtifact = os.Stat

func (d *artifactDetector) poll(ctx context.Context) (bool, completionEvidence, string, error) {
	path, found := artifactLocate(d.cfg)
	if !found {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	if isFinalPoll(ctx) || ctx.Err() != nil {
		if fi, serr := os.Stat(path); serr == nil && d.baseline.matches(path, fi) {
			return false, completionEvidence{}, "", nil
		}
		return d.completeWith(renameOnlyRelocate)
	}
	fi, serr := statArtifact(path)
	if serr != nil {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	if d.baseline.matches(path, fi) {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	if d.haveLast && path == d.lastPath && fi.Size() == d.lastSize && fi.ModTime().Equal(d.lastModTime) {
		d.stable++
	} else {
		d.stable = 1
	}
	d.haveLast, d.lastPath, d.lastSize, d.lastModTime = true, path, fi.Size(), fi.ModTime()

	if d.stable < artifactStableTicks {
		return false, completionEvidence{}, "", nil
	}
	if missing := d.missingSecondary(); missing != "" {
		return false, completionEvidence{}, "", nil
	}
	return d.completeWith(relocateFile)
}

func (d *artifactDetector) missingSecondary() string {
	for _, p := range d.cfg.SecondaryArtifacts {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			return p
		}
	}
	return ""
}

func (d *artifactDetector) completeWith(move func(src, dst string) error) (bool, completionEvidence, string, error) {
	ready, from, err := artifactCanonicalize(d.cfg, move)
	if err != nil {
		return false, completionEvidence{}, "", err
	}
	if !ready {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	return true, completionEvidence{}, d.completionNote(from), nil
}

func (d *artifactDetector) completionNote(relocatedFrom string) string {
	if relocatedFrom != "" {
		return fmt.Sprintf("artifact relocated from non-canonical %s → %s; appeared: %s", relocatedFrom, d.cfg.Artifact, d.cfg.Artifact)
	}
	return fmt.Sprintf("artifact appeared: %s", d.cfg.Artifact)
}

type stdoutDetector struct {
	cfg       *Config
	deps      Deps
	lp        tmuxLaunch
	threshold int

	haveBaseline bool
	baseline     string
	last         string
	stable       int
}

func (d *stdoutDetector) poll(ctx context.Context) (bool, completionEvidence, string, error) {
	pane, err := d.deps.Tmux.CapturePane(ctx, d.lp.session, d.lp.bootScrollback)
	if err != nil {
		return false, completionEvidence{}, "", nil
	}
	if !d.haveBaseline {
		d.baseline, d.last, d.haveBaseline = pane, pane, true
		return false, completionEvidence{}, "", nil
	}
	if pane == d.last {
		d.stable++
	} else {
		d.stable = 0
	}
	d.last = pane

	markerPresent := d.lp.promptMarker != "" && strings.Contains(pane, d.lp.promptMarker)
	if pane != d.baseline && markerPresent && d.stable >= d.threshold {
		return true, completionEvidence{}, fmt.Sprintf("stdout completion: REPL idle %d poll(s) with prompt marker", d.stable), nil
	}
	return false, completionEvidence{}, "", nil
}
