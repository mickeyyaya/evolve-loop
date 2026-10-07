package convergence_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestParse_Rule1_ReadsTheRoundsSchemaWithStableIDs(t *testing.T) {
	data := []byte(`{"loop": "console-lane", "round": 1,
	 "rounds": [
	  {"index": 0, "findings": [{"id": "H1", "severity": "HIGH", "kind": "defect", "class": "correctness",
	    "component": "bridge:model-check", "location": "go/internal/bridge/x.go:12", "status": "OPEN", "certificate": "TestX"}]},
	  {"index": 1, "findings": [{"id": "H1", "severity": "HIGH", "status": "FIXED"},
	    {"id": "L1", "severity": "LOW", "status": "OPEN", "late": true, "falsification": "refuted"}],
	   "fix_hunks": [{"file": "go/internal/bridge/x.go", "from": 10, "to": 20}],
	   "reentry": true, "fingerprint": "fp-1", "edge": "audit->build"}],
	 "components": {"bridge:model-check": {"separable": true, "fail_safe_certificate": "TestFailSafe"}},
	 "fixer": {"family": "claude-tmux", "tier": "deep", "model": "opus", "effort": "high"},
	 "judge": {"family": "claude-tmux", "tier": "deep"}}`)

	got, err := convergence.Parse(data)

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := convergence.Input{
		Loop: convergence.LoopConsoleLane, Round: 1,
		Rounds: []convergence.Judgment{
			{Index: 0, Findings: []convergence.Finding{{ID: "H1", Severity: convergence.SeverityHigh, Kind: convergence.KindDefect,
				Class: "correctness", Component: "bridge:model-check", Location: "go/internal/bridge/x.go:12",
				Status: convergence.StatusOpen, Certificate: "TestX"}}},
			{Index: 1, Findings: []convergence.Finding{
				{ID: "H1", Severity: convergence.SeverityHigh, Status: convergence.StatusFixed},
				{ID: "L1", Severity: convergence.SeverityLow, Status: convergence.StatusOpen, Late: true, Falsification: convergence.FalsificationRefuted}},
				FixHunks: []convergence.Hunk{{File: "go/internal/bridge/x.go", From: 10, To: 20}},
				Reentry:  true, Fingerprint: "fp-1", Edge: "audit->build"},
		},
		Components: map[string]convergence.ComponentFacts{"bridge:model-check": {Separable: true, FailSafeCertificate: "TestFailSafe"}},
		Fixer:      convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "high"},
		Judge:      convergence.TierEffort{Family: "claude-tmux", Tier: "deep"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse =\n%+v\nwant\n%+v", got, want)
	}
}

