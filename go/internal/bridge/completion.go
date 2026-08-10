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

// completion.go — the phase-completion Strategy (ADR-0027). runTmuxREPL used
// to hardcode one completion contract: poll for a non-empty artifact file
// (artifactReady). But there are several contracts in play —
//   - artifact: the agent's deliverable is a file it writes (scout/build/…);
//   - stdout:   the agent prints its answer to the REPL and writes no file
//     (the router/advisor — a meta phase whose JSON the orchestrator parses);
//   - git-evidence (ADR-0027, a later PR): the agent commits its deliverable
//     and completion is "HEAD advanced + Evolve-Phase trailer verified".
//
// A completionDetector decouples "is the phase done?" from the wait loop so
// the loop body (ADR-0026 stop-review/extend, auto-respond, inbox drain) stays
// identical regardless of contract. The detector ONLY decides readiness;
// liveness (extend vs pause) remains the reviewer's job.
//
// Default ("" / "artifact") preserves the legacy path-poll byte-for-byte, so
// the abstraction is dormant until a phase opts into a different contract.

// stdoutIdlePolls is how many consecutive unchanged poll ticks (each ~2s in
// the wait loop) with the REPL prompt marker visible count as "the turn
// finished" for the stdout contract. Debounce: a streaming agent's pane
// changes every tick, so the counter only accrues once output has settled.
const stdoutIdlePolls = 3

// artifactStableTicks is the artifact twin of stdoutIdlePolls: how many
// consecutive poll ticks (each ~2s in the wait loop) must observe the SAME
// (size, mtime) before the deliverable counts as finished. Two is the minimum
// that is actually a window — one observation is just the legacy first-sight
// read — and each extra tick costs ~2s on every phase, so it stays small.
const artifactStableTicks = 2

// finalPollGrace bounds the wait loop's ONE post-cancel completion poll. The
// poll runs on a context DETACHED from the cancelled one (see withFinalPoll) so
// detectors that shell a subprocess can actually run it; this timeout is what
// keeps that detachment from re-introducing an unbounded wait during teardown.
// Sized for a single tmux capture / `git rev-parse`, not for an agent turn.
const finalPollGrace = 5 * time.Second

// finalPollCtxKey marks a context as the wait loop's LAST look before it gives
// up. Finality is signalled EXPLICITLY rather than inferred from ctx.Err():
// the final poll now carries a live context (a dead one cannot fork tmux or
// git — completion.go's stdout/git detectors were starved by exactly that), so
// cancellation is no longer observable to the detector at all.
type finalPollCtxKey struct{}

// withFinalPoll returns the context for the wait loop's final completion poll:
// detached from the caller's cancellation, bounded by finalPollGrace, and
// carrying the finality marker. The caller MUST call the returned cancel.
func withFinalPoll(ctx context.Context) (context.Context, context.CancelFunc) {
	live := context.WithValue(context.WithoutCancel(ctx), finalPollCtxKey{}, true)
	return context.WithTimeout(live, finalPollGrace)
}

// isFinalPoll reports whether this poll is the wait loop's last look.
func isFinalPoll(ctx context.Context) bool {
	final, _ := ctx.Value(finalPollCtxKey{}).(bool)
	return final
}

// completionEvidence carries what a detector observed at completion. Empty for
// the artifact contract (the file at cfg.Artifact is the evidence, read by the
// engine). Reserved for the git-evidence contract (commit SHA) in a later PR.
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
func newCompletionDetector(mode string, cfg *Config, deps Deps, lp tmuxLaunch) completionDetector {
	switch mode {
	case "stdout":
		return &stdoutDetector{cfg: cfg, deps: deps, lp: lp, threshold: stdoutIdlePolls}
	case "git":
		return newGitEvidenceDetector(cfg, deps)
	default:
		return &artifactDetector{cfg: cfg}
	}
}

