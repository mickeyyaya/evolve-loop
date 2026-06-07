// Package specrunner turns a declarative phasespec.PhaseSpec into a runnable
// Lego brick — a core.PhaseRunner — with ZERO per-phase Go. It is the generic
// counterpart to the hand-written phase packages (scout, build, …): instead of
// a bespoke Hooks impl, it derives every variation point from spec fields.
//
// This is what makes a user phase "pure data": drop a phase.json (the spec) +
// agent.md (the prompt) + profile.json (permissions), and specrunner supplies
// the artifact name, prompt composition, and verdict classification the
// BaseRunner template needs.
//
// Stage 1 scope: specrunner is a constructor only — it does NOT self-register
// into the phase registry (the composition root wires spec-derived factories in
// Stage 2), so adding this package changes no live behavior.
package specrunner

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// hooks is the spec-driven runner.Hooks implementation. It holds the spec by
// value (specs are small, immutable data).
type hooks struct {
	spec phasespec.PhaseSpec
	// promptBody is an optional in-band prompt. When non-empty, the
	// BaseRunner uses it instead of reading agents/<AgentName>.md — the
	// path that lets a minted phase ship its persona as pure data.
	promptBody string
}

func (h hooks) PhaseName() string       { return h.spec.Name }
func (h hooks) AgentPromptName() string { return h.spec.AgentName() }
func (h hooks) DefaultModel() string    { return h.spec.ModelOrDefault() }

// InlinePromptBody satisfies runner.InlinePromptProvider. ok is true only
// when an in-band body was supplied; otherwise BaseRunner falls back to the
// on-disk agent doc (byte-identical to the legacy path).
func (h hooks) InlinePromptBody() (string, bool) { return h.promptBody, h.promptBody != "" }

// ArtifactFilename returns the basename of the first declared output file, or
// the conventional "<name>-report.md" when none is declared. The registry may
// store full templated paths (".evolve/runs/cycle-{cycle}/scout-report.md");
// BaseRunner joins the basename with req.Workspace, so we strip the directory.
func (h hooks) ArtifactFilename(_ core.PhaseRequest) string {
	if files := h.spec.Outputs.Files; len(files) > 0 && files[0] != "" {
		return filepath.Base(files[0])
	}
	return h.spec.Name + "-report.md"
}

// ComposePrompt appends a "## Cycle Context" block to the agent body, mirroring
// the hand-written phases. The keys listed in spec.prompt_context are pulled
// from req.Context (only non-empty values are emitted). req.UpstreamSignals is
// intentionally NOT injected here yet — declared-signal injection arrives with
// the Stage 3 signal bus.
func (h hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	for _, key := range h.spec.PromptContext {
		if v := req.Context[key]; v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", key, v)
		}
	}
	return b.String()
}

// Classify evaluates the spec's declarative ClassifyRules against the artifact.
// The next-phase hint comes from spec.OnPass (empty lets the orchestrator's
// state machine pick the successor — Stage 1 behavior).
func (h hooks) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, diags := EvaluateClassify(artifact, h.spec.Classify)
	return verdict, diags, h.spec.OnPass
}

// EvaluateClassify is the declarative verdict evaluator shared by specrunner and
// built-in phases. Pure function (no I/O) so it is exhaustively unit-testable.
//
//   - empty artifact → FAIL when rules are absent or rules.FailIfEmpty is set
//     (rules present with FailIfEmpty unset → an empty artifact is allowed to
//     pass; the operator opted out explicitly)
//   - every require_sections header must be present as a line-anchored markdown
//     header, else FAIL
//   - fail_if_signal is parsed but NOT evaluated here — it needs the Stage 3
//     signal bus. A non-empty fail_if_signal emits a loud WARN diagnostic
//     rather than being silently ignored.
//   - rules.VerdictOnPass overrides the pass verdict, but must be a canonical
//     verdict (guards against silent typos in user phase JSON); else FAIL
func EvaluateClassify(artifact string, rules *phasespec.ClassifyRules) (string, []core.Diagnostic) {
	if strings.TrimSpace(artifact) == "" && (rules == nil || rules.FailIfEmpty) {
		return core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "phase produced an empty artifact"}}
	}
	if rules == nil {
		return core.VerdictPASS, nil
	}

	var missing []string
	for _, section := range rules.RequireSections {
		if !hasSection(artifact, section) {
			missing = append(missing, section)
		}
	}
	if len(missing) > 0 {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "artifact missing required section(s): " + strings.Join(missing, ", "),
		}}
	}

	var diags []core.Diagnostic
	if len(rules.FailIfSignal) > 0 {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "fail_if_signal declared but Stage-3 signal bus not available — remove or defer this gate",
		}}
	}

	verdict := core.VerdictPASS
	if rules.VerdictOnPass != "" {
		if !core.IsVerdict(rules.VerdictOnPass) {
			return core.VerdictFAIL, append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("invalid verdict_on_pass %q: must be PASS/FAIL/WARN/SKIPPED", rules.VerdictOnPass),
			})
		}
		verdict = rules.VerdictOnPass
	}
	return verdict, diags
}

// hasSection reports whether section appears as a line-anchored markdown header
// (the section text begins a line, ignoring leading whitespace). This matches
// the line-anchored convention of the hand-written phases rather than a loose
// substring match that could fire on a mid-line occurrence.
func hasSection(artifact, section string) bool {
	for _, line := range strings.Split(artifact, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), section) {
			return true
		}
	}
	return false
}

// Config carries the runner dependencies, matching the hand-written phases.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// PromptBody, when non-empty, is forwarded as the inline prompt body;
	// empty (the default) loads agents/<AgentName>.md from disk — see the
	// hooks.promptBody field for the full contract.
	PromptBody string
}

// Phase is a spec-driven core.PhaseRunner.
type Phase struct{ *runner.BaseRunner }

// New constructs a spec-driven phase from a PhaseSpec.
func New(spec phasespec.PhaseSpec, c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{spec: spec, promptBody: c.PromptBody},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
		}),
	}
}
