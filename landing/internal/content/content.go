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
// section: one example goal, and the phases the model generates for it.
type PipelineDemo struct {
	Kicker     string        `json:"kicker"`
	Heading    string        `json:"heading"`
	Sub        string        `json:"sub"`
	Task       string        `json:"task"`
	Candidates []PhaseChoice `json:"candidates"`
	Outcome    string        `json:"outcome"`
}

// PhaseChoice is one phase the model could run. Use marks the phases it selects
// for this goal; Skip gives the reason it leaves a phase out.
type PhaseChoice struct {
	Phase string `json:"phase"`
	Use   bool   `json:"use"`
	Skip  string `json:"skip,omitempty"`
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
		{"pillars (>=3)", len(s.Pillars) < 3},
		{"comparison.rows", len(s.Comparison.Rows) == 0},
		{"footer.links", len(s.Footer.Links) == 0},
	}
	for _, c := range checks {
		if c.missing {
			return fmt.Errorf("missing required content field: %s", c.name)
		}
	}
	return nil
}
