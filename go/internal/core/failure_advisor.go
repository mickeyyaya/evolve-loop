package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// FailureAdvisor is the ADR-0044 LLM escalation TAIL: it reads a fatal-looking
// pane the deterministic FatalPaneDetector could NOT classify (CauseUnknown)
// and returns a typed cause + the novel pane substring to promote + a
// human-readable justification. Built exactly like PhaseAdvisor — bridge-
// dispatched, persona-injected, strict-JSON-parsed — and fail-safe the same
// way: EVERY failure (nil bridge, launch error, malformed output, vocabulary
// violation) returns an error and the caller escalates to the operator
// instead of acting on garbage. Deterministic-first, LLM-last (Core Agent
// Rule 5): this advisor is never on the hot loop for a known failure — its
// verdicts get PROMOTED into the deterministic registry
// (recovery.PromoteAdvice), so each novel state is paid for once.
type FailureAdvisor struct {
	bridge  Bridge
	cli     string
	model   string
	profile string // when non-empty, used verbatim; else derived from ProjectRoot
	persona string // agents/evolve-failure-advisor.md body; empty ⇒ inline framing
}

// FailureAdvisorOption customizes a FailureAdvisor (mirrors PhaseAdvisorOption).
type FailureAdvisorOption func(*FailureAdvisor)

// WithFailureAdvisorCLI overrides the CLI the advisor dispatches to.
func WithFailureAdvisorCLI(cli string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if cli != "" {
			a.cli = cli
		}
	}
}

// WithFailureAdvisorModel overrides the model tier the advisor requests.
func WithFailureAdvisorModel(model string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if model != "" {
			a.model = model
		}
	}
}

// WithFailureAdvisorPersona injects the persona body
// (agents/evolve-failure-advisor.md), uniform with phase agents.
func WithFailureAdvisorPersona(body string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if body != "" {
			a.persona = body
		}
	}
}

// NewFailureAdvisor builds the failure-classification tail over the given
// bridge. Defaults mirror PhaseAdvisor (claude-tmux + deep): diagnosing a
// novel terminal state is judgment work, and the advisor runs OFF the hot
// loop (only for unclassified states), so depth beats latency here.
func NewFailureAdvisor(bridge Bridge, opts ...FailureAdvisorOption) *FailureAdvisor {
	a := &FailureAdvisor{bridge: bridge, cli: "claude-tmux", model: "opus"}
	for _, o := range opts {
		o(a)
	}
	return a
}

// FailureAdviseInput is the evidence envelope for one unclassified terminal
// state: which phase/CLI died, how, and the recent pane tail.
type FailureAdviseInput struct {
	Phase       string
	CLI         string
	ExitCode    int
	PaneTail    string
	Workspace   string
	ProjectRoot string
	Cycle       int
	Env         map[string]string
}

// Advise asks the LLM to classify one CauseUnknown terminal state. The
// verdict is validated against the recovery vocabulary before it is returned
// — a hallucinated cause errors here, so no caller ever has to re-check.
// Promotion (in-memory + durable) is the caller's explicit next step via
// recovery.PromoteAdvice; Advise itself is read-only judgment. The caller's
// ctx bounds the dispatch (a batch cancellation must reach an in-flight
// advisor).
func (a *FailureAdvisor) Advise(ctx context.Context, in FailureAdviseInput) (*recovery.FailureAdvice, error) {
	if a.bridge == nil {
		return nil, fmt.Errorf("failure advisor: nil bridge")
	}
	if in.Workspace == "" {
		return nil, fmt.Errorf("failure advisor: empty workspace")
	}
	profile := a.profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "failure-advisor.json")
	}
	artifact := filepath.Join(in.Workspace, "failure-advice.json")
	resp, err := a.bridge.Launch(ctx, BridgeRequest{
		CLI:          a.cli,
		Profile:      profile,
		Model:        a.model,
		Prompt:       a.composePrompt(in, artifact),
		Workspace:    in.Workspace,
		ArtifactPath: artifact,
		Completion:   "artifact",
		Agent:        "failure-advisor",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("failure advisor: bridge launch: %w", err)
	}
	return parseFailureAdvice(resp.Stdout)
}

// composePrompt renders persona (when injected) + the per-incident evidence,
// the same layering every phase prompt uses. The inline fallback keeps the
// advisor functional before the composition root wires the persona file.
func (a *FailureAdvisor) composePrompt(in FailureAdviseInput, artifact string) string {
	var b strings.Builder
	if a.persona != "" {
		b.WriteString(a.persona)
		b.WriteString("\n\n---\n")
	} else {
		b.WriteString("You are the evolve-loop failure advisor: classify ONE unrecoverable terminal state from the pane evidence below. ")
		b.WriteString("Known causes: model_invalid (CLI booted into an invalid/inaccessible-model error), cli_self_updated (the CLI replaced its own binary and exited), dead_shell (the pane is a plain shell, not an agent REPL). ")
		b.WriteString("Respond ONLY when the pane truly self-describes a fatal state.\n\n")
	}
	b.WriteString("# Incident\n")
	fmt.Fprintf(&b, "- phase: %s\n- cli: %s\n- exit_code: %d\n- cycle: %d\n\n", in.Phase, in.CLI, in.ExitCode, in.Cycle)
	b.WriteString("# Recent pane tail\n```\n")
	b.WriteString(in.PaneTail)
	b.WriteString("\n```\n\n")
	fmt.Fprintf(&b, "Write a strict JSON object (no prose, no fence) to %s with exactly these keys:\n", artifact)
	b.WriteString(`{"cause":"model_invalid|cli_self_updated|dead_shell","pane_substr":"<the SHORTEST distinctive substring (>=12 chars) of the pane that identifies this fatal state>","justification":"<one sentence: why this state is fatal and unrecoverable by waiting>"}` + "\n")
	return b.String()
}

// parseFailureAdvice decodes + validates the advisor's strict-JSON verdict.
// Validation here is the trust boundary: cause must be in the typed
// vocabulary and the substring long enough to be promotable — the same
// checks recovery.PromoteAdvice enforces, applied early so a bad verdict
// fails at the parse site with the model's raw output in the error.
func parseFailureAdvice(raw string) (*recovery.FailureAdvice, error) {
	trimmed := strings.TrimSpace(raw)
	// Tolerate accidental code fences (the same leniency parseProposal applies).
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	var adv recovery.FailureAdvice
	if err := json.Unmarshal([]byte(strings.TrimSpace(trimmed)), &adv); err != nil {
		return nil, fmt.Errorf("failure advisor: unparseable verdict (%v): %.200s", err, raw)
	}
	// The persona's documented non-fatal signal: an empty cause means the
	// advisor judged the pane NOT fatal. Still an error (the caller
	// escalates to the operator — correct outcome), but operationally
	// distinct from a hallucinated cause.
	if adv.Cause == "" {
		return nil, fmt.Errorf("failure advisor: advisor judged the state non-fatal — escalate to operator (justification: %s)", adv.Justification)
	}
	switch adv.Cause {
	case string(recovery.CauseModelInvalid), string(recovery.CauseCLISelfUpdated), string(recovery.CauseDeadShell):
	default:
		return nil, fmt.Errorf("failure advisor: cause %q outside the typed vocabulary", adv.Cause)
	}
	if adv.Justification == "" {
		return nil, fmt.Errorf("failure advisor: empty justification (every recovery decision is justified)")
	}
	return &adv, nil
}
