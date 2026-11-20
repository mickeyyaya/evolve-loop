package prompts

import (
	"regexp"
	"strings"
	"testing"
)

var (
	orchestratorActionScriptRe = regexp.MustCompile(`\b(ship|release-pipeline|record-failure-to-state|merge-lesson-into-state)\.sh\b|\bcycle-state\.sh\s+(advance|set-agent|checkpoint)\b`)
	orchestratorDecisionHeadRe = regexp.MustCompile(`(?i)decision|verdict|skip|closure`)
)

func splitMarkdownH2Sections(doc string) []struct {
	heading string
	body    string
} {
	var sections []struct {
		heading string
		body    string
	}
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			sections = append(sections, struct {
				heading string
				body    string
			}{heading: strings.TrimPrefix(line, "## ")})
			continue
		}
		if len(sections) > 0 {
			sections[len(sections)-1].body += line + "\n"
		}
	}
	return sections
}

func TestOrchestratorPersonaDecisionSectionsProposeOnly(t *testing.T) {
	p, err := NewForProject(repoRoot(t)).Agent("evolve-orchestrator")
	if err != nil {
		t.Fatalf("load evolve-orchestrator: %v", err)
	}
	for _, line := range strings.Split(p.Body, "\n") {
		if orchestratorActionScriptRe.MatchString(line) {
			t.Errorf("persona names ship/release/state script as action: %q", strings.TrimSpace(line))
		}
	}
	sections := splitMarkdownH2Sections(p.Body)
	if len(sections) == 0 {
		t.Fatal("evolve-orchestrator.md has no ## sections")
	}
	decisionCount := 0
	for _, sec := range sections {
		if !orchestratorDecisionHeadRe.MatchString(sec.heading) {
			continue
		}
		decisionCount++
		low := strings.ToLower(sec.body)
		if !strings.Contains(low, "kernel") || !strings.Contains(low, "propos") {
			t.Errorf("decision section %q must state both proposal and kernel decision", sec.heading)
		}
	}
	if decisionCount == 0 {
		t.Fatal("expected at least one decision section in evolve-orchestrator.md")
	}
}
