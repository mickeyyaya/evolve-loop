package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Ship is the port of scripts/guards/ship-gate.sh.
// Rule: ship-class verbs (git commit, git push, gh release create, gh
// release edit) are denied UNLESS the command's entry point is
// scripts/lifecycle/ship.sh. Constructor-injected bypass=true bypasses.
func TestShip_Name(t *testing.T) {
	g := NewShip(false)
	if g.Name() != "ship" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestShip_AllowsCanonicalShipScript(t *testing.T) {
	g := NewShip(false)
	cases := []string{
		"bash scripts/lifecycle/ship.sh 'msg'",
		"scripts/lifecycle/ship.sh --class manual 'msg'",
		"bash /Users/x/evolve-loop/scripts/lifecycle/ship.sh --class cycle 'msg'",
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if !dec.Allow {
			t.Errorf("%q denied: %s", cmd, dec.Reason)
		}
	}
}

func TestShip_DeniesBareGitCommit(t *testing.T) {
	g := NewShip(false)
	cases := []string{
		"git commit -m 'msg'",
		"git push origin main",
		"git push --force",
		"gh release create v1.0.0",
		"gh release edit v1.0.0 --notes 'x'",
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if dec.Allow {
			t.Errorf("%q allowed (must deny)", cmd)
		}
	}
}

func TestShip_DeniesPipedShipVerbs(t *testing.T) {
	g := NewShip(false)
	// Common bypass attempts.
	for _, cmd := range []string{
		`echo y | git commit -m 'x'`,
		`bash -c "git commit -m 'x'"`,
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if dec.Allow {
			t.Errorf("%q allowed (bypass attempt)", cmd)
		}
	}
}

func TestShip_BypassAllows(t *testing.T) {
	g := NewShip(true)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "git commit -m 'x'"},
	})
	if !dec.Allow {
		t.Errorf("bypass env must allow, got: %s", dec.Reason)
	}
}

func TestShip_AllowsNonShipBash(t *testing.T) {
	g := NewShip(false)
	cases := []string{
		"ls",
		"git status",
		"git log -1",
		"git diff",
		"go test ./...",
		"echo 'commit time'", // contains 'commit' but not a verb
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if !dec.Allow {
			t.Errorf("%q denied: %s", cmd, dec.Reason)
		}
	}
}

func TestShip_MissingCommandAllows(t *testing.T) {
	g := NewShip(false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{},
	})
	if !dec.Allow {
		t.Errorf("missing command must allow: %s", dec.Reason)
	}
}

func TestShip_NonBashToolPassesThrough(t *testing.T) {
	g := NewShip(false)
	for _, tool := range []string{"Edit", "Write", "Read"} {
		dec := g.Decide(context.Background(), core.GuardInput{ToolName: tool})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}
