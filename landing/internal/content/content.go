// Package content is the typed, validated source of truth for the landing site.
// Every page is rendered from a single *Site loaded from shared/content.json, so
// facts and copy live in exactly one place and cannot drift across versions.
package content

import (
	"encoding/json"
	"fmt"
	"os"
)

// Site is the whole content model. Field order mirrors the page narrative arc.
type Site struct {
	Product      Product      `json:"product"`
	Hero         Hero         `json:"hero"`
	ProofBar     []Stat       `json:"proofBar"`
	Problem      Section      `json:"problem"`
	TheTurn      Section      `json:"theTurn"`
	PhaseSpine   []Phase      `json:"phaseSpine"`
	PipelineDemo PipelineDemo `json:"pipelineDemo"`
	PillarsIntro Section      `json:"pillarsIntro"`
	Pillars      []Pillar     `json:"pillars"`
	Incident     Incident     `json:"incident"`
	Comparison   Comparison   `json:"comparison"`
	Quickstart   Quickstart   `json:"quickstart"`
	Examples     Examples     `json:"examples"`
	Concurrency  Concurrency  `json:"concurrency"`
	TryIt        TryIt        `json:"tryIt"`
	InboxLab     InboxLab     `json:"inboxLab"`
	GateLab      GateLab      `json:"gateLab"`
	RecoveryLab  RecoveryLab  `json:"recoveryLab"`
	FinalCTA     FinalCTA     `json:"finalCta"`
	Footer       Footer       `json:"footer"`
}

type Product struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	MentalModel string `json:"mentalModel"`
	OneLiner    string `json:"oneLiner"`
	OneBreath   string `json:"oneBreath"`
	License     string `json:"license"`
	Repo        string `json:"repo"`
	RepoURL     string `json:"repoUrl"`
}

// CTA is a call to action — either a copyable command or a link.
type CTA struct {
	Label   string `json:"label"`
	Command string `json:"command,omitempty"`
	Href    string `json:"href,omitempty"`
}

// Verdict drives the signature hero animation (checks ticking → SHIP).
type Verdict struct {
	File    string   `json:"file"`
	Checks  []string `json:"checks"`
	Result  string   `json:"result"`
	Outcome string   `json:"outcome"`
}

type Hero struct {
	Headline         string   `json:"headline"`
	HeadlineLead     string   `json:"headlineLead"`     // headline up to the emphasized part
	HeadlineEmphasis string   `json:"headlineEmphasis"` // the part each layout renders as <em>
	HeadlineVariants []string `json:"headlineVariants"`
	Subhead          string   `json:"subhead"`
	CTAPrimary       CTA      `json:"ctaPrimary"`
	CTASecondary     CTA      `json:"ctaSecondary"`
	Verdict          Verdict  `json:"verdictAnimation"`
}

type Stat struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Section struct {
	Kicker  string   `json:"kicker"`
	Heading string   `json:"heading"`
	Body    []string `json:"body"`
}

type Phase struct {
	Phase    string `json:"phase"`
	Artifact string `json:"artifact"`
	Blurb    string `json:"blurb"`
}

// PipelineDemo powers the interactive "the model composes its own pipeline"
// section. Floor is the small set of phases policy always runs (the integrity
// invariant ship ⇒ build ∧ audit); Cases are the goals the demo loops through,
// each showing the pipeline the advisor composes for it.
type PipelineDemo struct {
	Kicker    string     `json:"kicker"`
	Heading   string     `json:"heading"`
	Sub       string     `json:"sub"`
	Floor     []string   `json:"floor"`
	Providers []string   `json:"providers"`
	Cases     []DemoCase `json:"cases"`
}

// DemoCase is one goal the advisor composes a pipeline for. Phases is the
// ordered list of every node shown for this case, in display order — common
// phases, mandated floor phases, and any the advisor mints for the goal.
type DemoCase struct {
	Goal   string        `json:"goal"`
	Scope  string        `json:"scope"`
	Phases []PhaseChoice `json:"phases"`
}

// PhaseChoice is one node in a case. Use marks the phases the advisor runs; Why
// gives the reason (a skip reason when Use is false). CLI is the LLM provider the
// advisor gave the phase to (Claude Code, Codex, Gemini) and Model the specific
// model — the advisor balances ownership across providers per policy. Mint marks
// a phase the advisor wrote on the spot because no common phase fit the goal.
type PhaseChoice struct {
	Phase string `json:"phase"`
	Use   bool   `json:"use"`
	Why   string `json:"why,omitempty"`
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
	Mint  bool   `json:"mint,omitempty"`
}

