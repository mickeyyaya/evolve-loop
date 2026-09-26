package prompts

import (
	"strings"
	"testing"
)

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

func TestRouterPersona_ProseFloor(t *testing.T) {
	_, body := routerContent(t)
	got := routerProseBytes(t, body)
	const minBytes = 1200
	if got < minBytes {
		t.Errorf("evolve-router.md prose region suspiciously small: %d bytes (floor=%d bytes).\n"+
			"TSC must compress prose, not delete routing decision context.\n"+
			"If prose is this small, critical instructions may have been lost.", got, minBytes)
	}
}

func TestRouterPersona_ExtendedVocabRunField(t *testing.T) {
	_, body := routerContent(t)
	if !strings.Contains(body, "run:") {
		t.Errorf("evolve-router.md missing domain vocab token \"run:\" after TSC pass.\n" +
			"Build-report spec vocab list includes 'run: true/false' — the routing-plan\n" +
			"field that controls phase execution. TSC must preserve this token verbatim.")
	}
}
