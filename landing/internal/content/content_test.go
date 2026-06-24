package content

import (
	"strings"
	"testing"
)

// The real source-of-truth file must always load and satisfy the page contract.
// This is the drift guard: if content.json loses a required field, the build fails loudly.
func TestLoad_RealContentFileIsValid(t *testing.T) {
	site, err := Load("../../shared/content.json")
	if err != nil {
		t.Fatalf("Load real content.json: %v", err)
	}
	if site.Product.Name != "Evolve Loop" {
		t.Errorf("Product.Name = %q, want Evolve Loop", site.Product.Name)
	}
	if site.Product.Version == "" {
		t.Error("Product.Version is empty")
	}
	if site.Hero.Headline == "" {
		t.Error("Hero.Headline is empty")
	}
	if site.Hero.CTAPrimary.Command == "" {
		t.Error("Hero.CTAPrimary.Command is empty")
	}
	if len(site.Pillars) < 3 {
		t.Errorf("len(Pillars) = %d, want >= 3", len(site.Pillars))
	}
	if len(site.PhaseSpine) < 5 {
		t.Errorf("len(PhaseSpine) = %d, want >= 5", len(site.PhaseSpine))
	}
	if len(site.Comparison.Rows) == 0 {
		t.Error("Comparison.Rows is empty")
	}
	if len(site.ProofBar) == 0 {
		t.Error("ProofBar is empty")
	}
}

// Validate must name the first missing required field so a content edit fails with a clear message.
func TestValidate_ReportsMissingHeadline(t *testing.T) {
	site := &Site{}
	site.Product.Name = "X"
	site.Product.Version = "v1"
	// Hero.Headline intentionally left empty.

	err := site.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error for the missing headline")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "headline") {
		t.Errorf("error %q does not mention 'headline'", err)
	}
}

// Section labels and the emphasized headline are CONTENT, not layout. Every
// layout binds them from here, so editing content.json updates all five versions
// at once — nothing copy-related stays hardcoded in a template.
func TestContent_AllCopyIsCentralized(t *testing.T) {
	site, err := Load("../../shared/content.json")
	if err != nil {
		t.Fatal(err)
	}
	if site.Hero.HeadlineLead == "" || site.Hero.HeadlineEmphasis == "" {
		t.Fatal("hero headline must be split into HeadlineLead + HeadlineEmphasis")
	}
	if got := site.Hero.HeadlineLead + site.Hero.HeadlineEmphasis; got != site.Hero.Headline {
		t.Errorf("HeadlineLead+HeadlineEmphasis = %q, want full Headline %q", got, site.Hero.Headline)
	}
	if site.PillarsIntro.Kicker == "" || site.PillarsIntro.Heading == "" {
		t.Error("PillarsIntro.Kicker/Heading missing (pillars section label is hardcoded somewhere)")
	}
	if site.Comparison.Kicker == "" {
		t.Error("Comparison.Kicker missing")
	}
	if site.Quickstart.Kicker == "" {
		t.Error("Quickstart.Kicker missing")
	}
}

// The live pipeline demo is content-driven: its scenarios (and the shape each
// one assembles) live in content.json so the interactive section stays in sync.
func TestContent_PipelineDemoLoads(t *testing.T) {
	site, err := Load("../../shared/content.json")
	if err != nil {
		t.Fatal(err)
	}
	d := site.PipelineDemo
	if d.Heading == "" {
		t.Error("PipelineDemo.Heading must be set")
	}
	if len(d.Floor) == 0 {
		t.Fatal("PipelineDemo.Floor must name the always-run phases")
	}
	if len(d.Cases) < 5 {
		t.Fatalf("PipelineDemo.Cases = %d, want >= 5 looping goals", len(d.Cases))
	}

	floor := make(map[string]bool, len(d.Floor))
	for _, f := range d.Floor {
		floor[f] = true
	}

	var mintedAnywhere int
	usedPerCase := make([]int, len(d.Cases))
	for ci, c := range d.Cases {
		if c.Goal == "" {
			t.Errorf("case %d must set a goal", ci)
		}
		if len(c.Phases) < 5 {
			t.Errorf("case %q has %d phases, want >= 5", c.Goal, len(c.Phases))
		}
		runFloor := make(map[string]bool)
		for _, p := range c.Phases {
			if p.Phase == "" {
				t.Errorf("case %q has a phase with no name", c.Goal)
			}
			if p.Use {
				usedPerCase[ci]++
				if p.CLI == "" || p.Model == "" {
					t.Errorf("case %q: run phase %q must name the LLM (cli) and model the advisor routed it to", c.Goal, p.Phase)
				}
				if floor[p.Phase] {
					runFloor[p.Phase] = true
				}
			} else if p.Why == "" {
				t.Errorf("case %q: skipped phase %q must give a reason", c.Goal, p.Phase)
			}
			if p.Mint {
				mintedAnywhere++
				if !p.Use {
					t.Errorf("case %q: minted phase %q should also run (use=true)", c.Goal, p.Phase)
				}
			}
		}
		// The integrity floor must run in every case — that is the whole point
		// of marking it a floor (ship ⇒ build ∧ audit).
		for _, f := range d.Floor {
			if !runFloor[f] {
				t.Errorf("case %q must run floor phase %q (the floor always runs)", c.Goal, f)
			}
		}
	}

	if mintedAnywhere == 0 {
		t.Error("at least one case should mint a custom phase (the advisor writes its own)")
	}
	// The demo earns its name only if the pipelines actually differ in weight.
	if len(usedPerCase) == 0 {
		t.Fatal("no cases to compare weights")
	}
	min, max := usedPerCase[0], usedPerCase[0]
	for _, n := range usedPerCase {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	if min == max {
		t.Errorf("cases should vary light↔heavy, but all run %d phases", min)
	}
}

func TestLoad_BadJSONFailsLoudly(t *testing.T) {
	if _, err := Load("testdata/malformed.json"); err == nil {
		t.Fatal("Load(malformed) returned nil error, want a parse error")
	}
}

// A file that is well-formed JSON but missing a required content field must be
// rejected by Load via Validate — not just by a direct Validate() call. This
// covers Load's validate-error branch and proves the error names the field and
// is wrapped under the "invalid content" prefix.
func TestLoad_ValidJSONButInvalidContentFailsLoudly(t *testing.T) {
	site, err := Load("testdata/missing-headline.json")
	if err == nil {
		t.Fatal("Load(missing-headline) returned nil error, want a validation error")
	}
	if site != nil {
		t.Errorf("Load returned a non-nil *Site (%v) on validation failure, want nil", site)
	}
	msg := err.Error()
	if !strings.Contains(msg, "hero.headline") {
		t.Errorf("error %q does not name the missing field 'hero.headline'", msg)
	}
	if !strings.Contains(msg, "invalid content") {
		t.Errorf("error %q is not wrapped with the Load validate-stage prefix 'invalid content'", msg)
	}
}

func TestLoad_MissingFileFailsLoudly(t *testing.T) {
	if _, err := Load("testdata/does-not-exist.json"); err == nil {
		t.Fatal("Load(missing) returned nil error, want a file error")
	}
}
