package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

const (
	completionArtifact = "artifact"
	completionStdout   = "stdout"
	completionGit      = "git"
)

// completionContractName names the contract a mode selects; "" is the artifact contract and any other
// mode is its own name.
func completionContractName(mode string) string {
	if mode == "" {
		return completionArtifact
	}
	return mode
}

// stdoutIdlePolls is how many consecutive unchanged poll ticks (each ~2s in the wait loop) with the REPL
// prompt marker visible count as "the turn finished" for the stdout contract: a streaming agent's pane
// changes every tick, so the counter only accrues once output has settled.
const stdoutIdlePolls = 3

// artifactStableTicks is the artifact twin of stdoutIdlePolls: how many consecutive poll ticks must observe
// the same (size, mtime) before the deliverable counts as finished. Two is the minimum that is actually a
// window (one observation is just a first-sight read), and each extra tick costs ~2s on every phase.
const artifactStableTicks = 2

// finalPollGrace bounds the wait loop's one post-cancel completion poll, which runs on a context detached
// from the cancelled one (see withFinalPoll) so detectors that shell a subprocess can actually run it;
// sized for a single tmux capture or `git rev-parse`, not for an agent turn.
const finalPollGrace = 5 * time.Second

// finalPollCtxKey marks a context as the wait loop's last look before it gives up: finality is signalled
// explicitly rather than inferred from ctx.Err(), since a dead context can't fork tmux or git for the
// stdout/git detectors.
type finalPollCtxKey struct{}

// withFinalPoll returns the context for the wait loop's final completion poll: detached from the caller's
// cancellation, bounded by finalPollGrace, and carrying the finality marker. The caller must call the
// returned cancel.
func withFinalPoll(ctx context.Context) (context.Context, context.CancelFunc) {
	live := context.WithValue(context.WithoutCancel(ctx), finalPollCtxKey{}, true)
	return context.WithTimeout(live, finalPollGrace)
}

// isFinalPoll reports whether this poll is the wait loop's last look.
func isFinalPoll(ctx context.Context) bool {
	final, _ := ctx.Value(finalPollCtxKey{}).(bool)
	return final
}

// completionEvidence carries what a detector observed at completion; empty for the artifact and stdout
// contracts, and carries the commit SHA for the git-evidence contract.
type completionEvidence struct {
	CommitSHA string
}

// completionDetector answers "is the phase done?" once per poll tick inside
// runTmuxREPL's wait loop. note is the human log line the loop emits on a
// terminal observation (ready, or a surfaced fault); err is a detector fault
// (e.g. a non-canonical artifact that could not be relocated) the loop logs
// once. A detector is single-use per launch and may hold cross-poll state.
type completionDetector interface {
	poll(ctx context.Context) (ready bool, evidence completionEvidence, note string, err error)
}

// newCompletionDetector builds the detector for the requested mode. Unknown /
// empty modes fall back to the artifact contract so a typo can never silently
// disable completion — it just keeps the legacy behavior.
func newCompletionDetector(mode string, cfg *Config, deps Deps, lp tmuxLaunch, base artifactBaseline) completionDetector {
	switch mode {
	case completionStdout:
		return &stdoutDetector{cfg: cfg, deps: deps, lp: lp, threshold: stdoutIdlePolls}
	case completionGit:
		return newGitEvidenceDetector(cfg, deps)
	default:
		return &artifactDetector{cfg: cfg, baseline: base}
	}
}

// artifactBaseline is the pre-dispatch snapshot of the artifact path: what was already on disk before this
// dispatch's prompt was delivered. An observation identical to the baseline is the prior attempt's work:
// it never begins a stability window and never completes, timeout being the honest outcome for an agent
// that wrote nothing.
type artifactBaseline struct {
	// entries maps each candidate path that existed pre-dispatch to its (size, mtime) snapshot — the whole
	// artifactCandidatePaths set, not just the canonical: a stray at a fallback, shadowed at capture time,
	// must not certify later when the canonical vanishes mid-session.
	entries map[string]baselineEntry
}

type baselineEntry struct {
	size    int64
	modTime time.Time
}

// captureArtifactBaseline snapshots the artifact path; must be called before prompt delivery, or an
// instant-writing agent's fresh artifact would be mistaken for the prior attempt's and refused. Any error
// degrades to an absent baseline (fail-open).
func captureArtifactBaseline(cfg *Config) artifactBaseline {
	var b artifactBaseline
	for _, path := range artifactCandidatePaths(cfg) {
		fi, err := os.Lstat(path)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
			continue // absent/irregular/empty: nothing to refuse later
		}
		if b.entries == nil {
			b.entries = map[string]baselineEntry{}
		}
		b.entries[path] = baselineEntry{size: fi.Size(), modTime: fi.ModTime()}
	}
	return b
}

// matches reports whether an observation is byte-for-byte a pre-dispatch artifact (same path, size, and
// mtime — the same key the stability window uses, so the two checks cannot drift).
func (b artifactBaseline) matches(path string, fi os.FileInfo) bool {
	e, ok := b.entries[path]
	return ok && fi.Size() == e.size && fi.ModTime().Equal(e.modTime)
}

