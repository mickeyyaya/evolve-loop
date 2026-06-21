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
