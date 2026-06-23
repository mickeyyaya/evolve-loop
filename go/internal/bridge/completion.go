package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core/evidence"
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

// artifactDetector is the legacy contract: completion = a non-empty file at
// cfg.Artifact (with the cycle-108 non-canonical relocate tolerance). It wraps
// artifactReady verbatim so behavior is identical to the pre-Strategy code.
type artifactDetector struct{ cfg *Config }

func (d *artifactDetector) poll(_ context.Context) (bool, completionEvidence, string, error) {
	ready, from, err := artifactReady(d.cfg)
	if err != nil {
		return false, completionEvidence{}, "", err
	}
	if !ready {
		return false, completionEvidence{}, "", nil
	}
	if from != "" {
		return true, completionEvidence{}, fmt.Sprintf("artifact relocated from non-canonical %s → %s; appeared: %s", from, d.cfg.Artifact, d.cfg.Artifact), nil
	}
	return true, completionEvidence{}, fmt.Sprintf("artifact appeared: %s", d.cfg.Artifact), nil
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
