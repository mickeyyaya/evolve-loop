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
	return nil
}