type Pillar struct {
	Title string `json:"title"`
	Claim string `json:"claim"`
	Proof string `json:"proof"`
}

type Incident struct {
	Kicker  string `json:"kicker"`
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type Comparison struct {
	Kicker  string     `json:"kicker"`
	Heading string     `json:"heading"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Note    string     `json:"note"`
}

type InstallStep struct {
	Step    string `json:"step"`
	Command string `json:"command"`
}

type Preset struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Blurb   string `json:"blurb"`
}

type Quickstart struct {
	Kicker  string        `json:"kicker"`
	Heading string        `json:"heading"`
	Install []InstallStep `json:"install"`
	Presets []Preset      `json:"presets"`
}

// Examples powers the "from one command to full control" usage ladder: a
// progressive list of commands, each with a behind-the-scenes note, plus a
// simple setup/configuration explainer.
type Examples struct {
	Kicker  string        `json:"kicker"`
	Heading string        `json:"heading"`
	Sub     string        `json:"sub"`
	Items   []ExampleItem `json:"items"`
	Setup   SetupInfo     `json:"setup"`
}

// ExampleItem is one rung of the usage ladder: a numbered command and what it
// does behind the scenes.
type ExampleItem struct {
	N       string `json:"n"`
	Label   string `json:"label"`
	Command string `json:"command"`
	Note    string `json:"note"`
}

// SetupInfo is the one-command setup explainer + the named presets it offers.
type SetupInfo struct {
	Heading string   `json:"heading"`
	Command string   `json:"command"`
	Note    string   `json:"note"`
	Presets []string `json:"presets"`
}

// Concurrency powers the animated "run several loops at once" section: a set of
// parallel lanes (one loop per feature, each on its own LLM and branch) plus a
// few support scenarios. It conveys the concept — isolated, parallel, serialized
// merge — not the internals.
type Concurrency struct {
	Kicker    string     `json:"kicker"`
	Heading   string     `json:"heading"`
	Sub       string     `json:"sub"`
	Lanes     []Lane     `json:"lanes"`
	Scenarios []Scenario `json:"scenarios"`
}

// Lane is one concurrent loop: a goal, the LLM it runs on, its branch, and the
// pipeline the advisor composed for it — each lane runs its OWN phases, so the
// lanes deliberately differ.
type Lane struct {
	Goal   string   `json:"goal"`
	CLI    string   `json:"cli"`
	Branch string   `json:"branch"`
	Phases []string `json:"phases"`
}

// Scenario is one short "here's when you'd use it" card.
type Scenario struct {
	Title string `json:"title"`
	Note  string `json:"note"`
}

// TryIt powers the conversion spine: the versioned install one-liner with a
// copy button, the AUTHENTIC post-paste terminal output (so first success is
// recognizable), the next two commands, and the curl|sh trust rail. Copy
// follows the 2026 dev-landing evidence: version-stamped install header,
// what-you-will-see output, view-the-script affordance, alternatives inline.
type TryIt struct {
	Kicker    string   `json:"kicker"`
	Heading   string   `json:"heading"`
	Sub       string   `json:"sub"`
	Command   string   `json:"command"`
	Platforms string   `json:"platforms"`
	Verified  string   `json:"verified"`
	Terminal  []string `json:"terminal"`
	NextSteps []CTA    `json:"nextSteps"`
	Trust     []Link   `json:"trust"`
	NoAccount string   `json:"noAccount"`
}

// InboxLab powers the interactive "feed the loop" section: visitors compose a
// weighted todo from real-item presets, inject it into a simulated
// .evolve/inbox queue, and run a triage cycle to watch weight-ordered pickup,
// the phase strip, and carryover aging — the daily control surface for
// continuous development, animated.
type InboxLab struct {
	Kicker  string        `json:"kicker"`
	Heading string        `json:"heading"`
	Sub     string        `json:"sub"`
	Note    string        `json:"note"`
	Presets []InboxPreset `json:"presets"`
	Seed    []InboxSeed   `json:"seed"`
}

// InboxPreset is one ready-to-inject todo. The fields mirror the real inbox
// JSON schema (id, weight, title, kind, fix) so the JSON the visitor sees in
// the composer is the JSON an operator would actually write.
type InboxPreset struct {
	Label  string  `json:"label"`
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
	Fix    string  `json:"fix"`
}

// InboxSeed is an item already waiting in the demo queue, so triage has
// competition to rank against.
type InboxSeed struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
}

// GateLab powers the integrity-floor playground: evidence toggles feed the ship
// predicate and a verdict lamp shows SHIP or BLOCKED with the failing gate
// named. Role semantics (which toggle is the build evidence, which waives tdd)
// flow through data attributes rendered from this model — client JS never
// hardcodes a check key.
type GateLab struct {
	Kicker    string         `json:"kicker"`
	Heading   string         `json:"heading"`
	Sub       string         `json:"sub"`
	Rule      string         `json:"rule"`
	Checks    []GateCheck    `json:"checks"`
	Scenarios []GateScenario `json:"scenarios"`
	Note      string         `json:"note"`
}

// GateCheck is one evidence toggle. Role wires it into the floor predicate:
// "build" | "audit" | "tdd" | "trivial" | "and" (an unconditioned conjunct).
type GateCheck struct {
	Key      string `json:"key"`
	Role     string `json:"role"`
	Label    string `json:"label"`
	Evidence string `json:"evidence"`
	On       bool   `json:"on"`
}

// GateScenario is one preset button: the named checks are switched on, all
// others off, and the lamp must land on Expect ("ship" or "blocked").
type GateScenario struct {
	Label  string   `json:"label"`
	On     []string `json:"on"`
	Expect string   `json:"expect"`
}

// RecoveryLab powers the failure-routing explorer: pick a real failure event
// and watch the failure adapter's verdict route it — retry, re-route to another
// provider, re-pin, reconcile — until the loop continues.
type RecoveryLab struct {
	Kicker  string          `json:"kicker"`
	Heading string          `json:"heading"`
	Sub     string          `json:"sub"`
	Events  []RecoveryEvent `json:"events"`
}

// RecoveryEvent is one failure and its routed recovery. Verdict uses the real
// failure-adapter vocabulary: PROCEED, RETRY, or BLOCK.
type RecoveryEvent struct {
	Label   string   `json:"label"`
	Code    string   `json:"code"`
	Verdict string   `json:"verdict"`
	Steps   []string `json:"steps"`
	Outcome string   `json:"outcome"`
}

type FinalCTA struct {
	Heading   string `json:"heading"`
	Subhead   string `json:"subhead"`
	Primary   CTA    `json:"primary"`
	Secondary CTA    `json:"secondary"`
}

type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type Footer struct {
	Links     []Link `json:"links"`
	Tagline   string `json:"tagline"`
	Grounding string `json:"grounding"`
}

// Load reads, parses, and validates the content file, failing loudly at each step.
func Load(path string) (*Site, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read content %q: %w", path, err)
	}
	var site Site
	if err := json.Unmarshal(data, &site); err != nil {
		return nil, fmt.Errorf("parse content %q: %w", path, err)
	}
	if err := site.Validate(); err != nil {
		return nil, fmt.Errorf("invalid content %q: %w", path, err)
	}
	return &site, nil
}

// Validate returns an error naming the first required field that is missing,
// so a bad content edit fails the build with an actionable message.
func (s *Site) Validate() error {
	checks := []struct {
		name    string
		missing bool
	}{
		{"product.name", s.Product.Name == ""},
		{"product.version", s.Product.Version == ""},
		{"hero.headline", s.Hero.Headline == ""},
		{"hero.subhead", s.Hero.Subhead == ""},
		{"hero.ctaPrimary.command", s.Hero.CTAPrimary.Command == ""},
		{"proofBar", len(s.ProofBar) == 0},
		{"phaseSpine (>=5)", len(s.PhaseSpine) < 5},
		{"pipelineDemo.heading", s.PipelineDemo.Heading == ""},
		{"pipelineDemo.floor (>=1)", len(s.PipelineDemo.Floor) == 0},
		{"pipelineDemo.providers (>=1)", len(s.PipelineDemo.Providers) == 0},
		{"pipelineDemo.cases (>=2)", len(s.PipelineDemo.Cases) < 2},
		{"examples.heading", s.Examples.Heading == ""},
		{"examples.items (>=5)", len(s.Examples.Items) < 5},
		{"concurrency.heading", s.Concurrency.Heading == ""},
		{"concurrency.lanes (>=2)", len(s.Concurrency.Lanes) < 2},
		{"tryIt.heading", s.TryIt.Heading == ""},
		{"tryIt.command", s.TryIt.Command == ""},
		{"tryIt.terminal (>=4)", len(s.TryIt.Terminal) < 4},
		{"tryIt.trust (>=2)", len(s.TryIt.Trust) < 2},
		{"tryIt.nextSteps (>=2)", len(s.TryIt.NextSteps) < 2},
		{"inboxLab.heading", s.InboxLab.Heading == ""},
		{"inboxLab.presets (>=3)", len(s.InboxLab.Presets) < 3},
		{"inboxLab.seed (>=2)", len(s.InboxLab.Seed) < 2},
		{"gateLab.heading", s.GateLab.Heading == ""},
		{"gateLab.rule", s.GateLab.Rule == ""},
		{"gateLab.checks (>=4)", len(s.GateLab.Checks) < 4},
		{"gateLab.scenarios (>=3)", len(s.GateLab.Scenarios) < 3},
		{"recoveryLab.heading", s.RecoveryLab.Heading == ""},
		{"recoveryLab.events (>=4)", len(s.RecoveryLab.Events) < 4},
		{"pillars (>=3)", len(s.Pillars) < 3},
		{"comparison.rows", len(s.Comparison.Rows) == 0},
		{"footer.links", len(s.Footer.Links) == 0},
	}
	for _, c := range checks {
		if c.missing {
			return fmt.Errorf("missing required content field: %s", c.name)
		}
	}
	// Cross-field coherence for the pipeline demo: a bad content.json should fail
	// the build here, not render a broken card. Every case needs phases, and every
	// phase the advisor runs must route to a declared provider.
	providers := make(map[string]bool, len(s.PipelineDemo.Providers))
	for _, p := range s.PipelineDemo.Providers {
		providers[p] = true
	}
	for i, c := range s.PipelineDemo.Cases {
		if len(c.Phases) == 0 {
			return fmt.Errorf("invalid content: pipelineDemo.cases[%d] (%q) has no phases", i, c.Goal)
		}
		for _, ph := range c.Phases {
			if ph.Use && !providers[ph.CLI] {
				return fmt.Errorf("invalid content: pipelineDemo.cases[%d] phase %q routes to %q, not in providers", i, ph.Phase, ph.CLI)
			}
		}
	}
	// Each concurrency lane composes its own pipeline.
	for i, l := range s.Concurrency.Lanes {
		if len(l.Phases) < 2 {
			return fmt.Errorf("invalid content: concurrency.lanes[%d] (%q) needs its own phases (>=2)", i, l.Goal)
		}
	}
	// Gate-lab coherence: every scenario key must name a declared check (a typo
	// would render a preset that silently does nothing), and expect must be one
	// of the two verdicts the lamp can show.
	gateKeys := make(map[string]bool, len(s.GateLab.Checks))
	for _, c := range s.GateLab.Checks {
		gateKeys[c.Key] = true
	}
	for i, sc := range s.GateLab.Scenarios {
		if sc.Expect != "ship" && sc.Expect != "blocked" {
			return fmt.Errorf("invalid content: gateLab.scenarios[%d] (%q) expect %q, want ship|blocked", i, sc.Label, sc.Expect)
		}
		for _, k := range sc.On {
			if !gateKeys[k] {
				return fmt.Errorf("invalid content: gateLab.scenarios[%d] (%q) references unknown check %q", i, sc.Label, k)
			}
		}
	}
	// Recovery events must use the real failure-adapter vocabulary and carry a
	// visible routing (>=2 steps) — a single-step "recovery" teaches nothing.
	for i, e := range s.RecoveryLab.Events {
		if e.Verdict != "PROCEED" && e.Verdict != "RETRY" && e.Verdict != "BLOCK" {
			return fmt.Errorf("invalid content: recoveryLab.events[%d] (%q) verdict %q, want PROCEED|RETRY|BLOCK", i, e.Label, e.Verdict)
		}
		if len(e.Steps) < 2 {
			return fmt.Errorf("invalid content: recoveryLab.events[%d] (%q) needs >= 2 steps", i, e.Label)
		}
	}
	return nil
}
