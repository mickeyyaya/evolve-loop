// Package retro implements the FAIL/WARN-only retrospective phase: a
// post-mortem that writes the retrospective report and a failure lesson.
// See docs/architecture/packages/internal-phases-retro.md.
package retro

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	phaseName  = string(core.PhaseRetro)
	retroAgent = "retrospective"
)

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	NowFn            func() time.Time
	Model            string
	CompactPrompts   bool
	Router           *cliroute.Router
	SleepFn          func(time.Duration)
	ContractVerifier func() runner.ContractVerifier
}

type Phase struct {
	*runner.BaseRunner
}

func New(c Config) *Phase {
	model := c.Model
	if model == "" {
		model = "auto"
	}
	opts := runner.Options{
		Hooks:            hooks{model: model},
		Bridge:           relaunchingBridge(c.Bridge),
		Prompts:          c.Prompts,
		NowFn:            c.NowFn,
		CompactPrompts:   c.CompactPrompts,
		Router:           c.Router,
		SleepFn:          c.SleepFn,
		ContractVerifier: c.ContractVerifier,
	}
	if src, ok := c.Bridge.(signalSource); ok {
		opts.Signals = src.Signals
	}
	return &Phase{BaseRunner: runner.New(opts)}
}

type signalSource interface{ Signals() *signalcenter.Center }

type deliveryRelaunch struct{ core.Bridge }

func relaunchingBridge(b core.Bridge) core.Bridge {
	if b == nil {
		return nil
	}
	return deliveryRelaunch{b}
}

func (b deliveryRelaunch) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	resp, err := b.Bridge.Launch(ctx, req)
	cause := core.DeliveryFailureCause(err)
	if cause == "" || ctx.Err() != nil {
		return resp, err
	}
	fmt.Fprintf(os.Stderr, "[retro] WARN delivery failure (%s) on %s; relaunching once\n", cause, req.CLI)
	keepFirstAttemptLogs(req)
	return b.Bridge.Launch(ctx, req)
}

func keepFirstAttemptLogs(req core.BridgeRequest) {
	for _, stream := range []string{"stdout", "stderr"} {
		name := filepath.Join(req.Workspace, req.Agent+"-"+stream)
		if err := os.Rename(name+".log", name+".attempt-1.log"); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "[retro] WARN the attempt-1 %s log is not kept: %v\n", stream, err)
		}
	}
}

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
	if retroApplies(req) {
		req = refreshExplanationHandoff(ctx, req)
		req.Worktree = retroWorktree(req)
	}
	resp, err := p.BaseRunner.Run(ctx, req)
	if err == nil || resp.Verdict != core.VerdictFAIL {
		return resp, err
	}
	fmt.Fprintf(os.Stderr, "[retro] WARN %v — emitting FAIL verdict; orchestrator routes via failure-adapter (non-fatal)\n", err)
	return resp, nil
}

func retroApplies(req core.PhaseRequest) bool {
	prev := req.Context["previous_verdict"]
	return prev == core.VerdictFAIL || prev == core.VerdictWARN
}

type hooks struct {
	model string
}

func (hooks) PhaseName() string { return phaseName }

func (hooks) AgentPromptName() string { return "evolve-" + retroAgent }

func (hooks) ArtifactFilename(core.PhaseRequest) string { return "retrospective-report.md" }

func (h hooks) DefaultModel() string { return h.model }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	return composePrompt(body, req, req.Context["previous_verdict"])
}

func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if retroApplies(req) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseEnd), nil
}

func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {
	return []string{
		filepath.Join(req.Workspace, "disposition.json"),
		filepath.Join(req.Workspace, "carryover-todos.json"),
	}
}

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict := core.VerdictPASS
	var diagnostics []core.Diagnostic
	advisories, reviewErr := validateExplanationReview(artifact, req)
	for _, advisory := range advisories {
		diagnostics = append(diagnostics, core.Diagnostic{Severity: "warning", Message: explanationdocs.AdvisoryPrefix + advisory})
	}
	if reviewErr != nil {
		diagnostics = append(diagnostics, core.Diagnostic{Severity: "error", Message: reviewErr.Error()})
		verdict = core.VerdictFAIL
	}
	if strings.TrimSpace(artifact) == "" || !hasFailureLesson(req.ProjectRoot, req.Workspace, req.Cycle) {
		verdict = core.VerdictFAIL
	}
	return verdict, diagnostics, string(core.PhaseEnd)
}

func refreshExplanationHandoff(ctx context.Context, req core.PhaseRequest) core.PhaseRequest {
	binding := explanationdocs.CycleBinding{
		ProjectRoot: req.ProjectRoot, Worktree: req.Worktree, Workspace: req.Workspace,
		BaseSHA: req.WorktreeBaseSHA, Cycle: req.Cycle, RunID: req.RunID,
		ContractVersion: req.ExplanationDocumentationVersion,
	}
	hostActive, err := explanationdocs.CrossCheckActivation(binding)
	switch {
	case err != nil:
		return invalidExplanationHandoff(req, compactError(err))
	case !hostActive:
		req.BuildExplanationState = core.BuildExplanationLegacy
		return req
	case req.BuildExplanationState == core.BuildExplanationNotYetBuilt:
		return req
	}
	verified, _, err := explanationdocs.Verify(ctx, binding)
	switch {
	case err != nil:
		return invalidExplanationHandoff(req, compactError(err))
	case !explanationdocs.SameView(req.BuildExplanation, verified):
		return invalidExplanationHandoff(req, "typed handoff does not match verified host snapshot")
	}
	req.BuildExplanation = verified
	req.BuildExplanationState = core.BuildExplanationAvailable
	req.BuildExplanationError = ""
	return req
}

func invalidExplanationHandoff(req core.PhaseRequest, reason string) core.PhaseRequest {
	req.BuildExplanation = nil
	req.BuildExplanationState = core.BuildExplanationInvalid
	req.BuildExplanationError = reason
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
