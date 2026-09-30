// Package specrunner turns a declarative phasespec.PhaseSpec into a runnable core.PhaseRunner with no per-phase Go.
// See docs/architecture/packages/internal-phases-specrunner.md.
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

type hooks struct {
	spec             phasespec.PhaseSpec
	inlinePromptBody string
}

func (h hooks) PhaseName() string       { return h.spec.Name }
func (h hooks) AgentPromptName() string { return h.spec.AgentName() }
func (h hooks) DefaultModel() string    { return h.spec.ModelOrDefault() }

func (h hooks) InlinePromptBody() (string, bool) { return h.inlinePromptBody, h.inlinePromptBody != "" }

func (h hooks) ArtifactFilename(_ core.PhaseRequest) string {
	if files := h.spec.Outputs.Files; len(files) > 0 && files[0] != "" {
		return filepath.Base(files[0])
	}
	return h.spec.Name + "-report.md"
}

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

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	o := evaluate(artifact, h.spec.Classify)
	rec, optedIn := shadowRecord(req.Cycle, h.spec.Name, sentinelStageOf(h.spec.Classify), o)
	if err := writeVerdictShadow(req.Workspace, rec, optedIn); err != nil {
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message:  "verdict_from_sentinel: shadow record not written: " + err.Error(),
		})
	}
	return o.effectiveVerdict, o.diags, h.spec.OnPass
}

func sentinelStageOf(rules *phasespec.ClassifyRules) string {
	if rules == nil {
		return SentinelStageOff
	}
	return rules.VerdictFromSentinel
}

func EvaluateClassify(artifact string, rules *phasespec.ClassifyRules) (string, []core.Diagnostic) {
	o := evaluate(artifact, rules)
	return o.effectiveVerdict, o.diags
}

type classifyOutcome struct {
	structuralVerdict string
	sentinelVerdict   string
	sentinelConsulted bool
	sentinelPresent   bool
	effectiveVerdict  string
	diags             []core.Diagnostic
}

func structuralOnly(verdict string, diags []core.Diagnostic) classifyOutcome {
	return classifyOutcome{structuralVerdict: verdict, effectiveVerdict: verdict, diags: diags}
}

func evaluate(artifact string, rules *phasespec.ClassifyRules) classifyOutcome {
	if strings.TrimSpace(artifact) == "" && (rules == nil || rules.FailIfEmpty) {
		return structuralOnly(core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "phase produced an empty artifact"}})
	}
	if rules == nil {
		return structuralOnly(core.VerdictPASS, nil)
	}

	var missing []string
	for _, section := range rules.RequireSections {
		if !hasSection(artifact, section) {
			missing = append(missing, section)
		}
	}
	if len(missing) > 0 {
		return structuralOnly(core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "artifact missing required section(s): " + strings.Join(missing, ", "),
		}})
	}

	if len(rules.FailIfSignal) > 0 {
		return structuralOnly(core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "fail_if_signal declared but Stage-3 signal bus not available — remove or defer this gate",
		}})
	}

	verdict := core.VerdictPASS
	if rules.VerdictOnPass != "" {
		if !core.IsVerdict(rules.VerdictOnPass) {
			return structuralOnly(core.VerdictFAIL, []core.Diagnostic{{
				Severity: "error",
				Message:  fmt.Sprintf("invalid verdict_on_pass %q: must be PASS/FAIL/WARN/SKIPPED", rules.VerdictOnPass),
			}})
		}
		verdict = rules.VerdictOnPass
	}
	return applySentinelStage(classifyOutcome{structuralVerdict: verdict, effectiveVerdict: verdict}, artifact, rules.VerdictFromSentinel)
}

func hasSection(artifact, section string) bool {
	want := stripHeadingMarker(section)
	for _, line := range strings.Split(artifact, "\n") {
		if strings.HasPrefix(stripHeadingMarker(line), want) {
			return true
		}
	}
	return false
}

func stripHeadingMarker(s string) string {
	s = strings.TrimSpace(s)
	hashes := 0
	for hashes < len(s) && s[hashes] == '#' {
		hashes++
	}
	isHeadingMarker := hashes > 0 && hashes < len(s) && (s[hashes] == ' ' || s[hashes] == '\t')
	if isHeadingMarker {
		return strings.TrimSpace(s[hashes:])
	}
	return s
}

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
	PromptBody       string
}

type Phase struct{ *runner.BaseRunner }

func New(spec phasespec.PhaseSpec, c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:            hooks{spec: spec, inlinePromptBody: c.PromptBody},
			Bridge:           c.Bridge,
			ContractVerifier: c.ContractVerifier,
			HostEffects:      c.HostEffects,
			Prompts:          c.Prompts,
			NowFn:            c.NowFn,
		}),
	}
}
