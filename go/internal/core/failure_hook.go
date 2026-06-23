package core

// failure_hook.go — ADR-0044 C3: the orchestrator's escalate→advise→promote
// hook, the composition point where the chain's ActionAdvise verdict becomes
// a real LLM consultation. Runs ONLY when the program dial is at enforce
// (cfg.PhaseRecovery — shadow, the default, never dispatches an advisor) and
// ONLY for an artifact-timeout abort whose escalation report carries a pane
// the deterministic registry cannot classify. Strictly best-effort: every
// failure is a WARN; the hook never alters the abort control flow — it runs
// AFTER the phase outcome is recorded and the failure diag is written, and
// its only durable side effect is a validated promotion
// (recovery.PromoteAdvice) that makes the NEXT occurrence deterministic.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
)

// adviseTimeout bounds the LLM consultation on the abort path. The cycle has
// already failed when the hook runs, so spending up to this long ONCE to
// learn a novel signature is a good trade (each promotion saves ~20 min of
// maxExtends burn on every future occurrence) — but the abort must never
// hang on a wedged advisor. 3 min covers a tmux REPL boot + an 8-turn
// classification; the parent ctx still wins if shorter.
const adviseTimeout = 3 * time.Minute

// FailureAdviser is the port the hook consults — satisfied by
// *FailureAdvisor (the bridge-backed LLM tail) and by test fakes. Mirrors
// how router.Proposer/Planner port the PhaseAdvisor.
type FailureAdviser interface {
	Advise(ctx context.Context, in FailureAdviseInput) (*recovery.FailureAdvice, error)
}

// WithFailureAdviser injects the ADR-0044 LLM failure-advisor tail. Nil (the
// default) keeps the hook inert regardless of stage.
func WithFailureAdviser(a FailureAdviser) Option {
	return func(o *Orchestrator) {
		if a != nil {
			o.failureAdviser = a
		}
	}
}

// FailureAdviserWired reports whether the advisor tail is injected —
// introspection for composition-root wiring tests and the soak preflight
// (R8: an enforce flip with no adviser wired would silently skip the
// advise→promote path the flip exists to activate).
func (o *Orchestrator) FailureAdviserWired() bool { return o.failureAdviser != nil }

// fatalSignaturesDir is where validated promotions persist, relative to the
// project root (the same dir the tmux driver replays at boot).
func fatalSignaturesDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "instincts", "fatal-signatures")
}

// adviseOnUnclassifiedFailure runs the C3 escalate→advise→promote path for
// one aborted phase. See the file header for the gating contract.
func (o *Orchestrator) adviseOnUnclassifiedFailure(ctx context.Context, cycle int, workspace, projectRoot string, phase Phase, failErr error, env map[string]string) {
	if o.failureAdviser == nil || o.cfg.PhaseRecovery != config.StageEnforce {
		return
	}
	if !errors.Is(failErr, ErrArtifactTimeout) {
		return // only the timeout family carries a pane worth classifying
	}
	// The bridge's escalation report carries the final pane (final_pane) —
	// the evidence envelope. Absent report ⇒ nothing to classify.
	data, err := os.ReadFile(filepath.Join(workspace, string(phase)+"-escalation-report.json"))
	if err != nil {
		return
	}
	var report struct {
		FinalPane string `json:"final_pane"`
	}
	if jerr := json.Unmarshal(data, &report); jerr != nil || report.FinalPane == "" {
		return
	}
	sigDir := fatalSignaturesDir(projectRoot)
	det := recovery.SeedDetectorWithPromotions(sigDir)
	if cause, _, known := det.Detect(report.FinalPane); known {
		// Deterministic-first: the registry already classifies this pane —
		// the fast-fail (C2) owns acting on it; no LLM consultation.
		fmt.Fprintf(os.Stderr, "[orchestrator] phase-recovery: pane already classified (%s); skipping advisor\n", cause)
		return
	}
	advCtx, advCancel := context.WithTimeout(ctx, adviseTimeout)
	defer advCancel()
	advice, aerr := o.failureAdviser.Advise(advCtx, FailureAdviseInput{
		Phase:       string(phase),
		ExitCode:    81,
		PaneTail:    report.FinalPane,
		Workspace:   workspace,
		ProjectRoot: projectRoot,
		Cycle:       cycle,
		Env:         env,
	})
	if aerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-recovery advisor: %v (escalating to operator)\n", aerr)
		return
	}
	if perr := recovery.PromoteAdvice(det, sigDir, *advice); perr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-recovery promotion rejected: %v\n", perr)
		return
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] phase-recovery: advisor classified %s pane as %s; signature promoted (%s)\n", phase, advice.Cause, advice.Justification)
}
