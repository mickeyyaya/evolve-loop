package buildsite

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The "AI-composed pipeline" demo draws a moving connector line (.pld-railfill,
// with a glowing leading dot) along a rail that runs BEHIND the phase nodes.
//
// For that line to stay HIDDEN behind a phase card (and never show through / on
// top of it), each on-rail phase card must be visually OPAQUE. Two things break
// that, and both have bitten us:
//
//  1. A state sets a translucent background (e.g. pick = rgba(accent,0.1)) with no
//     opaque base behind it  -> the line shows through the tint.
//  2. A state sets element opacity < 1 (default/considering/ghost used to)        -> the
//     WHOLE card, opaque base included, becomes translucent and the line bleeds.
//
// These tests pin the fix so neither class can silently regress. They parse the
// shared pipeline-demo partial (one source of truth for all five themes) and
// assert the stacking invariants directly.
//
// Exemptions (intentional, asserted explicitly so a reviewer sees them):
//   - .pld-node.skip   : translated 48px DOWN, off the rail line, so nothing is
//                        behind it to show through; its fade is a "removed" signal.
//   - .pld-node.premint: the invisible pre-pop placeholder (opacity:0); it carries
//                        no visible card, so there is nothing to occlude.

const pipelinePartial = "../../templates/partials.html"

// onRailVisibleStates are the phase-card states that sit ON the rail and are
// visible to the user. Each MUST stay opaque so the connector line hides behind it.
var onRailVisibleStates = []string{
	".pld-node",             // base / not-yet-decided
	".pld-node.considering", // being evaluated
	".pld-node.pick",        // chosen common phase
	".pld-node.floor.on",    // mandated floor phase
	".pld-node.mint",        // newly written phase
	".pld-node.ghost",       // "+ unlimited" placeholder
}

// tintedStates layer a translucent tint and therefore MUST composite it over the
// opaque --pl-node-base base layer.
var tintedStates = []string{".pld-node.pick", ".pld-node.floor.on", ".pld-node.mint"}

func readPipelinePartial(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(pipelinePartial)
	if err != nil {
		t.Fatalf("read %s: %v", pipelinePartial, err)
	}
	return string(b)
}

// ruleBody returns the declaration block ({...}) of the CSS rule whose selector is
// EXACTLY sel (so ".pld-node.pick" does not accidentally match
// ".pld-node.pick .pl-badge" or ".pld-node.picked"). Returns ("", false) if absent.
func ruleBody(css, sel string) (string, bool) {
	// sel, optional whitespace, then "{" — the "{" right after the selector is what
	// makes the match exact (a descendant/compound selector would have more tokens).
	re := regexp.MustCompile(regexp.QuoteMeta(sel) + `\s*\{([^}]*)\}`)
	m := re.FindStringSubmatch(css)
	if m == nil {
		return "", false
	}
	return m[1], true
}

var opacityDecl = regexp.MustCompile(`(?:^|[;{\s])opacity\s*:\s*([0-9.]+)`)

var zIndexDecl = regexp.MustCompile(`z-index\s*:\s*([0-9]+)`)

// declaredOpacity returns the opacity declared in a rule body and whether one was
// declared at all (a rule with no opacity inherits, which for on-rail cards is the
// opaque base value).
func declaredOpacity(body string) (float64, bool) {
	m := opacityDecl.FindStringSubmatch(body)
	if m == nil {
		return 0, false
	}
	// hand-roll a tiny parse; values here are like "1", ".24", "0.6"
	s := m[1]
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	var v float64
	whole, frac, hasFrac := splitDot(s)
	v = float64(whole)
	if hasFrac {
		div := 1.0
		for range frac {
			div *= 10
		}
		v += float64(atoi(frac)) / div
	}
	return v, true
}

func splitDot(s string) (whole int, frac string, hasFrac bool) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return atoi(s[:i]), s[i+1:], true
	}
	return atoi(s), "", false
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			continue
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// TestPipelineNodesAreOpaque_NoElementOpacity asserts that no on-rail visible
// phase-card state uses element opacity < 1. Element opacity < 1 makes the whole
// card (opaque base included) translucent, so the connector line would bleed
// through. This is the regression guard for the default/considering/ghost fix.
func TestPipelineNodesAreOpaque_NoElementOpacity(t *testing.T) {
	css := readPipelinePartial(t)
	for _, sel := range onRailVisibleStates {
		body, ok := ruleBody(css, sel)
		if !ok {
			t.Errorf("rule %q not found in %s — did a selector rename break the stacking guard?", sel, pipelinePartial)
			continue
		}
		op, declared := declaredOpacity(body)
		if declared && op < 1 {
			t.Errorf("%q declares opacity:%v (<1) — an on-rail card MUST stay opaque so the connector line stays hidden behind it; dim it via muted color, not element opacity", sel, op)
		}
	}
}

