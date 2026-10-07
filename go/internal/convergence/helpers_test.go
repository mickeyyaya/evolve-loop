package convergence_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type mod func(*convergence.Finding)

func finding(id string, sev convergence.Severity, mods ...mod) convergence.Finding {
	f := convergence.Finding{ID: id, Severity: sev, Status: convergence.StatusOpen}
	for _, m := range mods {
		m(&f)
	}
	return f
}

func fixed(f *convergence.Finding)      { f.Status = convergence.StatusFixed }
func disputed(f *convergence.Finding)   { f.Status = convergence.StatusDisputed }
func late(f *convergence.Finding)       { f.Late = true }
func survived(f *convergence.Finding)   { f.Falsification = convergence.FalsificationSurvived }
func refuted(f *convergence.Finding)    { f.Falsification = convergence.FalsificationRefuted }
func capability(f *convergence.Finding) { f.Kind = convergence.KindCapability }

func at(location string) mod   { return func(f *convergence.Finding) { f.Location = location } }
func ofComponent(c string) mod { return func(f *convergence.Finding) { f.Component = c } }
func ofClass(class string) mod { return func(f *convergence.Finding) { f.Class = class } }
func withStatus(s convergence.Status) mod {
	return func(f *convergence.Finding) { f.Status = s }
}

func defaults() policy.ConvergenceConfig { return policy.Policy{}.ConvergenceConfig() }

func withBudget(n int) policy.ConvergenceConfig {
	c := defaults()
	c.MaxFixRounds = n
	return c
}

func judgments(rounds ...[]convergence.Finding) []convergence.Judgment {
	out := make([]convergence.Judgment, len(rounds))
	for i, findings := range rounds {
		out[i] = convergence.Judgment{Index: i, Findings: findings}
	}
	return out
}

func inputFor(loop convergence.Loop, rounds ...[]convergence.Finding) convergence.Input {
	return inputOf(loop, judgments(rounds...))
}

func inputOf(loop convergence.Loop, js []convergence.Judgment) convergence.Input {
	return convergence.Input{Loop: loop, Round: len(js) - 1, Rounds: js, Config: defaults()}
}

func decide(t *testing.T, in convergence.Input) convergence.Decision {
	t.Helper()
	if err := in.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return convergence.Decide(in)
}

func reasonFor(d convergence.Decision, code string) string {
	for _, r := range d.Reasons {
		if strings.HasPrefix(r, code+" ") || r == code {
			return r
		}
	}
	return ""
}

func requireReason(t *testing.T, d convergence.Decision, code string, fields ...string) {
	t.Helper()
	for _, r := range d.Reasons {
		if strings.HasPrefix(r, code+" ") && hasFields(r, fields) {
			return
		}
	}
	t.Fatalf("no %s reason with %q in %q", code, fields, d.Reasons)
}

func hasFields(reason string, fields []string) bool {
	for _, field := range fields {
		if !strings.Contains(reason+" ", " "+field+" ") {
			return false
		}
	}
	return true
}

func refuseReason(t *testing.T, d convergence.Decision, code string) {
	t.Helper()
	if r := reasonFor(d, code); r != "" {
		t.Fatalf("unexpected %s reason %q", code, r)
	}
}

func requireOutcome(t *testing.T, d convergence.Decision, action convergence.Action, rung int) {
	t.Helper()
	if d.Action != action || d.Rung != rung {
		t.Fatalf("decision = %s at rung %d, want %s at rung %d (reasons %q)", d.Action, d.Rung, action, rung, d.Reasons)
	}
}

func many(prefix string, n int, sev convergence.Severity, mods ...mod) []convergence.Finding {
	out := make([]convergence.Finding, n)
	for i := range out {
		out[i] = finding(prefix+string(rune('a'+i)), sev, mods...)
	}
	return out
}

func join(groups ...[]convergence.Finding) []convergence.Finding {
	var out []convergence.Finding
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func asFixed(findings []convergence.Finding) []convergence.Finding {
	out := make([]convergence.Finding, len(findings))
	for i, f := range findings {
		f.Status = convergence.StatusFixed
		out[i] = f
	}
	return out
}

var (
	deepJudge = convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "high"}
	topJudge  = convergence.TierEffort{Family: "claude-tmux", Tier: "top", Model: "opus", Effort: "xhigh"}

	todaysTables = convergence.HeadroomTable{"claude-tmux": {
		"fast": {Model: "haiku"}, "balanced": {Model: "sonnet"}, "deep": {Model: "opus"}, "top": {Model: "opus"},
	}}
	v3bTables = convergence.HeadroomTable{"claude-tmux": {
		"fast": {Model: "haiku", Effort: "low"}, "balanced": {Model: "sonnet", Effort: "medium"},
		"deep": {Model: "opus", Effort: "high"}, "top": {Model: "opus", Effort: "xhigh"},
	}}
)
