package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// phasePreparation is the immutable output of the pre-dispatch stage. It
// carries only inputs needed by routing, dispatch, and reconciliation.
type phasePreparation struct {
	start          time.Time
	phase          string
	prompt         string
	artifactPath   string
	preDispatch    artifactSnapshot
	hadPreDispatch bool
	profileDir     string
	profileName    string
	profilePath    string
	profile        *profiles.Profile
}

// preparePhaseExecution validates the runner, resolves the prompt and profile,
// and snapshots the artifact before any bridge side effect. A non-nil early
// response represents a phase-owned skip or a typed configuration failure.
func (b *BaseRunner) preparePhaseExecution(req core.PhaseRequest) (phasePreparation, *core.PhaseResponse, error) {
	prep := phasePreparation{start: b.nowFn(), phase: b.hooks.PhaseName()}
	if b.bridge == nil {
		return prep, nil, fmt.Errorf("%s: bridge required", prep.phase)
	}
	if b.prompts == nil {
		return prep, nil, fmt.Errorf("%s: prompts loader required", prep.phase)
	}

	if skipper, ok := b.hooks.(Skipper); ok {
		if skipped, verdict, nextPhase, diags := skipper.ShouldSkip(req); skipped {
			resp := core.PhaseResponse{
				Phase:        prep.phase,
				Verdict:      verdict,
				ArtifactsDir: req.Workspace,
				NextPhase:    nextPhase,
				DurationMS:   b.nowFn().Sub(prep.start).Milliseconds(),
				Diagnostics:  diags,
			}
			return prep, &resp, nil
		}
	}

	body, inline := "", false
	if provider, ok := b.hooks.(InlinePromptProvider); ok {
		body, inline = provider.InlinePromptBody()
	}
	if !inline {
		agent, err := b.prompts.Agent(b.hooks.AgentPromptName())
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && !errors.Is(err, prompts.ErrNoSource) {
				return prep, nil, fmt.Errorf("%s: load agent: %w: %w", prep.phase, core.ErrAgentDocMissing, err)
			}
			return prep, nil, fmt.Errorf("%s: load agent: %w", prep.phase, err)
		}
		body = agent.Body
		if b.compactPrompts {
			body = prompts.StripOnDemandSections(body)
		}
	}
	if req.ExplanationDocumentationVersion != 0 {
		var err error
		body, err = b.withExplanationContract(body, prep.phase)
		if err != nil {
			return prep, nil, fmt.Errorf("%s: load explanation contract: %w", prep.phase, err)
		}
	}

	materialized := digest.Materialize([]byte(body), prep.phase)
	shadow := digest.NewShadowRecord([]byte(body), materialized)
	b.diag.Infof("%s\n", FormatDigestShadowLog(prep.phase, shadow))
	if materialized.Outcome == digest.OutcomeMalformed {
		return prep, nil, fmt.Errorf("%s: materialize digest: %w", prep.phase, materialized.Err)
	}
	if shadow.Parity {
		body = string(materialized.Digest)
	}

	prep.prompt = b.hooks.ComposePrompt(body, req)
	prep.artifactPath = filepath.Join(req.Workspace, b.hooks.ArtifactFilename(req))
	prep.preDispatch, prep.hadPreDispatch = statArtifactSnapshot(prep.artifactPath)
	prep.profileDir = filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	prep.profileName = strings.TrimPrefix(b.hooks.AgentPromptName(), "evolve-")
	prep.profilePath = filepath.Join(prep.profileDir, prep.profileName+".json")

	if loader := profiles.NewFromDir(prep.profileDir); loader != nil {
		if profile, err := loader.Get(prep.profileName); err == nil {
			prep.profile = &profile
		} else if info, statErr := os.Stat(prep.profileDir); statErr == nil && info.IsDir() {
			msg := fmt.Sprintf("profile not found: %s", prep.profilePath)
			resp := core.PhaseResponse{
				Phase:        prep.phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
			}
			return prep, &resp, fmt.Errorf("%s: %s: %w", prep.phase, msg, err)
		}
	}

	if prep.profile != nil && prep.profile.TurnBudgetHint > 0 {
		prep.prompt += fmt.Sprintf("\n\n## Budget\nAdvisory turn budget for this phase: ~%d turns. Prioritize breadth over depth; write your report as soon as the completion gates are satisfied.\n", prep.profile.TurnBudgetHint)
	}
	if contract, ok := phasecontract.For(prep.phase); ok && contract.RequireChallengeToken {
		if tokenBytes, err := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); err == nil {
			if token := strings.TrimSpace(string(tokenBytes)); token != "" {
				prep.prompt += fmt.Sprintf("\n\n## Challenge Token (proof-of-read — MANDATORY)\nCopy this token verbatim into your report as an HTML comment near the top: <!-- challenge-token: %s -->\nA report without it is rejected and re-dispatched.\n", token)
			}
		}
	}
	return prep, nil, nil
}
