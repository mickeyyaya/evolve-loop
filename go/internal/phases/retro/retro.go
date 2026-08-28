// Package retro implements the FAIL/WARN-only post-mortem phase as a
// core.PhaseRunner. It runs only when the previous verdict is FAIL or
// WARN; PASS cycles short-circuit to SKIPPED (the Memo phase handles
// PASS-cycle observation).
//
// Verdict mapping:
//
//   - previous verdict != FAIL/WARN → SKIPPED, no bridge call
//   - retrospective.md non-empty AND a failure lesson for THIS cycle → PASS
//     (resolved where the persona writes them: .evolve/instincts/lessons/
//     inst-L<cycle>*.yaml; the legacy workspace failure-lesson*.yaml shape is
//     still accepted — see hasFailureLesson)
//   - otherwise → FAIL
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
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const phaseName = string(core.PhaseRetro)

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// Model is the LLM model passed to the bridge for the retrospective run.
	// Empty string defaults to "auto".
	Model string
	// CompactPrompts strips the on-demand reference tail (below ## Reference Index)
	// from the agent body before dispatching, mirroring BaseRunner compaction.
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

// retroWorktree resolves the working directory this retro launch is dispatched
// against.
//
// A LIVE provisioned worktree passes through verbatim — a fallback that fired
// unconditionally would strand every normal retro in an empty scratch dir with
// no repo. The fallback exists for one window only: under a fleet supervisor the
// bridge drivers REFUSE a launch whose working dir does not clear the guard —
// empty is refused as errWorktreeRequired (bridge/driver_tmux_repl.go:27) and a
// non-existent path is refused at gobridge.IsDir (driver_tmux_repl.go:123,
// ExitBadFlags, stderr only, no error return) — instead of falling back to the
// process cwd. Either way a lane whose worktree was torn down (or never
// provisioned after exhausted retries) loses its retrospective entirely — a
// failure in the failure-handler. Retro is read-mostly and Evaluate-archetype,
// so a disposable cwd under the workspace it already owns clears the guard.
//
// The condition is the guard's own predicate, not a string shape: a torn-down
// fleet lane hands retro a NON-EMPTY but stale path (cs.ActiveWorktree), and
// testing only for "" would pass it straight into the refusal. Both no-worktree
// shapes are one contract (cycle-1278; the empty half alone was cycle-1270).
//
// The two shapes deliberately NOT used: the shared main tree (req.ProjectRoot —
// refuted by PR #400; worktree is the write-authority predicate) and the
// dispatching process cwd (the exact leak the guard closes). With no owned
// workspace there is nowhere safe to mint, so this returns "" and the bridge
// decides exactly as it does today — never a fabricated path. The guard itself is
// untouched: the fix is supplied by the phase.
func retroWorktree(req core.PhaseRequest) string {
	if !fleetMode(req) || gobridge.IsDir(req.Worktree) {
		return req.Worktree
	}
	return gobridge.ScratchCwd(req.Workspace, "retro-scratch-cwd")
}

