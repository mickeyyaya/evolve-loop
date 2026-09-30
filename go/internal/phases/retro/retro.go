// Package retro implements the FAIL/WARN-only retrospective phase: a
// post-mortem that writes the retrospective report and a failure lesson.
// See docs/architecture/packages/internal-phases-retro.md.
package retro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

const phaseName = string(core.PhaseRetro)

type Config struct {
	Bridge         core.Bridge
	Prompts        *prompts.Loader
	NowFn          func() time.Time
	Model          string
	CompactPrompts bool
}

type Phase struct {
	bridge         core.Bridge
	prompts        *prompts.Loader
	nowFn          func() time.Time
	model          string
	compactPrompts bool
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	model := c.Model
	if model == "" {
		model = "auto"
	}
	return &Phase{bridge: c.Bridge, prompts: c.Prompts, nowFn: nowFn, model: model, compactPrompts: c.CompactPrompts}
}

func (p *Phase) Name() string { return phaseName }

func retroWorktree(req core.PhaseRequest) string {
	bridgeGuardAcceptsWorktree := !fleetMode(req) || gobridge.IsDir(req.Worktree)
	if bridgeGuardAcceptsWorktree {
		return req.Worktree
	}
	return gobridge.ScratchCwd(req.Workspace, "retro-scratch-cwd")
}

func fleetMode(req core.PhaseRequest) bool {
	v := req.Env[ipcenv.FleetKey]
	if v == "" {
		v = os.Getenv(ipcenv.FleetKey)
	}
	return envchain.BoolValue(v, false)
}

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()

	prev := req.Context["previous_verdict"]
	if prev != core.VerdictFAIL && prev != core.VerdictWARN {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictSKIPPED,
			NextPhase:    string(core.PhaseEnd),
			ArtifactsDir: req.Workspace,
			DurationMS:   p.nowFn().Sub(start).Milliseconds(),
		}, nil
	}
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: prompts loader required")
	}
	req = refreshExplanationHandoff(ctx, req)

	agent, err := p.prompts.Agent("evolve-retrospective")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: load agent: %w", err)
	}

	body := agent.Body
	if p.compactPrompts {
		body = prompts.StripOnDemandSections(body)
	}
	prompt := composePrompt(body, req, prev)
	artifactPath := filepath.Join(req.Workspace, "retrospective-report.md")
	profilePath := filepath.Join(req.ProjectRoot, ".evolve", "profiles", "retrospective.json")

	var prof profiles.Profile
	haveProf := false
	if loader := profiles.NewFromDir(filepath.Join(req.ProjectRoot, ".evolve", "profiles")); loader != nil {
		if loaded, err := loader.Get("retrospective"); err == nil {
			prof = loaded
			haveProf = true
		}
	}
	cli := resolveCLI(req.Env["EVOLVE_CLI"], prof.CLI)

	model := p.model
	if model == "auto" {
		model = "balanced"
		if haveProf && prof.ModelTierDefault != "" {
			model = prof.ModelTierDefault
		}
	}

	overlaySkills := policy.ResolveLaunchOverlaysFailOpen(req.ProjectRoot, phaseName, cli, model)

	bridgeReq := core.BridgeRequest{
		CLI:         cli,
		Profile:     profilePath,
		Model:       model,
		Prompt:      prompt,
		Workspace:   req.Workspace,
		Worktree:    retroWorktree(req),
		ProjectRoot: req.ProjectRoot,
		SecondaryArtifacts: []string{
			filepath.Join(req.Workspace, "disposition.json"),
			filepath.Join(req.Workspace, "carryover-todos.json"),
		},
		ArtifactPath: artifactPath,
		Agent:        "retrospective",
		Cycle:        req.Cycle,
		Env:          req.Env,
		Skills:       overlaySkills,
	}
	fence := treefence.Begin(ctx, bridgeReq.Worktree, req.WorktreeReadOnly)
	if err := fence.TakeErr(); err != nil {
		fmt.Fprintf(os.Stderr, "[retro] WARN worktree fence: snapshot failed (%v) — the tree this phase hands downstream is unverified\n", err)
	}
	bres, bridgeErr := p.bridge.Launch(ctx, bridgeReq)
	if bridgeErr != nil && core.DeliveryFailureCause(bridgeErr) != "" {
		bres, bridgeErr = p.bridge.Launch(ctx, bridgeReq)
	}
	durationMS := p.nowFn().Sub(start).Milliseconds()
	fenceDiags := fence.End(context.WithoutCancel(ctx)).Diagnostics(phaseName)
	for _, d := range fenceDiags {
		fmt.Fprintf(os.Stderr, "[retro] WARN %s\n", d.Message)
	}

	if bridgeErr != nil {
		fmt.Fprintf(os.Stderr, "[retro] WARN bridge failed (%v) — emitting FAIL verdict; orchestrator routes via failure-adapter (non-fatal)\n", bridgeErr)
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			NextPhase:    string(core.PhaseEnd),
			CostUSD:      bres.CostUSD,
			Tokens:       bres.Tokens,
			DurationMS:   durationMS,
			Diagnostics:  append(fenceDiags, core.Diagnostic{Severity: "error", Message: bridgeErr.Error()}),
		}, nil
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}
	verdict := core.VerdictPASS
	diagnostics := fenceDiags
	advisories, reviewErr := validateExplanationReview(content, req)
	for _, advisory := range advisories {
		diagnostics = append(diagnostics, core.Diagnostic{Severity: "warning", Message: explanationdocs.AdvisoryPrefix + advisory})
	}
	if reviewErr != nil {
		diagnostics = append(diagnostics, core.Diagnostic{Severity: "error", Message: reviewErr.Error()})
		verdict = core.VerdictFAIL
	}
	if strings.TrimSpace(content) == "" || !hasFailureLesson(req.ProjectRoot, req.Workspace, req.Cycle) {
		verdict = core.VerdictFAIL
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      verdict,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseEnd),
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
		Diagnostics:  diagnostics,
	}, nil
}

