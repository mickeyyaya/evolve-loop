package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// router_persona_test.go — RED contract for cycle-420 task router-persona-tsc-compress.
//
// RED state (before builder):
//   - evolve-router.md has no "<!-- TSC applied" marker (TSC=0)
//   - prose region (frontmatter-end → "## Phase Catalog — Core Values") is 6169 bytes
//     (want < 5243, i.e. ≥15% reduction)
//   - catalog section (7988 bytes) must remain byte-identical

// routerContent reads evolve-router.md and returns raw bytes and body string.
// Reuses the package-level repoRoot from realdoc_strip_test.go.
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

// routerProseBytes returns the byte length of the prose region in evolve-router.md:
// from the end of the YAML frontmatter block to (not including) the
// "## Phase Catalog — Core Values" heading.
func routerProseBytes(t *testing.T, body string) int {
	t.Helper()
	const fmDelim = "---\n"
	const catalogHeading = "## Phase Catalog — Core Values"
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
	return len([]byte(body[bodyStart:catalogIdx]))
}

// routerCatalogBytes returns the byte length of the "## Phase Catalog — Core Values"
// section (from the heading to the next "## " heading or EOF).
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

// TestRouterPersona_TSCMarkerPresent asserts that evolve-router.md carries the
// "<!-- TSC applied" marker, matching scout/builder/auditor.
//
// AC1 — router-persona-tsc-compress.
//
// RED baseline: evolve-router.md has no TSC marker (TSC=0 per scout-report.md).
// Builder must add "<!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->"
// (or similar form matching the pattern) at the top of the persona body.
func TestRouterPersona_TSCMarkerPresent(t *testing.T) {
	_, body := routerContent(t)
	if !strings.Contains(body, "<!-- TSC applied") {
		t.Errorf("RED: evolve-router.md missing '<!-- TSC applied' marker.\n" +
			"Builder must add the TSC marker (matching scout/builder/auditor personas).\n" +
			"Expected something like: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// TestRouterPersona_ProseRegionByteReduction asserts that the prose region of
// evolve-router.md (from end of frontmatter to the Phase Catalog heading) is
// strictly less than 5243 bytes (≥15% below the 6169-byte baseline).
//
// AC2 — router-persona-tsc-compress.
//
// RED baseline: prose region is 6169 bytes; 6169 ≥ 5243 → fails until TSC applied.
// Edge: 5243 = floor(6169 × 0.85); even hitting exactly 5243 still fails (strict <).
func TestRouterPersona_ProseRegionByteReduction(t *testing.T) {
	_, body := routerContent(t)
	got := routerProseBytes(t, body)
	const baselineBytes = 6169
	const maxBytes = 5243 // floor(6169 * 0.85) — ≥15% reduction required
	if got >= maxBytes {
		t.Errorf("RED: prose region is %d bytes (want <%d, baseline=%d).\n"+
			"Builder must apply TSC to the prose sections (## Your job, ## Output contract, ## Goal-Type Recipes prose)\n"+
			"to achieve ≥15%% reduction. Current: %d bytes, need to save ≥%d bytes.",
			got, maxBytes, baselineBytes, got, got-maxBytes+1)
	}
}

// TestRouterPersona_CatalogByteIdentical_Negative asserts that the
// "## Phase Catalog — Core Values" section is exactly 7988 bytes (the baseline).
// TSC must not touch the catalog table — it is guarded by TestRouterCompaction.
//
// AC3 (negative) — router-persona-tsc-compress.
//
// Pre-existing GREEN: catalog is 7988 bytes before Builder runs.
// Anti-gaming sentinel: catches a builder who reduces prose bytes by trimming the catalog.
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

// TestRouterPersona_DomainVocabPreserved asserts that key domain vocabulary tokens
// (code spans, JSON keys, operator strings) are preserved verbatim in evolve-router.md.
//
// AC4 (edge) — router-persona-tsc-compress.
//
// Pre-existing GREEN: all tokens present before Builder runs.
// Regression guard: TSC must not strip backtick spans or JSON field names.
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

// TestRouterPersona_LoaderAndRenderParseGreen asserts that prompts.ParseFrontmatter
// still parses evolve-router.md correctly after the TSC pass.
//
// AC5 (regression) — router-persona-tsc-compress.
//
// Pre-existing GREEN: file parses cleanly before Builder runs.
// Regression guard: TSC must not corrupt the YAML frontmatter or break the body.
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
	// Also verify the existing compaction test still passes by calling SubprocessOutput.
	goDir := filepath.Join(root, "go")
	_, stderr, code, subErr := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-run", "TestRouterCompaction", "./internal/prompts/")
	if subErr != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterCompaction failed after TSC pass (exit=%d, err=%v):\n%s",
			code, subErr, stderr)
	}
}
