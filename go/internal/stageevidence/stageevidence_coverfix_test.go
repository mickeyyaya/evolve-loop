package stageevidence

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

type unverifiedReader struct {
	chain Chain
}

func (u unverifiedReader) ReadChain(context.Context, string) (Chain, error) {
	return u.chain, nil
}

func TestStageEvidence_ErrorAndFallbackBranches(t *testing.T) {
	t.Parallel()

	badJSON := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badJSON, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FileChain(badJSON).ReadChain(context.Background(), "k"); err == nil {
		t.Fatal("FileChain on malformed JSON returned nil")
	}

	r1 := Link("", Record{Key: "k", Version: "v1", Kind: KindDecision, ID: 1, Seq: 1, Fields: map[string]string{"would_stop": "false"}})
	chain := Chain{Records: []Record{r1}, Outcomes: []Outcome{{ID: 1, Shipped: true}}}
	scored := Score(chain, "v1", gatecontract.Promotion{MinDecisions: 1}, nil)
	if !scored.Ready || scored.Scorer != "default" {
		t.Fatalf("Score with nil scorer = %+v", scored)
	}

	if res, err := Verify(context.Background(), nil, "k", "v1", gatecontract.Promotion{}, nil); !errors.Is(err, ErrEmptyChain) || res.Error != "empty-chain" || res.Scorer != "default" {
		t.Fatalf("Verify with nil reader = (%+v, %v)", res, err)
	}

	tampered := Chain{Records: []Record{{Key: "k", Version: "v1", Kind: KindDecision, ID: 2, Seq: 2, Hash: "bad"}}}
	if res, err := Verify(context.Background(), unverifiedReader{chain: tampered}, "k", "v1", gatecontract.Promotion{}, nil); !errors.Is(err, ErrBrokenChain) || res.Error != "broken-chain" {
		t.Fatalf("Verify with unverified tampered chain = (%+v, %v)", res, err)
	}
}

func TestDecisionVersion_EdgeAndErrorBranches(t *testing.T) {
	t.Parallel()

	if _, err := DecisionVersion("   ", nil, []byte(`{}`), "k"); err == nil {
		t.Fatal("DecisionVersion on blank moduleDir returned nil")
	}
	if _, err := DecisionVersion(t.TempDir(), nil, []byte(`{}`), "k"); err == nil {
		t.Fatal("DecisionVersion without go.mod returned nil")
	}

	noModDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(noModDir, "go.mod"), []byte("go 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DecisionVersion(noModDir, nil, []byte(`{}`), "k"); err == nil {
		t.Fatal("DecisionVersion without module directive returned nil")
	}

	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module example.com/m\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DecisionVersion(modDir, nil, []byte(`{"k":1} {"extra":2}`), "k"); !errors.Is(err, ErrTrailingData) {
		t.Fatalf("DecisionVersion with trailing JSON = %v, want trailing data error", err)
	}
	if v, err := DecisionVersion(modDir, []string{"", " ./ "}, []byte(`{"a":1}`), ""); err != nil || v == "" {
		t.Fatalf("DecisionVersion with empty subtree and blank closure = (%q, %v)", v, err)
	}
	if v, err := DecisionVersion(modDir, nil, []byte(`{"a":1}`), "a.b"); err != nil || v == "" {
		t.Fatalf("DecisionVersion with scalar parent subtree = (%q, %v)", v, err)
	}
}

func TestHashListedPackages_ErrorBranches(t *testing.T) {
	t.Parallel()

	h := sha256.New()
	if err := hashListedPackages(h, []string{"example.com/missing"}, map[string]listedPkg{}); !errors.Is(err, ErrMissingPackage) {
		t.Fatalf("hashListedPackages missing package = %v", err)
	}

	pkgs := map[string]listedPkg{
		"example.com/p": {dir: t.TempDir(), files: []string{"absent.go"}},
	}
	if err := hashListedPackages(h, []string{"example.com/p"}, pkgs); err == nil {
		t.Fatal("hashListedPackages missing file returned nil")
	}

	gotPkgs, gotGraph := parseClosureList("\n|dir|imp|a.go|\nbadline\n")
	if len(gotPkgs) != 0 || len(gotGraph) != 0 {
		t.Fatalf("parseClosureList invalid lines = (%+v, %+v), want empty", gotPkgs, gotGraph)
	}
}
