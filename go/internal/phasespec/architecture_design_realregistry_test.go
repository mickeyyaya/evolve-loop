package phasespec

import (
	"path/filepath"
	"testing"
)

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

func TestRealRegistry_PlanReviewAgentResolves(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	cat, err := Load(path)
	if err != nil {
		t.Fatalf("Load real registry: %v", err)
	}
	s, ok := cat.Get("plan-review")
	if !ok {
		t.Fatal("plan-review missing from the real registry")
	}
	if got := s.AgentName(); got != "plan-reviewer" {
		t.Errorf("AgentName = %q, want plan-reviewer (the persona file that exists); "+
			"the evolve-plan-review default has no persona", got)
	}
	if s.RoleOrDefault() != RolePlan {
		t.Errorf("archetype = %q, want plan (fallback candidate + advisor catalog card)", s.RoleOrDefault())
	}
}