// TestPipelineTintedNodesCompositeOverOpaqueBase asserts every translucent-tinted
// state layers its tint OVER var(--pl-node-base) (the opaque base), so the tint
// stays visible while the card still occludes the line.
func TestPipelineTintedNodesCompositeOverOpaqueBase(t *testing.T) {
	css := readPipelinePartial(t)
	for _, sel := range tintedStates {
		body, ok := ruleBody(css, sel)
		if !ok {
			t.Fatalf("rule %q not found in %s", sel, pipelinePartial)
		}
		if !strings.Contains(body, "background:") {
			t.Errorf("%q sets no background — expected a tint composited over var(--pl-node-base)", sel)
			continue
		}
		if !strings.Contains(body, "var(--pl-node-base)") {
			t.Errorf("%q background does not composite over var(--pl-node-base) — the translucent tint will let the connector line show through; append \", var(--pl-node-base)\" as the opaque base layer", sel)
		}
	}
}

// TestPipelineNodeBaseIsOpaqueByConstruction asserts the opaque base var exists,
// is the base layer of the default node, and is not mixed with `transparent`
// (which would make it translucent regardless of theme).
func TestPipelineNodeBaseIsOpaqueByConstruction(t *testing.T) {
	css := readPipelinePartial(t)

	def := regexp.MustCompile(`--pl-node-base\s*:\s*([^;]+);`)
	m := def.FindStringSubmatch(css)
	if m == nil {
		t.Fatalf("--pl-node-base is not defined in %s — it is the opaque backing every phase card relies on", pipelinePartial)
	}
	if strings.Contains(m[1], "transparent") {
		t.Errorf("--pl-node-base = %q mixes in `transparent` — it must resolve opaque so cards occlude the line", strings.TrimSpace(m[1]))
	}

	base, ok := ruleBody(css, ".pld-node")
	if !ok {
		t.Fatal(".pld-node base rule not found")
	}
	if !strings.Contains(base, "background:var(--pl-node-base)") {
		t.Errorf(".pld-node base background must be var(--pl-node-base) (opaque); got body %q", base)
	}
}

// TestPipelineRailStaysBehindNodes asserts the z-order contract: phase nodes paint
// above the rail. Nodes pin z-index:2; the rail/fill must not claim a z-index that
// would lift the line on top of the cards.
func TestPipelineRailStaysBehindNodes(t *testing.T) {
	css := readPipelinePartial(t)

	base, ok := ruleBody(css, ".pld-node")
	if !ok || !strings.Contains(base, "z-index:2") {
		t.Errorf(".pld-node must keep z-index:2 so it paints above the rail; body %q", base)
	}
	for _, sel := range []string{".pld-rail", ".pld-railfill"} {
		body, ok := ruleBody(css, sel)
		if !ok {
			t.Fatalf("rule %q not found", sel)
		}
		if zi := zIndexDecl.FindStringSubmatch(body); zi != nil {
			if atoi(zi[1]) >= 2 {
				t.Errorf("%q sets z-index:%s (>=2) — that lifts the connector line on top of the phase cards", sel, zi[1])
			}
		}
	}
}