func TestParse_RefusesAnUnknownFieldOrMalformedJSON(t *testing.T) {
	for name, data := range map[string]string{
		"unknown finding field": `{"loop": "console-lane", "round": 0, "rounds": [{"index": 0, "findings": [{"id": "a", "severity": "HIGH", "status": "OPEN", "blocking": true}]}]}`,
		"unknown top field":     `{"loop": "console-lane", "round": 0, "rounds": [], "max_rounds": 3}`,
		"malformed":             `{"loop": "console-lane", "round": `,
		"two documents":         `{"loop": "console-lane", "round": 0, "rounds": []} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := convergence.Parse([]byte(data)); err == nil {
				t.Fatalf("Parse(%s) = nil error, want a refusal", data)
			}
		})
	}
}

func TestValidate_RefusesMalformedRoundsUnderTheirCause(t *testing.T) {
	good := func() convergence.Input {
		in := inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("a", convergence.SeverityHigh)})
		in.Rounds[0].FixHunks = []convergence.Hunk{{File: "x.go", From: 1, To: 2}}
		return in
	}
	for _, tc := range []struct {
		cause string
		edit  func(*convergence.Input)
	}{
		{"loop", func(in *convergence.Input) { in.Loop = "grind" }},
		{"no judgment", func(in *convergence.Input) { in.Rounds = nil; in.Round = -1 }},
		{"round", func(in *convergence.Input) { in.Round = 1 }},
		{"index", func(in *convergence.Input) { in.Rounds[0].Index = 3 }},
		{"id", func(in *convergence.Input) { in.Rounds[0].Findings[0].ID = "" }},
		{"severity", func(in *convergence.Input) { in.Rounds[0].Findings[0].Severity = "SEVERE" }},
		{"status", func(in *convergence.Input) { in.Rounds[0].Findings[0].Status = "WONTFIX" }},
		{"kind", func(in *convergence.Input) { in.Rounds[0].Findings[0].Kind = "feature" }},
		{"class", func(in *convergence.Input) { in.Rounds[0].Findings[0].Class = "vibes" }},
		{"falsification", func(in *convergence.Input) { in.Rounds[0].Findings[0].Falsification = "maybe" }},
		{"twice", func(in *convergence.Input) {
			in.Rounds[0].Findings = append(in.Rounds[0].Findings, in.Rounds[0].Findings[0])
		}},
		{"never DEFERRED", func(in *convergence.Input) {
			in.Rounds[0].Findings[0] = finding("c", convergence.SeverityCritical, withStatus(convergence.StatusDeferred))
		}},
		{"never FILED", func(in *convergence.Input) {
			in.Rounds[0].Findings[0] = finding("c", convergence.SeverityCritical, withStatus(convergence.StatusFiled))
		}},
		{"hunk", func(in *convergence.Input) { in.Rounds[0].FixHunks[0].From = 3 }},
		{"hunk", func(in *convergence.Input) { in.Rounds[0].FixHunks[0].From = 0 }},
		{"hunk", func(in *convergence.Input) { in.Rounds[0].FixHunks[0].File = "" }},
		{"config", func(in *convergence.Input) { in.Config = policy.ConvergenceConfig{} }},
		{"config", func(in *convergence.Input) { in.Config.RaisedBlockingBar = "LOUD" }},
		{"config", func(in *convergence.Input) { in.Config.MaxBackwardEdges = 0 }},
	} {
		t.Run(tc.cause, func(t *testing.T) {
			in := good()
			tc.edit(&in)

			err := in.Validate()

			if err == nil || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("Validate = %v, want an error naming %q", err, tc.cause)
			}
		})
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("the well-formed input: Validate = %v", err)
	}
}

func TestValidate_AcceptsEveryKnownWord(t *testing.T) {
	var findings []convergence.Finding
	for i, sev := range []convergence.Severity{convergence.SeverityCritical, convergence.SeverityHigh, convergence.SeverityMedium, convergence.SeverityLow, convergence.SeverityInfo} {
		findings = append(findings, finding(string(sev), sev, withStatus([]convergence.Status{
			convergence.StatusOpen, convergence.StatusFixed, convergence.StatusDeferred, convergence.StatusDisputed, convergence.StatusFiled}[i])))
	}
	for i, class := range []string{"correctness", "concurrency", "architecture", "format", "docs", "hygiene"} {
		findings = append(findings, finding("c"+class, convergence.SeverityLow, ofClass(class), []mod{capability, func(*convergence.Finding) {}}[i%2]))
	}
	findings = append(findings, finding("survived", convergence.SeverityHigh, late, survived), finding("refuted", convergence.SeverityHigh, late, refuted))
	for _, loop := range []convergence.Loop{convergence.LoopAuditRepair, convergence.LoopCodeReview, convergence.LoopExplanationReauthor,
		convergence.LoopConsoleLane, convergence.LoopCycle, convergence.LoopInboxItem, convergence.LoopShipRecovery} {
		in := inputFor(loop, findings)

		if err := in.Validate(); err != nil {
			t.Fatalf("Validate(%s) = %v, want every known word accepted", loop, err)
		}
	}
}

func TestDecide_Rule2_INFONeverBlocksAndIsFiledNeverDeferred(t *testing.T) {
	onlyInfo := decide(t, inputFor(convergence.LoopConsoleLane, many("i", 3, convergence.SeverityInfo)))

	requireOutcome(t, onlyInfo, convergence.ActionLand, 0)
	if !reflect.DeepEqual(onlyInfo.File, []string{"ia", "ib", "ic"}) || len(onlyInfo.Defer) != 0 {
		t.Fatalf("file = %q defer = %q, want every INFO filed and none deferred", onlyInfo.File, onlyInfo.Defer)
	}

	in := inputFor(convergence.LoopConsoleLane,
		many("h0", 2, convergence.SeverityHigh),
		join(asFixed(many("h0", 2, convergence.SeverityHigh)), []convergence.Finding{finding("h1", convergence.SeverityHigh)}),
		[]convergence.Finding{finding("h1", convergence.SeverityHigh, fixed), finding("h2", convergence.SeverityHigh),
			finding("m2", convergence.SeverityMedium), finding("i2", convergence.SeverityInfo)})
	atRung2 := decide(t, in)

	requireOutcome(t, atRung2, convergence.ActionContinue, 2)
	if !reflect.DeepEqual(atRung2.Defer, []string{"m2"}) || !reflect.DeepEqual(atRung2.File, []string{"i2"}) {
		t.Fatalf("defer = %q file = %q, want the MEDIUM deferred and the INFO filed", atRung2.Defer, atRung2.File)
	}
}

func TestDecide_Rule2_INFOIsNeverRepairDamage(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("p", convergence.SeverityHigh)},
		join([]convergence.Finding{finding("h", convergence.SeverityHigh, fixed), finding("p", convergence.SeverityHigh)},
			many("i", 3, convergence.SeverityInfo, at("go/x/x.go:5"))))
	js[1].FixHunks = []convergence.Hunk{{File: "go/x/x.go", From: 1, To: 9}}
	in := inputOf(convergence.LoopConsoleLane, js)

	d := decide(t, in)

	refuseReason(t, d, "CONVERGENCE_REPAIR_DAMAGE")
	requireOutcome(t, d, convergence.ActionContinue, 1)
}

func TestDecide_Rule3_TheComponentFallsBackToTheLocationsDirectory(t *testing.T) {
	for _, tc := range []struct{ location, component, want string }{
		{"go/internal/bridge/launch.go:12", "", "go/internal/bridge"},
		{"go/internal/bridge/launch.go:12", "bridge:model-check", "bridge:model-check"},
		{"go/internal/bridge/launch.go:L12", "", "go/internal/bridge"},
		{"go/acs/cycle1821/predicate_001_test.go:3", "", "go/acs/cycle1821"},
		{"go/acs/cycle1821/deep/x_test.go:3", "", "go/acs/cycle1821"},
		{"skills/loop/references/x.md:9", "", "skills/loop"},
		{"docs/architecture/convergence-policy.md:40", "", "docs/architecture"},
		{".evolve/profiles/builder.json", "", ".evolve/profiles"},
		{"agents/evolve-builder.md:2", "", "agents/evolve-builder.md"},
		{"CLAUDE.md:65", "", "CLAUDE.md"},
		{"legacy/scripts/release/poll.sh:7", "", "legacy/scripts/release"},
	} {
		t.Run(tc.location+"|"+tc.component, func(t *testing.T) {
			mods := []mod{at(tc.location), ofComponent(tc.component)}
			in := inputFor(convergence.LoopConsoleLane,
				many("a", 5, convergence.SeverityLow, mods...), many("b", 5, convergence.SeverityLow, mods...), many("c", 5, convergence.SeverityLow, mods...))

			d := decide(t, in)

			requireReason(t, d, "CONVERGENCE_CONCENTRATION", "component="+tc.want)
		})
	}
}

func TestDecide_Rule4_AFindingWithNoLocationCountsTowardMassButNotConcentration(t *testing.T) {
	noLocation := inputFor(convergence.LoopConsoleLane,
		many("a", 5, convergence.SeverityLow), many("b", 5, convergence.SeverityLow), many("c", 5, convergence.SeverityLow))

	refuseReason(t, decide(t, noLocation), "CONVERGENCE_CONCENTRATION")

	grows := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("h", convergence.SeverityHigh)},
		[]convergence.Finding{finding("h", convergence.SeverityHigh), finding("g", convergence.SeverityHigh)})

	requireReason(t, decide(t, grows), "CONVERGENCE_NO_PROGRESS", "mass_prev=4", "mass=8", "bar=MEDIUM")
}

func TestDecide_Rule7_ABarRaiseDoesNotFakeProgress(t *testing.T) {
	deferredMediums := many("m2", 5, convergence.SeverityMedium, withStatus(convergence.StatusDeferred))
	in := inputFor(convergence.LoopConsoleLane,
		many("h0", 2, convergence.SeverityHigh),
		join(asFixed(many("h0", 2, convergence.SeverityHigh)), []convergence.Finding{finding("h1", convergence.SeverityHigh)}),
		join([]convergence.Finding{finding("h1", convergence.SeverityHigh, fixed)},
			many("h2", 2, convergence.SeverityHigh), many("m2", 5, convergence.SeverityMedium)),
		join(asFixed(many("h2", 2, convergence.SeverityHigh)), deferredMediums, many("h3", 3, convergence.SeverityHigh)))

	d := decide(t, in)

	requireReason(t, d, "CONVERGENCE_NO_PROGRESS", "mass_prev=8", "mass=12", "bar=HIGH")
}

func TestDecide_Rule8_TheRungsFollowMaxFixRounds(t *testing.T) {
	for _, tc := range []struct {
		budget int
		rungs  []int
	}{
		{1, []int{2, 3, 3}},
		{2, []int{0, 2, 3, 3}},
		{3, []int{0, 1, 2, 3, 3}},
		{4, []int{0, 1, 1, 2, 3}},
		{5, []int{0, 1, 1, 1, 2, 3}},
	} {
		for r, want := range tc.rungs {
			in := inputOf(convergence.LoopConsoleLane, steadyRounds(r))
			in.Config = withBudget(tc.budget)

			d := decide(t, in)

			if d.Rung != want {
				t.Fatalf("N=%d after round %d: rung %d, want %d (reasons %q)", tc.budget, r, d.Rung, want, d.Reasons)
			}
		}
	}
}

func steadyRounds(r int) []convergence.Judgment {
	all := many("s", 2*r+2, convergence.SeverityHigh)
	rounds := make([][]convergence.Finding, r+1)
	for k := range rounds {
		rounds[k] = join(asFixed(all[:2*k]), all[2*k:])
	}
	return judgments(rounds...)
}

func TestDecide_Rule9_AReentryRoundSkipsTheMarginalGainTestButNotTheBudget(t *testing.T) {
	grown := judgments(
		[]convergence.Finding{finding("h", convergence.SeverityHigh)},
		[]convergence.Finding{finding("h", convergence.SeverityHigh, fixed), finding("g", convergence.SeverityHigh), finding("k", convergence.SeverityHigh)})
	grown[1].Reentry = true
	unrepaired := judgments(
		[]convergence.Finding{finding("a", convergence.SeverityMedium)},
		[]convergence.Finding{finding("a", convergence.SeverityMedium, fixed)},
		[]convergence.Finding{finding("a", convergence.SeverityMedium, fixed), finding("b", convergence.SeverityHigh), finding("c", convergence.SeverityMedium)})
	unrepaired[2].Reentry = true
	for _, js := range [][]convergence.Judgment{grown, unrepaired} {
		in := inputOf(convergence.LoopCodeReview, js)
		in.Config = withBudget(4)

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, 1)
		refuseReason(t, d, "CONVERGENCE_NO_PROGRESS")
		refuseReason(t, d, "CONVERGENCE_REPAIR_DAMAGE")
	}

	budget := inputOf(convergence.LoopCodeReview, steadyRounds(3))
	budget.Rounds[2].Reentry = true

	requireOutcome(t, decide(t, budget), convergence.ActionStop, 3)
}

func TestDecide_Rule10_EveryBarThePolicyAcceptsIsASeverity(t *testing.T) {
	for _, word := range []string{"CRITICAL", "HIGH", "MEDIUM"} {
		p := policy.Policy{Workflow: &policy.WorkflowPolicy{Convergence: &policy.ConvergencePolicy{BaseBlockingBar: word, RaisedBlockingBar: word}}}
		cfg := p.ConvergenceConfig()
		if len(cfg.Warnings) != 0 {
			t.Fatalf("policy warned on %s: %q", word, cfg.Warnings)
		}
		in := inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("c", convergence.SeverityCritical)})
		in.Config = cfg

		d := decide(t, in)

		if d.BlockingBar != convergence.Severity(word) {
			t.Fatalf("bar %s: decision bar %s", word, d.BlockingBar)
		}
	}
}
