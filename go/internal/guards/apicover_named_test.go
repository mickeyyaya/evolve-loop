//go:build integration

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises the exported guard TYPES apicover flagged uncovered in this
// package. The existing *_test.go files only call the New* constructors, so the
// bare type identifiers (Chain/DocDelete/Quota/Ship) never appear as tokens in
// test source — apicover reports the types UNCOVERED even though their methods
// are tested. Each test below names the concrete type via a typed declaration
// AND asserts a real contract: the type satisfies core.Guard and its Decide
// returns the documented Allow verdict (Rule 9 — no bare `var _ T` padding).
package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestChainType_SatisfiesGuardAndDenies names the *Chain type via a typed
// declaration, binds it to the core.Guard interface the kernel dispatches
// against, and asserts a nil-ledger Chain denies (its documented contract).
func TestChainType_SatisfiesGuardAndDenies(t *testing.T) {
	var g *Chain = NewChain(nil)
	var _ core.Guard = g // *Chain must satisfy the kernel Guard port.
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

// TestDocDeleteType_SatisfiesGuardAndDenies names the *DocDelete type and
// asserts it denies an `rm docs/` command (its documented contract).
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

// TestQuotaType_SatisfiesGuardAndEnforcesCap names the *Quota type and asserts
// it denies once the per-agent WebSearch cap is exhausted (its core contract).
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

// TestShipType_SatisfiesGuardAndDenies names the *Ship type and asserts it
// denies a bare `git commit` (the un-sanctioned ship path it gates).
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