// fleetMode reads the fleet flag through the SAME key + parser the bridge guard
// uses (ipcenv.FleetKey + envchain.BoolValue, driver_tmux_repl.go:117), so the
// phase and the guard can never disagree about which launches fail closed. The
// request env wins; the process env is the fallback, matching the driver's own
// env-chain lookup order.
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

	// CLI resolution chain: EVOLVE_CLI > profile.cli > claude-tmux — matching
	// BaseRunner (runner.go). This hand-rolled runner had regressed to the
	// cycle-107 class (EVOLVE_CLI-or-hardcoded, profile.cli ignored), which
	// made the 2026-08-26 deep-tier sol arrangement's flagship flip —
	// retrospective, ~40% of deep dispatch volume — dead on arrival until
	// review caught it against the dispatched BridgeRequest.
	var prof profiles.Profile
	haveProf := false
	if loader := profiles.NewFromDir(filepath.Join(req.ProjectRoot, ".evolve", "profiles")); loader != nil {
		if loaded, err := loader.Get("retrospective"); err == nil {
			prof = loaded
			haveProf = true
		}
	}
	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		if haveProf && prof.CLI != "" {
			cli = prof.CLI
		}
	}
	if cli == "" {
		cli = "claude-tmux"
	}

	model := p.model
	if model == "auto" {
		model = "balanced"
		if haveProf && prof.ModelTierDefault != "" {
			model = prof.ModelTierDefault
		}
	}

	// Skill overlays: resolve the tier-gated persona for this retro launch and
	// thread the NAMES onto BridgeRequest.Skills, matching the phase runner — a
	// deep/top-tier retro gets the fable operating-discipline overlay. Fail-open.
	overlaySkills := policy.ResolveLaunchOverlaysFailOpen(req.ProjectRoot, phaseName, cli, model)

	bridgeReq := core.BridgeRequest{
		CLI:                cli,
		Profile:            profilePath,
		Model:              model,
		Prompt:             prompt,
		Workspace:          req.Workspace,
		Worktree:           retroWorktree(req),
		SecondaryArtifacts: []string{filepath.Join(req.Workspace, "disposition.json")},
		ArtifactPath:       artifactPath,
		Agent:              "retrospective",
		Cycle:              req.Cycle,
		Env:                req.Env,
		Skills:             overlaySkills,
	}
	bres, bridgeErr := p.bridge.Launch(ctx, bridgeReq)
	if bridgeErr != nil && core.DeliveryFailureCause(bridgeErr) != "" {
		bres, bridgeErr = p.bridge.Launch(ctx, bridgeReq)
	}
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if bridgeErr != nil {
		// GAP 9 (self-healing): retro is the failure-analysis phase on the
		// audit-FAIL path. A non-nil error from Run propagates to RunCycle as a
		// hard abort that stops the WHOLE batch (the runs 154-162 abort mode) — a
		// failure in the failure-handler must never be fatal. Return a FAIL verdict
		// with NIL error so the orchestrator routes through decideAfterRetro
		// (failure-adapter: retry/block/proceed) instead of aborting. The bridge
		// error is preserved as a diagnostic for forensics; NextPhase is advisory
		// (decideAfterRetro picks the real successor from the verdict + history).
		fmt.Fprintf(os.Stderr, "[retro] WARN bridge failed (%v) — emitting FAIL verdict; orchestrator routes via failure-adapter (non-fatal)\n", bridgeErr)
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			NextPhase:    string(core.PhaseEnd),
			CostUSD:      bres.CostUSD,
			Tokens:       bres.Tokens,
			DurationMS:   durationMS,
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
		}, nil
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}
	verdict := core.VerdictPASS
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
	}, nil
}

func composePrompt(body string, req core.PhaseRequest, prev string) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- previous_verdict: %s\n", prev)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	return b.String()
}

// lessonsDirRel is where the retro persona is instructed to write lessons
// (agents/evolve-retrospective.md: "Output path:
// .evolve/instincts/lessons/inst-LXXX-<slug>.yaml"). Single-sourced here so the
// gate's search location and the persona's documented output path cannot drift —
// the drift that graded 220 of 238 retros FAIL for not producing an artifact the
// persona is instructed never to produce.
const lessonsDirRel = ".evolve/instincts/lessons"

// lessonPrefixForCycle is the persona's own id convention: inst-L<cycle><suffix>,
// e.g. inst-L1574a-<slug>.yaml. Matching on it keeps the check CYCLE-SCOPED —
// 600 lessons exist in that directory (135 inst-L*, 464 an older cycle-<N>-*
// convention), and a gate that accepted any of them would pass unconditionally,
// converting a permanently-failing gate into a rubber stamp.
func lessonPrefixForCycle(cycle int) string {
	return "inst-L" + strconv.Itoa(cycle)
}

// matchesCycleLesson requires a NON-DIGIT delimiter after the cycle number. A
// plain prefix match would let cycle 157's gate be satisfied by
// inst-L1574a-<slug>.yaml, and cycle 1's by any lesson whose number starts with 1
// — re-opening the rubber-stamp hole the cycle-scoping exists to close.
func matchesCycleLesson(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	c := name[len(prefix)]
	return c < '0' || c > '9'
}

// hasFailureLesson reports whether THIS cycle produced a failure lesson, looking
// where the persona actually writes them and, for the pre-2026-08 corpus, in the
// workspace as well.
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
	// Legacy: cycle-1571-era runs wrote failure-lesson*.yaml into the workspace.
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

// init self-registers the retro phase factory with the phase registry, like
// every other built-in phase. The subprocess dispatcher (internal/cli/phasecmd)
// resolves phases by name and never constructs retro directly — keeping the
// flow phase-agnostic (ADR-0035/0038). The factory builds the default
// project-rooted bridge + prompts loader (Model "auto"), matching the prior
// phasecmd wiring byte-for-byte (cmdutil.NewPromptsLoader was a duplicate of
// prompts.NewForProject).
func init() {
	registry.Register(string(core.PhaseRetro), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
			Model:   "auto",
		})
	})
}