func resolveCLI(envCLI, profileCLI string) string {
	switch {
	case envCLI != "":
		return envCLI
	case profileCLI != "":
		return profileCLI
	}
	return "claude-tmux"
}

func refreshExplanationHandoff(ctx context.Context, req core.PhaseRequest) core.PhaseRequest {
	if req.ExplanationDocumentationVersion == 0 {
		req.BuildExplanationState = core.BuildExplanationLegacy
		return req
	}
	if req.BuildExplanationState == core.BuildExplanationNotYetBuilt {
		return req
	}
	verified, active, err := explanationdocs.Verify(ctx, explanationdocs.CycleBinding{
		ProjectRoot: req.ProjectRoot, Worktree: req.Worktree, Workspace: req.Workspace,
		BaseSHA: req.WorktreeBaseSHA, Cycle: req.Cycle, RunID: req.RunID,
		ContractVersion: req.ExplanationDocumentationVersion,
	})
	if err != nil || !active || !explanationdocs.SameView(req.BuildExplanation, verified) {
		req.BuildExplanation = nil
		req.BuildExplanationState = core.BuildExplanationInvalid
		switch {
		case err != nil:
			req.BuildExplanationError = compactError(err)
		case !active:
			req.BuildExplanationError = "active explanation contract was not found"
		default:
			req.BuildExplanationError = "typed handoff does not match verified host snapshot"
		}
		return req
	}
	req.BuildExplanation = verified
	req.BuildExplanationState = core.BuildExplanationAvailable
	req.BuildExplanationError = ""
	return req
}

func compactError(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func composePrompt(body string, req core.PhaseRequest, prev string) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- previous_verdict: %s\n", prev)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	runner.AppendExplanationContext(&b, req)
	return b.String()
}

const lessonsDirRel = ".evolve/instincts/lessons"

func lessonPrefixForCycle(cycle int) string {
	return "inst-L" + strconv.Itoa(cycle)
}

func matchesCycleLesson(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	charAfterCycle := name[len(prefix)]
	return charAfterCycle < '0' || charAfterCycle > '9'
}

func hasFailureLesson(projectRoot, ws string, cycle int) bool {
	if cycle > 0 && projectRoot != "" {
		prefix := lessonPrefixForCycle(cycle)
		if entries, err := os.ReadDir(filepath.Join(projectRoot, lessonsDirRel)); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				n := e.Name()
				if matchesCycleLesson(n, prefix) && strings.HasSuffix(n, ".yaml") {
					return true
				}
			}
		}
	}
	return hasLegacyWorkspaceFailureLesson(ws)
}

func hasLegacyWorkspaceFailureLesson(ws string) bool {
	entries, err := os.ReadDir(ws)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, "failure-lesson") && strings.HasSuffix(n, ".yaml") {
			return true
		}
	}
	return false
}

func init() {
	registry.Register(string(core.PhaseRetro), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot, nil),
			Prompts: prompts.NewForProject(req.ProjectRoot),
			Model:   "auto",
		})
	})
}