// gitEvidenceDetector implements the git-evidence contract: completion is a new commit carrying an
// Evolve-Phase trailer for this phase and the cycle's challenge token; an advance without a matching
// trailer re-baselines and keeps watching. gitCmd is a seam so the detector is unit-testable without a
// real repo.
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
		// Fail-closed: an empty token makes Verify always false, so the detector would wait forever;
		// surface it loudly rather than hang silently.
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
		// Worktree not ready or git error: keep waiting (reviewer bounds a hang).
		return false, completionEvidence{}, "", nil
	}
	if !d.haveBaseline {
		d.baseline, d.haveBaseline = head, true
		return false, completionEvidence{}, "", nil
	}
	if head == d.baseline {
		return false, completionEvidence{}, "", nil // HEAD not advanced yet
	}
	// HEAD advanced — scan every new commit in baseline..HEAD, not just the tip: two commits can land
	// between polls, and inspecting only HEAD would re-baseline past an earlier evidence commit and wait
	// forever. rev-list lists newest-first; any verifying commit in the range completes.
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
	// No verifying commit in the new range: re-baseline to HEAD and keep watching rather than false-completing.
	d.baseline = head
	return false, completionEvidence{}, "", nil
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// artifactDetector implements the artifact contract: completion is a non-empty file at cfg.Artifact that
// has stopped changing. artifactLocate answers "is it there?" without touching the file; a cross-poll
// stability window answers "is it finished?"; artifactReady then canonicalizes it.
type artifactDetector struct {
	cfg      *Config
	baseline artifactBaseline

	haveLast    bool
	lastPath    string
	lastSize    int64
	lastModTime time.Time
	stable      int
}

func (d *artifactDetector) poll(ctx context.Context) (bool, completionEvidence, string, error) {
	path, found := artifactLocate(d.cfg)
	if !found {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	// Final look: the wait loop's one last poll before giving up. The artifact is on disk, which is the
	// evidence, so this short-circuits the stability window rather than laundering a finished-at-the-buzzer
	// session into ExitArtifactTimeout.
	//
	// No window was ever closed here, so the mover is renameOnlyRelocate, not relocateFile: completing on
	// an unwitnessed artifact is a bounded concession, but destroying the agent's source file after
	// snapshotting it half-written is not. If the rename can't be done, the poll reports the error and the
	// phase takes its artifact timeout.
	//
	// Two keys, deliberately: isFinalPoll is the explicit signal the wait loop sends (its final context is
	// live, so ctx.Err() would never fire there), and ctx.Err() still covers a detector polled on a context
	// that died under it mid-wait.
	if isFinalPoll(ctx) || ctx.Err() != nil {
		// The finality concession stops at the pre-dispatch baseline: an artifact byte-identical to what
		// was on disk before the prompt went out is the prior attempt's report. Only an affirmative match
		// refuses; a stat error keeps the concession (fail-open).
		if fi, serr := os.Stat(path); serr == nil && d.baseline.matches(path, fi) {
			return false, completionEvidence{}, "", nil
		}
		return d.completeWith(renameOnlyRelocate)
	}
	fi, serr := os.Stat(path)
	if serr != nil {
		// Vanished between artifactLocate and here (or unreadable): treat as not
		// ready and restart the window rather than completing on a stale read.
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	if d.baseline.matches(path, fi) {
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	// path is part of the key: an artifact that moved between ticks (a fallback the agent rewrote at the
	// canonical path) is a new observation, not a continuation of the old file's window.
	if d.haveLast && path == d.lastPath && fi.Size() == d.lastSize && fi.ModTime().Equal(d.lastModTime) {
		d.stable++
	} else {
		d.stable = 1 // this observation is the first of a new window
	}
	d.haveLast, d.lastPath, d.lastSize, d.lastModTime = true, path, fi.Size(), fi.ModTime()

	if d.stable < artifactStableTicks {
		return false, completionEvidence{}, "", nil
	}
	// The window closed, but the contract may name secondary deliverables; hold completion until every
	// secondary exists non-empty.
	// See ADR-0084.
	if missing := d.missingSecondary(); missing != "" {
		return false, completionEvidence{}, "", nil
	}
	// This artifact has been observed to stop changing, so the full mover (relocateFile) is safe here.
	return d.completeWith(relocateFile)
}

// completeWith canonicalizes the artifact with the caller's mover and reports the phase done; the mover is
// a parameter rather than a constant because the two callers differ in what they have proven about the
// artifact — the window-close path witnessed it settle (relocateFile), the finality short-circuit did not
// (renameOnlyRelocate).
// missingSecondary returns the first contract secondary that does not yet exist non-empty, or "" when the
// set is satisfied (or empty — legacy single-artifact phases are byte-identical).
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

// completionNote is the operator-facing log line for a completing tick, naming
// the non-canonical source when this launch relocated one.
func (d *artifactDetector) completionNote(relocatedFrom string) string {
	if relocatedFrom != "" {
		return fmt.Sprintf("artifact relocated from non-canonical %s → %s; appeared: %s", relocatedFrom, d.cfg.Artifact, d.cfg.Artifact)
	}
	return fmt.Sprintf("artifact appeared: %s", d.cfg.Artifact)
}

// stdoutDetector implements the stdout contract for agents (the router/advisor) that print their answer to
// the REPL and write no artifact file. Completion is the prompt marker visible, the pane stable for
// `threshold` consecutive polls, and the settled pane differing from the baseline (proof the agent produced
// visible output); the baseline-difference check guards against both the marker already being present in
// the just-delivered pane and an agent that crashes back to the bare prompt without ever answering.
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
		// Transient capture error: keep waiting; the reviewer's no-progress budget bounds a genuinely stuck session.
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
