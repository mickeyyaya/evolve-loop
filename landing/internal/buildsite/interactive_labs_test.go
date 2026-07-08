package buildsite

import (
	"os"
	"strings"
	"testing"

	"evolve-loop-landing/internal/content"
)

// The three interactive labs (inbox / gates / recovery) are single-source
// partials projected into every style page. These tests pin two invariants:
//
//  1. partials.html defines each lab's markup partial AND its js twin — the
//     labs exist exactly once.
//  2. EVERY style template includes all six — the classic drift here is a new
//     section landing in two styles and silently missing from the other three,
//     which no per-template test would notice.

var labPartials = []string{"tryit", "inboxlab", "gatelab", "recoverylab"}

// copyjs is listed because tryit's copy button depends on it — a style page
// including tryit without copyjs would render a dead button.
var labScripts = []string{"copyjs", "inboxlabjs", "gatelabjs", "recoverylabjs"}

var styleTemplates = []string{
	"../../templates/editorial.html",
	"../../templates/aurora.html",
	"../../templates/luminous.html",
	"../../templates/noir.html",
	"../../templates/blueprint.html",
}

func TestPartials_DefineAllLabs(t *testing.T) {
	src, err := os.ReadFile("../../templates/partials.html")
	if err != nil {
		t.Fatalf("read partials.html: %v", err)
	}
	for _, name := range append(append([]string{}, labPartials...), labScripts...) {
		if !strings.Contains(string(src), `{{define "`+name+`"}}`) {
			t.Errorf("partials.html missing {{define %q}}", name)
		}
	}
}

func TestEveryStyleTemplate_IncludesAllLabs(t *testing.T) {
	for _, path := range styleTemplates {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, name := range labPartials {
			if !strings.Contains(string(src), `{{template "`+name+`" .}}`) {
				t.Errorf("%s missing {{template %q .}}", path, name)
			}
		}
		for _, name := range labScripts {
			if !strings.Contains(string(src), `{{template "`+name+`"}}`) {
				t.Errorf("%s missing {{template %q}}", path, name)
			}
		}
	}
}

// The gate-lab predicate is evaluated in client JS over ROLE groups (the
// partial's own closed vocabulary: build/audit/tdd/trivial/and — the formula
// itself). Check KEYS are content-owned and open: renaming one in content.json
// must never require a JS edit, so no check key literal may appear in the
// script. This loads the real keys so the pin updates itself with the content.
func TestGateLabScript_HasNoHardcodedCheckKeys(t *testing.T) {
	src, err := os.ReadFile("../../templates/partials.html")
	if err != nil {
		t.Fatalf("read partials.html: %v", err)
	}
	js := extractDefine(t, string(src), "gatelabjs")
	site, err := content.Load("../../shared/content.json")
	if err != nil {
		t.Fatalf("load content: %v", err)
	}
	for _, c := range site.GateLab.Checks {
		if strings.Contains(js, `'`+c.Key+`'`) || strings.Contains(js, `"`+c.Key+`"`) {
			t.Errorf("gatelabjs hardcodes check key %q — keys must flow from content.json via data attributes", c.Key)
		}
	}
}

// The agent-readable briefing (llms.txt) must exist, carry the install
// one-liner, and be wired as a RootFile so it lands at the site root — the
// audience increasingly arrives as an AI agent acting for the developer.
func TestLLMsTxt_ExistsAndWiredAsRootFile(t *testing.T) {
	txt, err := os.ReadFile("../../shared/llms.txt")
	if err != nil {
		t.Fatalf("read shared/llms.txt: %v", err)
	}
	if !strings.Contains(string(txt), "install.sh | sh") {
		t.Error("llms.txt missing the install one-liner")
	}
	mainSrc, err := os.ReadFile("../../cmd/build/main.go")
	if err != nil {
		t.Fatalf("read cmd/build/main.go: %v", err)
	}
	if !strings.Contains(string(mainSrc), `llms.txt`) {
		t.Error("cmd/build/main.go does not wire shared/llms.txt as a RootFile")
	}
}

// extractDefine returns the body from {{define "name"}} to the FIRST {{end}} —
// correct only for defines with no nested template actions (true for the
// script partials it is used on). A define that grows a nested {{if}}/{{range}}
// would truncate here; extend to depth-counting before reusing it on markup.
func extractDefine(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, `{{define "`+name+`"}}`)
	if start < 0 {
		t.Fatalf("define %q not found", name)
	}
	end := strings.Index(src[start:], "{{end}}")
	if end < 0 {
		t.Fatalf("define %q has no end", name)
	}
	return src[start : start+end]
}

// Each lab must auto-play once scrolled into view (the pipelinedemo idiom:
// IntersectionObserver → start loop, skipped under prefers-reduced-motion) and
// must hand control to the user permanently on real interaction. These pins
// keep a future edit from silently reverting a lab to click-to-start.
func TestLabScripts_AutoPlayOnScrollWithUserHandoff(t *testing.T) {
	src, err := os.ReadFile("../../templates/partials.html")
	if err != nil {
		t.Fatalf("read partials.html: %v", err)
	}
	for _, name := range []string{"inboxlabjs", "gatelabjs", "recoverylabjs"} {
		js := extractDefine(t, string(src), name)
		if !strings.Contains(js, "IntersectionObserver") {
			t.Errorf("%s: no IntersectionObserver — lab will not auto-play on scroll", name)
		}
		if !strings.Contains(js, "userDriving") {
			t.Errorf("%s: no userDriving handoff — auto-play would fight the user", name)
		}
	}
}
