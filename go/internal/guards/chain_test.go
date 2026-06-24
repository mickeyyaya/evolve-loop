package guards

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func setupLedger(t *testing.T) (*ledger.FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return ledger.New(evolveDir), evolveDir
}

func TestChain_Name(t *testing.T) {
	g := NewChain(nil)
	if g.Name() != "chain" {
		t.Errorf("Name=%q, want chain", g.Name())
	}
}

func TestChain_AllowsIntact(t *testing.T) {
	l, _ := setupLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "build", Cycle: 1})

	g := NewChain(l)
	dec := g.Decide(context.Background(), core.GuardInput{})
	if !dec.Allow {
		t.Errorf("intact chain denied: %s", dec.Reason)
	}
}

func TestChain_DeniesBroken(t *testing.T) {
	l, dir := setupLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "build", Cycle: 1})

	// Tamper to break the chain.
	path := filepath.Join(dir, "ledger.jsonl")
	raw, _ := os.ReadFile(path)
	tampered := strings.Replace(string(raw), `"role":"scout"`, `"role":"FORGED"`, 1)
	_ = os.WriteFile(path, []byte(tampered), 0o644)

	g := NewChain(l)
	dec := g.Decide(context.Background(), core.GuardInput{})
	if dec.Allow {
		t.Error("broken chain allowed — guard didn't catch tampering")
	}
	if dec.Reason == "" {
		t.Error("denied without a reason")
	}
}

// A fake ledger that returns a non-chain-broken error — guard should
// still deny (it doesn't try to distinguish error kinds).
type erroringLedger struct{}

func (e *erroringLedger) Append(_ context.Context, _ core.LedgerEntry) error { return nil }
func (e *erroringLedger) Verify(_ context.Context) error {
	return errors.New("synthetic verify error")
}
func (e *erroringLedger) Iter(_ context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("not used")
}

func TestChain_DeniesOnAnyVerifyError(t *testing.T) {
	g := NewChain(&erroringLedger{})
	dec := g.Decide(context.Background(), core.GuardInput{})
	if dec.Allow {
		t.Error("verify error → guard must deny")
	}
}

func TestChain_NilLedgerDenies(t *testing.T) {
	g := NewChain(nil)
	dec := g.Decide(context.Background(), core.GuardInput{})
	if dec.Allow {
		t.Error("nil ledger must deny")
	}
	if dec.Reason == "" {
		t.Error("denied without reason")
	}
}

func TestChain_AllowsEmptyLedger(t *testing.T) {
	l, _ := setupLedger(t)
	g := NewChain(l)
	dec := g.Decide(context.Background(), core.GuardInput{})
	if !dec.Allow {
		t.Errorf("empty ledger denied: %s", dec.Reason)
	}
}
