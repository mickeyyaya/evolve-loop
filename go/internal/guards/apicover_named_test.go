//go:build integration

package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// The typed declarations below name each guard type, which apicover needs to count the type as covered.
func TestChainType_SatisfiesGuardAndDenies(t *testing.T) {
	var g *Chain = NewChain(nil)
	var _ core.Guard = g
	if g.Name() != "chain" {
		t.Fatalf("Chain.Name() = %q, want chain", g.Name())
	}
	dec := g.Decide(context.Background(), core.GuardInput{})
	if dec.Allow {
		t.Fatal("Chain with nil ledger must deny, got Allow=true")
	}
	if dec.Reason == "" {
		t.Fatal("Chain denial must carry a Reason")
	}
}

func TestDocDeleteType_SatisfiesGuardAndDenies(t *testing.T) {
	var g *DocDelete = NewDocDelete(false, nil)
	var _ core.Guard = g
	if g.Name() != "docdelete" {
		t.Fatalf("DocDelete.Name() = %q, want docdelete", g.Name())
	}
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "rm -rf docs/architecture"},
	})
	if dec.Allow {
		t.Fatal("DocDelete must deny rm against docs/, got Allow=true")
	}
}

func TestShipType_SatisfiesGuardAndDenies(t *testing.T) {
	var g *Ship = NewShip(false)
	var _ core.Guard = g
	if g.Name() != "ship" {
		t.Fatalf("Ship.Name() = %q, want ship", g.Name())
	}
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "git commit -m 'x'"},
	})
	if dec.Allow {
		t.Fatal("Ship must deny a bare git commit, got Allow=true")
	}
}
