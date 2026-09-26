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
	var g *DocDelete = NewDocDelete(false)
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

func TestQuotaType_SatisfiesGuardAndEnforcesCap(t *testing.T) {
	var g *Quota = NewQuota(QuotaConfig{WebSearch: 1})
	var _ core.Guard = g
	if g.Name() != "quota" {
		t.Fatalf("Quota.Name() = %q, want quota", g.Name())
	}
	in := core.GuardInput{ToolName: "WebSearch", ToolInput: map[string]any{"agent": "scout"}}
	if dec := g.Decide(context.Background(), in); !dec.Allow {
		t.Fatalf("first WebSearch under cap=1 must allow: %s", dec.Reason)
	}
	if dec := g.Decide(context.Background(), in); dec.Allow {
		t.Fatal("second WebSearch over cap=1 must deny, got Allow=true")
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
