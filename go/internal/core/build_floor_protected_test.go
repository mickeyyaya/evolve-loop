package core

import (
	"context"
	"strings"
	"testing"
)

func TestProtectedSurfaceFailures_NamesEveryProtectedPath(t *testing.T) {
	member := func(p string) bool { return p == "go/internal/core/cyclerun.go" || p == "agents/evolve-builder.md" }
	got := protectedSurfaceFailures([]string{"go/internal/core/lost_landing_floor.go", "go/internal/core/cyclerun.go", "docs/x.md", "agents/evolve-builder.md"}, member, "b279c319")
	if len(got) != 2 {
		t.Fatalf("one failure per protected changed path, got %d: %v", len(got), got)
	}
	for i, p := range []string{"go/internal/core/cyclerun.go", "agents/evolve-builder.md"} {
		if !strings.Contains(got[i], p) || !strings.Contains(got[i], "ADR-0064") || !strings.Contains(got[i], "git checkout b279c319 -- "+p) || !strings.Contains(got[i], "delete it if it is new") {
			t.Errorf("failure %d must name %s, the boundary and the exact in-phase fix against the cycle base: %q", i, p, got[i])
		}
	}
	if got := protectedSurfaceFailures([]string{"docs/x.md"}, member, "HEAD"); got != nil {
		t.Errorf("no protected path, no failure: %v", got)
	}
	if got := protectedSurfaceFailures([]string{"go/internal/core/cyclerun.go"}, nil, "HEAD"); got != nil {
		t.Errorf("an unwired predicate fails open like every floor: %v", got)
	}
}

func TestProtectedSurfaceFloorChecks_FailsOpenWithoutAWorktree(t *testing.T) {
	check := ProtectedSurfaceFloorChecks(func(string) bool { return true })
	if got := check(context.Background(), ReviewInput{Phase: string(PhaseBuild)}); got != nil {
		t.Errorf("no worktree, nothing to judge: %v", got)
	}
}
