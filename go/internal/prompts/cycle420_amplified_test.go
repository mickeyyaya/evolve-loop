package prompts

// cycle420_amplified_test.go — Adversarial amplification for cycle-420 task T2.
//
// Probes gaps NOT covered by router_persona_test.go (AC1–AC5):
//
//   - GENERATED block markers: <!-- GENERATED:goal-recipes BEGIN/END --> must survive TSC
//     (build-report: "NOT touched" — no regression guard existed before this test).
//   - Section headings: ## Your job, ## Output contract, ## Goal-Type Recipes must be present
//     after TSC (headings are the structural skeleton, distinct from prose content).
//   - Prose floor: prose region must be >2500 bytes (prevents over-deletion gaming that
//     preserves domain tokens while stripping all decision logic context).
//   - Extended vocab — run: field: domain token "run:" preserved (build-report vocab list
//     item not covered by the 4-token TestRouterPersona_DomainVocabPreserved test).

import (
	"strings"
	"testing"
)

// TestRouterPersona_GeneratedBlockMarkersPreserved asserts that the GENERATED block
// markers <!-- GENERATED:goal-recipes BEGIN --> and <!-- GENERATED:goal-recipes END -->
// are present in agents/evolve-router.md after the TSC pass.
//
// Amplification angle: the build-report states "NOT touched: <!-- GENERATED:goal-recipes
// BEGIN/END --> block". No existing test guards this contract. If TSC accidentally
// deleted or altered the markers, the code generator that re-fills the block would
// fail silently on the next generation pass.
func TestRouterPersona_GeneratedBlockMarkersPreserved(t *testing.T) {
	_, body := routerContent(t)
	for _, marker := range []string{
		"GENERATED:goal-recipes BEGIN",
		"GENERATED:goal-recipes END",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("evolve-router.md missing GENERATED block marker %q after TSC pass.\n"+
				"Build-report spec: '<!-- GENERATED:goal-recipes BEGIN/END --> block NOT touched'.\n"+
				"TSC must leave the GENERATED block delimiters intact.", marker)
		}
	}
}

// TestRouterPersona_SectionHeadingsPreserved asserts that the key section headings
// in agents/evolve-router.md are still present after TSC prose compression.
//
// Amplification angle: AC2 checks byte count; AC4 checks code-span tokens; neither
// verifies that the structural headings (## Your job, ## Output contract,
// ## Goal-Type Recipes) still exist. A builder who deletes a heading and inlines its
// content under another section could pass byte-count and vocab tests while breaking
// the router's semantic structure.
func TestRouterPersona_SectionHeadingsPreserved(t *testing.T) {
	_, body := routerContent(t)
	for _, heading := range []string{
		"## Your job",
		"## Output contract",
		"## Goal-Type Recipes",
	} {
		if !strings.Contains(body, heading) {
			t.Errorf("evolve-router.md missing section heading %q after TSC pass.\n"+
				"TSC §3 rule: compress prose content, not structural headings.\n"+
				"Removing headings breaks the router's semantic layout.", heading)
		}
	}
}

// TestRouterPersona_ProseFloor asserts that the prose region of agents/evolve-router.md
// (from end of frontmatter to ## Phase Catalog — Core Values) is at least 2500 bytes.
//
// Amplification angle: AC2 asserts an upper bound (<5243 bytes, ≥15% reduction).
// Without a floor, a builder could game the byte limit by stripping all prose content
// except domain tokens, passing AC2 while destroying all decision-routing context.
// 2500 bytes ≈ 48% of the original 5235-byte post-TSC prose — a generous floor that
// catches catastrophic over-deletion without constraining legitimate future compression.
func TestRouterPersona_ProseFloor(t *testing.T) {
	_, body := routerContent(t)
	got := routerProseBytes(t, body)
	const minBytes = 2500
	if got < minBytes {
		t.Errorf("evolve-router.md prose region suspiciously small: %d bytes (floor=%d bytes).\n"+
			"TSC must compress prose, not delete routing decision context.\n"+
			"If prose is this small, critical instructions may have been lost.", got, minBytes)
	}
}

// TestRouterPersona_ExtendedVocabRunField asserts that the "run:" domain token is
// preserved in agents/evolve-router.md after TSC.
//
// Amplification angle: TestRouterPersona_DomainVocabPreserved covers 4 tokens
// (routing-plan.json, fast|balanced|deep, writes_source, ClampPlanToFloor).
// The build-report spec lists a 5th token: "run: true/false" — the JSON field that
// controls whether a phase executes. This token is critical for routing-plan semantics.
func TestRouterPersona_ExtendedVocabRunField(t *testing.T) {
	_, body := routerContent(t)
	if !strings.Contains(body, "run:") {
		t.Errorf("evolve-router.md missing domain vocab token \"run:\" after TSC pass.\n" +
			"Build-report spec vocab list includes 'run: true/false' — the routing-plan\n" +
			"field that controls phase execution. TSC must preserve this token verbatim.")
	}
}