// gitEvidenceDetector implements the ADR-0027 git-evidence contract: completion
// = the worktree HEAD advanced to a NEW commit carrying an Evolve-Phase trailer
// for this phase AND the cycle's challenge token. HEAD-advance alone is
// insufficient (a stray/unrelated commit must not false-complete), so the
// trailer is verified; an advance without a matching trailer just re-baselines
// and keeps watching. gitCmd is a seam (default shells `git -C <worktree>` via
// deps.Runner) so the detector is unit-testable without a real repo.
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
		// Fail-closed: an empty token makes Verify always false, so the detector
		// would wait forever. Surface it loudly rather than hang silently — the
		// prompt template likely omitted $CHALLENGE_TOKEN.
		fmt.Fprintf(deps.Stderr, "[git-evidence] WARN: challenge-token.txt missing/empty in %s — completion will never verify\n", cfg.Workspace)
	}
	worktree := cfg.Worktree
	return &gitEvidenceDetector{
		phase:       cfg.Agent,
		expectedTok: tok,
		gitCmd: func(ctx context.Context, args ...string) (string, error) {
			var out strings.Builder
			full := append([]string{"-C", worktree}, args...)
			_, err := deps.Runner(ctx, "git", "", full, driverEnv(deps), nil, &out, io.Discard)
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
	// HEAD advanced — scan EVERY new commit in baseline..HEAD, not just the tip.
	// Two commits can land between polls (e.g. the phase evidence commit then an
	// orchestrator commit); inspecting only HEAD would re-baseline past the
	// evidence commit and wait forever. rev-list lists newest-first; any
	// verifying commit in the range completes.
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
	// No verifying commit in the new range (only stray commits): re-baseline to
	// HEAD and keep watching rather than false-completing.
	d.baseline = head
	return false, completionEvidence{}, "", nil
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// artifactDetector implements the artifact contract: completion = a non-empty
// file at cfg.Artifact (with the cycle-108 non-canonical relocate tolerance)
// that has STOPPED CHANGING. artifactLocate answers "is it there?" without
// touching the file; a cross-poll stability window answers "is it finished?";
// artifactReady then canonicalizes it.
//
// Why the window (cycle-1198): an agent's deliverable is typically a Write
// followed seconds later by an Edit. First-sight completion accepted the
// half-written intermediate — the gate rejected a scout-report.md that parsed
// perfectly moments afterwards. The deliverable-side grace retry
// (deliverable.go) covers absence/emptiness only; a "parses fine, wrong
// content" read is not retried, by design. So the fix belongs here, at the
// source.
//
// The state is carried ACROSS poll calls, not within one: an in-poll settle
// sleep (the rejected cycle-1212 design) is tens of milliseconds and cannot
// span a multi-second Write→Edit gap. The wait loop already calls poll every
// ~2s; that cadence IS the window. mtime is in the stability key because size
// alone is content-blind to an equal-length fix-up Edit.
// The window gates the DESTRUCTIVE relocation, it does not merely follow it
// (cycle-1249). poll's read-only half is artifactLocate; relocateFile — whose
// cross-device branch copies a partial file into the canonical path and then
// REMOVES the source the agent is still appending to — runs only on the tick the
// window closes. Relocating that way on first sight would defeat the debounce on
// the very path scout flagged as highest-risk, leaving a permanently stable,
// permanently truncated deliverable that the window then certifies as finished.
//
// Precisely stated, because the earlier wording of this paragraph claimed more
// than the code did and cycle-1256 audited it as a refuted safety claim (D2):
// the ONE path that completes without a closed window — the finality
// short-circuit below — still canonicalizes a non-canonical fallback, but it is
// restricted to renameOnlyRelocate. Rename relinks an inode and so is safe for a
// file that may still be growing; copy+remove is not, and under finality it
// never runs.
type artifactDetector struct {
	cfg *Config

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
	// Final look: this is the wait loop's ONE last poll before it gives up
	// (driver_tmux_repl.go). Demanding a fresh window it can never get would
	// launder every finished-at-the-buzzer session into ExitArtifactTimeout —
	// turning a truncated-read fix into a worse false-FAIL generator. The
	// artifact is on disk, which is the evidence; short-circuit. Checked AFTER
	// artifactLocate, whose found result already proves a non-empty artifact
	// exists, so finality can never manufacture completion from nothing.
	//
	// stable is 0 here — no window was ever closed on this artifact — so the
	// mover is renameOnlyRelocate, NOT relocateFile (cycle-1256 D1). Completing
	// on an unwitnessed artifact is a deliberate, bounded concession; deleting
	// the agent's source file after snapshotting it half-written is not, and
	// that is exactly what relocateFile's copy+remove branch does. Rename-only
	// keeps the concession reversible: worst case the canonical path holds a
	// file the agent's fd is still appending into, best case a finished one, and
	// never a truncated snapshot with the original destroyed. If the rename
	// cannot be done, the poll reports the error and the phase takes its
	// artifact timeout — the honest outcome for "we could not safely finish".
	//
	// Two keys, deliberately: isFinalPoll is the explicit signal the wait loop
	// now sends (its final context is LIVE, so ctx.Err() would never fire there
	// again), and ctx.Err() still covers a detector polled on a context that
	// died under it mid-wait — the pre-existing contract, unchanged.
	if isFinalPoll(ctx) || ctx.Err() != nil {
		return d.completeWith(renameOnlyRelocate)
	}
	fi, serr := os.Stat(path)
	if serr != nil {
		// Vanished between artifactLocate and here (or unreadable): treat as not
		// ready and restart the window rather than completing on a stale read.
		d.haveLast, d.stable = false, 0
		return false, completionEvidence{}, "", nil
	}
	// path is part of the key: an artifact that moved between ticks (a fallback
	// the agent rewrote at the canonical path) is a NEW observation, not a
	// continuation of the old file's window.
	if d.haveLast && path == d.lastPath && fi.Size() == d.lastSize && fi.ModTime().Equal(d.lastModTime) {
		d.stable++
	} else {
		d.stable = 1 // this observation is the first of a new window
	}
	d.haveLast, d.lastPath, d.lastSize, d.lastModTime = true, path, fi.Size(), fi.ModTime()

	if d.stable < artifactStableTicks {
		return false, completionEvidence{}, "", nil
	}
	// The window closed — but the contract may name SECONDARY deliverables
	// (Phase B, the single-artifact cutoff class: retro wrote its report and
	// the session died before disposition.json). Hold completion until every
	// secondary EXISTS non-empty; the artifact-timeout final poll above still
	// completes without them (bounded wait), and the phase gate then names
	// the absence loudly.
	if missing := d.missingSecondary(); missing != "" {
		return false, completionEvidence{}, "", nil
	}
	// The window closed: this artifact HAS been observed to stop changing, so
	// the full mover — including relocateFile's cross-device copy+remove — is
	// safe here and only here.
	return d.completeWith(relocateFile)
}

// completeWith canonicalizes the artifact with the caller's mover and reports
// the phase done. This is the ONLY place the non-canonical fallback is moved —
// deferring the move to here is what keeps a still-growing fallback where the
// agent left it. The mover is a parameter rather than a constant because the two
// callers differ in what they have PROVEN about the artifact: the window-close
// path witnessed it settle (relocateFile), the finality short-circuit did not
// (renameOnlyRelocate). A relocation failure surfaces as the detector's error
// (the wait loop logs it once); an artifact that vanished between the window's
// last look and this call restarts the window rather than completing on nothing.
// missingSecondary returns the first contract secondary that does not yet
// exist non-empty, or "" when the set is satisfied (or empty — legacy
// single-artifact phases are byte-identical).
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

// stdoutDetector implements the stdout contract for agents (the router/advisor)
// that print their answer to the REPL and write no artifact file. Completion =
// the prompt marker is visible AND the pane has been stable for `threshold`
// consecutive polls AND the settled pane DIFFERS from the baseline (proof the
// agent produced visible output). The baseline-difference check guards two
// false-fires at once: the marker being present in the just-delivered pane
// before the turn starts, and an agent that crashes and reverts the pane to the
// bare prompt (== baseline) without ever answering.
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
		// Transient capture error: keep waiting. The reviewer's no-progress
		// budget bounds a genuinely stuck session, so we never swallow a hang.
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
