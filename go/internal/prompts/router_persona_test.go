package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func routerContent(t *testing.T) (raw []byte, body string) {
	t.Helper()
	root := repoRoot(t)
	p := filepath.Join(root, "agents", "evolve-router.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return raw, string(raw)
}

// routerProseBytes measures the body above the catalog heading minus the generated goal-recipes
// table, which is a registry projection that grows with the catalog, not prose TSC governs.
func routerProseBytes(t *testing.T, body string) int {
	t.Helper()
	const fmDelim = "---\n"
	const catalogHeading = "## Phase Catalog — Core Values"
	const genBegin = "<!-- GENERATED:goal-recipes BEGIN"
	const genEnd = "<!-- GENERATED:goal-recipes END -->"
	// Skip the opening "---\n" at position 0, find the closing "---\n".
	closingFM := strings.Index(body[3:], fmDelim)
	if closingFM < 0 {
		t.Fatalf("could not find closing YAML frontmatter delimiter in evolve-router.md")
	}
	bodyStart := 3 + closingFM + len(fmDelim)
	catalogIdx := strings.Index(body, catalogHeading)
	if catalogIdx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' heading")
	}
	region := body[bodyStart:catalogIdx]
	gb := strings.Index(region, genBegin)
	ge := strings.Index(region, genEnd)
	if gb < 0 || ge < gb {
		t.Fatalf("evolve-router.md prose region missing the GENERATED:goal-recipes markers")
	}
	return len([]byte(region)) - len([]byte(region[gb:ge+len(genEnd)]))
}

func routerCatalogBytes(t *testing.T, body string) int {
	t.Helper()
	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values'")
	}
	section := body[idx:]
	next := strings.Index(section[len(heading):], "\n## ")
	if next >= 0 {
		section = section[:len(heading)+next]
	}
	return len([]byte(section))
}

func TestRouterPersona_TSCMarkerPresent(t *testing.T) {
	_, body := routerContent(t)
	if !strings.Contains(body, "<!-- TSC applied") {
		t.Errorf("RED: evolve-router.md missing '<!-- TSC applied' marker.\n" +
			"Builder must add the TSC marker (matching scout/builder/auditor personas).\n" +
			"Expected something like: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// Despite the name, this pins an anti-bloat ceiling on the prose region, not a reduction.
func TestRouterPersona_ProseRegionByteReduction(t *testing.T) {
	_, body := routerContent(t)
	got := routerProseBytes(t, body)
	const baselineBytes = 2349 // prose-only size at the re-baseline
	const maxBytes = 3000      // room for real edits; still fails on a regrowth wave
	if got >= maxBytes {
		t.Errorf("RED: prose region (excluding the generated recipe table) is %d bytes (want <%d, re-baseline=%d).\n"+
			"Apply TSC to the prose sections (## Your job, ## Output contract, ## Goal-Type Recipes prose)\n"+
			"— never the generated table or the catalog. Current: %d bytes, need to save ≥%d bytes.",
			got, maxBytes, baselineBytes, got, got-maxBytes+1)
	}
}

func TestRouterPersona_CatalogByteIdentical_Negative(t *testing.T) {
	_, body := routerContent(t)
	got := routerCatalogBytes(t, body)
	const baselineCatalogBytes = 7988
	if got != baselineCatalogBytes {
		t.Errorf("Negative: '## Phase Catalog — Core Values' section changed from baseline %d bytes to %d bytes.\n"+
			"TSC MUST NOT touch the catalog section — it is already guarded by TestRouterCompaction.\n"+
			"Only compress prose ABOVE the catalog heading.", baselineCatalogBytes, got)
	}
}

func TestRouterPersona_DomainVocabPreserved(t *testing.T) {
	_, body := routerContent(t)
	for _, token := range []string{
		"routing-plan.json",  // artifact path — must survive prose compression
		"fast|balanced|deep", // tier enum in code span — must not be paraphrased
		"writes_source",      // JSON field name in mint block example
		"ClampPlanToFloor",   // Go function name — domain vocabulary
	} {
		if !strings.Contains(body, token) {
			t.Errorf("Edge: domain vocab token %q absent from evolve-router.md after TSC.\n"+
				"TSC §3 rule: preserve code spans, JSON keys, and operator strings verbatim.\n"+
				"Check that the Builder did not paraphrase or abbreviate this token.", token)
		}
	}
}

func TestRouterPersona_LoaderAndRenderParseGreen(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	fm, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter(agents/evolve-router.md): %v — TSC may have corrupted the YAML frontmatter", err)
	}
	if fm == nil {
		t.Fatal("ParseFrontmatter returned nil frontmatter map — YAML fence is broken")
	}
	for _, key := range []string{"name", "model", "description", "tools"} {
		if _, ok := fm[key]; !ok {
			t.Errorf("frontmatter missing key %q — TSC must not corrupt YAML fields", key)
		}
	}
	if len(body) < 100 {
		t.Errorf("parsed body suspiciously short (%d bytes) — TSC may have over-deleted content", len(body))
	}
	goDir := filepath.Join(root, "go")
	_, stderr, code, subErr := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-run", "TestRouterCompaction", "./internal/prompts/")
	if subErr != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterCompaction failed after TSC pass (exit=%d, err=%v):\n%s",
			code, subErr, stderr)
	}
}
