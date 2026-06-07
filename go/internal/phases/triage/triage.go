// Package triage implements the cycle-scope task-selection phase. The
// phase boilerplate lives in internal/phases/runner; this file only
// encodes triage-specific variation points.
//
// Skip semantics (Skipper interface):
//   - EVOLVE_TRIAGE_DISABLE=1 → SKIPPED, NextPhase=tdd, no bridge call
//
// Verdict mapping:
//   - empty artifact → FAIL
//   - missing "## top_n" heading → FAIL
//   - "## top_n" section with no list items → FAIL
//   - "## top_n" with ≥1 list item → PASS
package triage

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// topNHeadingRE locates the selection-section heading (phasecontract.Triage,
// single source).
var topNHeadingRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(phasecontract.Triage.Sections[0].Canonical) + `\b`)

// listItemRE matches a single non-empty Markdown list item line.
var listItemRE = regexp.MustCompile(`(?m)^[-*]\s+\S`)

// nextHeadingRE finds the next "## " section heading.
var nextHeadingRE = regexp.MustCompile(`(?m)^## `)

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseTriage) }
func (hooks) AgentPromptName() string                     { return "evolve-triage" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "triage-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

// ShouldSkip delegates the enable/skip decision to the central PhasePolicy
// (config.Load is the sole reader of EVOLVE_TRIAGE_DISABLE), instead of
// reading the env flag literal here. Legacy posture preserved: triage runs
// unless disabled.
func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseTriage)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseTDD), nil
}

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if s := req.Context["carryover_summary"]; s != "" {
		fmt.Fprintf(&b, "- carryover_summary: %s\n", s)
	}
	return b.String()
}

func (hooks) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	// EvaluateClassify handles the empty-artifact and section-presence checks.
	verdict, diags := specrunner.EvaluateClassify(artifact, &phasespec.ClassifyRules{
		RequireSections: []string{phasecontract.Triage.Sections[0].Canonical},
		FailIfEmpty:     true,
	})
	if verdict != core.VerdictPASS {
		return verdict, diags, string(core.PhaseTDD)
	}
	// Extra triage invariant: ## top_n must contain at least one list item.
	if !hasTopNItems(strings.TrimSpace(artifact)) {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "## top_n section has no list items",
		}}, string(core.PhaseTDD)
	}
	return core.VerdictPASS, nil, string(core.PhaseTDD)
}

func hasTopNItems(trimmed string) bool {
	loc := topNHeadingRE.FindStringIndex(trimmed)
	if loc == nil {
		return false
	}
	body := trimmed[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return listItemRE.MatchString(body)
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseTriage), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
