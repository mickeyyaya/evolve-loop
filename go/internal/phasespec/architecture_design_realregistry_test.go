package phasespec

import (
	"path/filepath"
	"testing"
)

// TestRealRegistry_ArchitectureDesign guards the architecture-design phase entry
// in the actual repo registry: it must load as a Plan-archetype, kind:llm,
// optional spec phase whose AgentName resolves to its persona, with the required
// classify sections. This is the WS3 free-topology proof phase — the specrunner
// fallback (registerBuiltinSpecRunners) wires it only because it is non-Control
// kind:llm with a persona, and the advisor catalog (phaseCardsFromCatalog) shows
// it only because RoleOrDefault()==plan (non-Control).
func TestRealRegistry_ArchitectureDesign(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	cat, err := Load(path)
	if err != nil {
		t.Fatalf("Load real registry: %v", err)
	}
	s, ok := cat.Get("architecture-design")
	if !ok {
		t.Fatal("architecture-design missing from the real registry")
	}
	if got := s.RoleOrDefault(); got != RolePlan {
		t.Errorf("archetype = %q, want plan (advisor catalog includes it as a Plan card)", got)
	}
	if s.KindOrDefault() != "llm" {
		t.Errorf("kind = %q, want llm (specrunner fallback only wires kind:llm)", s.KindOrDefault())
	}
	if !s.Optional {
		t.Error("architecture-design must be optional (advisor-selected / trigger-inserted, not on the spine)")
	}
	if s.AgentName() != "evolve-architecture-design" {
		t.Errorf("AgentName = %q, want evolve-architecture-design (its persona file)", s.AgentName())
	}
	if s.Classify == nil || len(s.Classify.RequireSections) == 0 {
		t.Fatal("architecture-design must declare classify.require_sections")
	}
	wantSection := "## Decision"
	found := false
	for _, sec := range s.Classify.RequireSections {
		if sec == wantSection {
			found = true
		}
	}
	if !found {
		t.Errorf("require_sections %v must include %q", s.Classify.RequireSections, wantSection)
	}
}
