package core

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

const adviseTimeout = 3 * time.Minute

type FailureAdviser interface {
	Advise(ctx context.Context, in FailureAdviseInput) (*recovery.FailureAdvice, error)
}

func WithFailureAdviser(a FailureAdviser) Option {
	return func(o *Orchestrator) {
		if a != nil {
			o.failureAdviser = a
		}
	}
}

func (o *Orchestrator) FailureAdviserWired() bool { return o.failureAdviser != nil }

func fatalSignaturesDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "instincts", "fatal-signatures")
}

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
	if cause, _, known := det.Detect(recovery.StripAgentContent(report.FinalPane, "", nil)); known {
		fmt.Fprintf(o.errOut(), "[orchestrator] phase-recovery: pane already classified (%s); skipping advisor\n", cause)
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
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN phase-recovery advisor: %v (escalating to operator)\n", aerr)
		return
	}
	if perr := recovery.PromoteAdvice(det, sigDir, *advice); perr != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN phase-recovery promotion rejected: %v\n", perr)
		return
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] phase-recovery: advisor classified %s pane as %s; signature promoted (%s)\n", phase, advice.Cause, advice.Justification)
}
