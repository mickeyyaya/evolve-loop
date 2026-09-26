package core

// See ADR-0044.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// adviseTimeout bounds the LLM consultation on the abort path: long enough
// once to learn a novel signature (each promotion saves ~20 min of maxExtends
// burn on every future occurrence), but the abort must never hang on a
// wedged advisor.
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
// introspection for composition-root wiring tests and the soak preflight,
// since an enforce flip with no adviser wired would silently skip the
// advise→promote path the flip exists to activate.
func (o *Orchestrator) FailureAdviserWired() bool { return o.failureAdviser != nil }

// ModelCatalogLookupWired reports whether the composition root bound the model
// resolvability lookup consulted by router.ClampPlanModelRouting. The clamp
// short-circuits on a nil lookup, so an unwired gate is indistinguishable from
// a passing one at runtime; this seam lets the composition root prove, in a
// real test, that its wiring actually reaches production.
func (o *Orchestrator) ModelCatalogLookupWired() bool { return o.modelCatalogLookup != nil }

// fatalSignaturesDir is where validated promotions persist, relative to the
// project root (the same dir the tmux driver replays at boot).
func fatalSignaturesDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "instincts", "fatal-signatures")
}

// adviseOnUnclassifiedFailure runs the C3 escalate→advise→promote path for
// one aborted phase.
func (o *Orchestrator) adviseOnUnclassifiedFailure(ctx context.Context, cycle int, workspace, projectRoot string, phase Phase, failErr error, env map[string]string) {
	if o.failureAdviser == nil || o.cfg.PhaseRecovery != config.StageEnforce {
		return
	}
	if !errors.Is(failErr, ErrArtifactTimeout) {
		return
	}
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
	// Scan the agent-stripped pane, not the raw evidence, so a genuinely novel
	// wedge whose pane merely quotes a seeded signature in agent-authored diff
	// content is not misread as "already classified". The empty prompt is
	// deliberate: the echo half is neutered by the protect list regardless, so
	// plumbing the phase prompt in here would add I/O for no behavior change.
	if cause, _, known := det.Detect(recovery.StripAgentContent(report.FinalPane, "", nil)); known {
		// Deterministic-first: the registry already classifies this pane —
		// the fast-fail (C2) owns acting on it; no LLM consultation.
		fmt.Fprintf(os.Stderr, "[orchestrator] phase-recovery: pane already classified (%s); skipping advisor\n", cause)
		return
	}
	advCtx, advCancel := context.WithTimeout(ctx, adviseTimeout)
	defer advCancel()
	advice, aerr := o.failureAdviser.Advise(advCtx, FailureAdviseInput{
		Phase:       string(phase),
		ExitCode:    failurediag.ExitCodeArtifactTimeout,
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
