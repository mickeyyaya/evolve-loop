package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestComposedGatesTo_ARedGateCarriesItsNamedTestsAndFlags(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	makefile := "build:\n\t@true\n" +
		"test:\n\t@printf -- '--- FAIL: TestX (0.00s)\\nFAIL\\nFAIL\\texample.com/m/p\\t0.1s\\n'; exit 1\n" +
		"test-acs-durable:\n\t@printf -- '--- FAIL: TestY (0.00s)\\nFAIL\\nFAIL\\texample.com/m/q\\t0.1s\\n'; exit 1\n" +
		"apicover-enforce:\n\t@echo boom; exit 1\n"
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go", "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}

	results := composedGatesTo(&bytes.Buffer{})(context.Background(), worktree)

	if got, want := results["test"].Reds, (baseattr.Reds{Keys: []baseattr.Key{{Package: "example.com/m/p", Test: "TestX"}}, Flags: []string{"-race"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("test reds = %+v, want %+v", got, want)
	}
	if got, want := results["acs"].Reds, (baseattr.Reds{Keys: []baseattr.Key{{Package: "example.com/m/q", Test: "TestY"}}}); !reflect.DeepEqual(got, want) {
		t.Errorf("acs reds = %+v, want %+v", got, want)
	}
	if got := results["apicover"].Reds; got.Attributable() || len(got.Flags) != 2 || got.Flags[0] != "-tags" || got.Flags[1] != "integration" {
		t.Errorf("apicover reds = %+v, want no named test and the integration tag", got)
	}
	if got := results["compile"].Reds; got.Attributable() || got.Flags != nil {
		t.Errorf("a green gate has reds %+v, want none", got)
	}
}
