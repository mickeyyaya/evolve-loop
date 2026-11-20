package testaudit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	dir := t.TempDir()
	src := "package fx\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\t_ = 1\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	var (
		_ Guard
		_ Fake
	)
	found, err := Analyze(dir, cfg)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := Finding{Package: dir, Test: "TestNothing", Rule: "no-assertion"}
	if len(found) != 1 || found[0] != want {
		t.Fatalf("Analyze = %v, want [%v]", found, want)
	}
	if len(RuleIDs()) != len(rules) {
		t.Errorf("RuleIDs() = %v, want one id per rule", RuleIDs())
	}
	if err := WriteShard(dir, found); err != nil {
		t.Fatalf("WriteShard: %v", err)
	}
	if err := CheckShard(dir, cfg); err != nil {
		t.Errorf("CheckShard on a fresh shard: %v", err)
	}
	treeRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(treeRoot, "pkg", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(treeRoot, "pkg", "x", "x_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	treeFound, err := AnalyzeTree(treeRoot, cfg)
	if err != nil || len(treeFound) != 1 || treeFound[0].Package != "pkg/x" {
		t.Fatalf("AnalyzeTree = %v, %v; want one finding in pkg/x", treeFound, err)
	}
	if err := Rebaseline(treeRoot, "pkg/x", cfg); err != nil {
		t.Fatalf("Rebaseline: %v", err)
	}
	if err := CheckTree(treeRoot, cfg); err != nil {
		t.Errorf("CheckTree after Rebaseline: %v", err)
	}
}

func TestWriteShard_NoFindingsRemovesTheShard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	found := []Finding{{Package: dir, Test: "TestA", Rule: "sleep-sync"}}
	if err := WriteShard(dir, found); err != nil {
		t.Fatal(err)
	}
	if err := WriteShard(dir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, shardRelPath)); !os.IsNotExist(err) {
		t.Errorf("shard still present after an empty write: %v", err)
	}
}

func TestConfigValidate_RejectsRowsThatBreakTheBounds(t *testing.T) {
	t.Parallel()
	cases := map[string]Config{
		"no bound":         {},
		"too many guards":  {MaxGuardRows: 1, Guards: []Guard{{Package: "a", Reason: "r"}, {Package: "b", Reason: "r"}}},
		"guard no reason":  {MaxGuardRows: 1, Guards: []Guard{{Package: "a"}}},
		"none no reason":   {MaxGuardRows: 1, Fakes: []Fake{{Tool: "git", RealSide: "none"}}},
		"real side no dir": {MaxGuardRows: 1, Fakes: []Fake{{Tool: "git", RealSide: "binary"}}},
	}
	for name, cfg := range cases {
		if err := cfg.validate(); err == nil {
			t.Errorf("%s: validate accepted %+v", name, cfg)
		}
	}
}

func TestRowError_Unwrap(t *testing.T) {
	missing := &RowError{Kind: RowMissing}
	stale := &RowError{Kind: RowStale}
	if !errors.Is(missing, ErrNoBaselineRow) || errors.Is(missing, ErrStaleBaselineRow) {
		t.Errorf("missing row unwraps to %v, want only ErrNoBaselineRow", missing.Unwrap())
	}
	if !errors.Is(stale, ErrStaleBaselineRow) || errors.Is(stale, ErrNoBaselineRow) {
		t.Errorf("stale row unwraps to %v, want only ErrStaleBaselineRow", stale.Unwrap())
	}
}
