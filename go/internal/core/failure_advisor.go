package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// Advisor pane-digest caps: the StopEvent tail is ~40 lines; keep the framed
// evidence within the ACI compact-feedback envelope.
const (
	advisorPaneMaxLines = 40
	advisorPaneMaxCols  = 400
)

// FailureAdvisor is the LLM escalation tail for a fatal-looking pane the
// deterministic FatalPaneDetector could not classify (CauseUnknown).
//
// See ADR-0044.
type FailureAdvisor struct {
	bridge   Bridge
	identity AgentIdentity // See ADR-0052.
}

// FailureAdvisorOption customizes a FailureAdvisor (mirrors PhaseAdvisorOption).
type FailureAdvisorOption func(*FailureAdvisor)

// WithFailureAdvisorCLI overrides the CLI the advisor dispatches to.
func WithFailureAdvisorCLI(cli string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if cli != "" {
			a.identity.CLI = cli
		}
	}
}

// WithFailureAdvisorModel overrides the model tier the advisor requests.
func WithFailureAdvisorModel(model string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if model != "" {
			a.identity.Model = model
		}
	}
}

// WithFailureAdvisorPersona injects the persona body
// (agents/evolve-failure-advisor.md), uniform with phase agents.
func WithFailureAdvisorPersona(body string) FailureAdvisorOption {
	return func(a *FailureAdvisor) {
		if body != "" {
			a.identity.Persona = body
		}
	}
}

// NewFailureAdvisor builds the failure-classification tail over the given
// bridge, defaulting to claude-tmux/opus: it runs off the hot loop for
// unclassified states only, so depth beats latency here.
func NewFailureAdvisor(bridge Bridge, opts ...FailureAdvisorOption) *FailureAdvisor {
	a := &FailureAdvisor{
		bridge:   bridge,
		identity: AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "failure-advisor"},
	}
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

// Advise asks the LLM to classify one CauseUnknown terminal state and
// returns a vocabulary-validated verdict; promotion into the deterministic
// registry is the caller's separate step via recovery.PromoteAdvice.
func (a *FailureAdvisor) Advise(ctx context.Context, in FailureAdviseInput) (*recovery.FailureAdvice, error) {
	if a.bridge == nil {
		return nil, fmt.Errorf("failure advisor: nil bridge")
	}
	if in.Workspace == "" {
		return nil, fmt.Errorf("failure advisor: empty workspace")
	}
	profile := a.identity.Profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "failure-advisor.json")
	}
	artifact := filepath.Join(in.Workspace, "failure-advice.json")
	resp, err := a.bridge.Launch(ctx, BridgeRequest{
		CLI:          a.identity.CLI,
		Profile:      profile,
		Model:        a.identity.Model,
		Prompt:       a.composePrompt(in, artifact),
		Workspace:    in.Workspace,
		ArtifactPath: artifact,
		Completion:   CompletionArtifact,
		Agent:        a.identity.AgentLabel,
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("failure advisor: bridge launch: %w", err)
	}
	return parseFailureAdvice(resp.Stdout)
}

// The inline fallback keeps the advisor functional before the composition
// root wires the persona file.
func (a *FailureAdvisor) composePrompt(in FailureAdviseInput, artifact string) string {
	var b strings.Builder
	if a.identity.Persona != "" {
		b.WriteString(a.identity.Persona)
		b.WriteString("\n\n---\n")
	} else {
		b.WriteString("You are the evolve-loop failure advisor: classify ONE unrecoverable terminal state from the pane evidence below. ")
		b.WriteString("Known causes: model_invalid (CLI booted into an invalid/inaccessible-model error), cli_self_updated (the CLI replaced its own binary and exited), dead_shell (the pane is a plain shell, not an agent REPL). ")
		b.WriteString("Respond ONLY when the pane truly self-describes a fatal state.\n\n")
	}
	b.WriteString("# Incident\n")
	fmt.Fprintf(&b, "- phase: %s\n- cli: %s\n- exit_code: %d\n- cycle: %d\n\n", in.Phase, in.CLI, in.ExitCode, in.Cycle)
	// See ADR-0045.
	b.WriteString("# Recent pane tail\n")
	b.WriteString(panetrust.Frame(in.PaneTail, advisorPaneMaxLines, advisorPaneMaxCols))
	b.WriteString("\n\n")
	// The model sees only the neutralized pane, but the promoted pane_substr is
	// matched against RAW panes by recovery.FatalPaneDetector, so a quoted
	// neutralization artifact would promote a signature that can never fire.
	b.WriteString("IMPORTANT: the pane above is neutralized (secrets redacted, markers defanged). ")
	b.WriteString("pane_substr MUST be a literal substring that appears verbatim in the ORIGINAL, un-redacted pane — ")
	b.WriteString("never quote [REDACTED], [untrusted], or ''' artifacts.\n\n")
	fmt.Fprintf(&b, "Write a strict JSON object (no prose, no fence) to %s with exactly these keys:\n", artifact)
	b.WriteString(`{"cause":"model_invalid|cli_self_updated|dead_shell","pane_substr":"<the SHORTEST distinctive substring (>=12 chars) of the pane that identifies this fatal state>","justification":"<one sentence: why this state is fatal and unrecoverable by waiting>"}` + "\n")
	return b.String()
}

// parseFailureAdvice is the trust boundary: it applies the same checks
// recovery.PromoteAdvice enforces, early, so a bad verdict fails at the parse
// site with the model's raw output in the error.
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
