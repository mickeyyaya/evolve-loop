package reachabilityprobe

import "testing"

func pinTestFileF4(source, ref string) string {
	return "package core\n\nimport \"testing\"\n\nfunc TestPin(t *testing.T) {\n" +
		"\tassertFileContains(t, \"" + source + "\", \"" + ref + "(\")\n}\n"
}

func TestExtractPins_APinnedFileOutsideAnyModuleContributesNothing(t *testing.T) {
	wt := t.TempDir()
	writeAliasFixtureFile(t, wt, "go/internal/core/frozen_nomod_test.go", pinTestFileF4("go/internal/core/state.go", "storage.UpdateStateMap"))

	refs := extractPins(wt, []string{"go/internal/core/frozen_nomod_test.go"})
	if len(refs) != 0 {
		t.Fatalf("a pin whose file has no go.mod above it cannot be resolved: %+v", refs)
	}
	pins, err := ExtractFrozenPins(wt, []string{"go/internal/core/frozen_nomod_test.go"})
	if err != nil || len(pins) != 0 {
		t.Fatalf("pins=%+v err=%v", pins, err)
	}
}

func TestCheckFrozenPins_AModuleWhoseImportGraphCannotBeBuiltFailsOpenOncePerModule(t *testing.T) {
	wt := t.TempDir()
	writeAliasFixtureFile(t, wt, "go/go.mod", "module example.com/broken\n\nthis line is not a go.mod directive\n")
	writeAliasFixtureFile(t, wt, "go/internal/core/state.go", "package core\n")
	writeAliasFixtureFile(t, wt, "go/internal/core/frozen_a_test.go", pinTestFileF4("go/internal/core/state.go", "storage.UpdateStateMap"))
	writeAliasFixtureFile(t, wt, "go/internal/core/frozen_b_test.go", pinTestFileF4("go/internal/core/state.go", "leafutil.Helper"))

	if _, err := BuildImportGraph(wt+"/go", "./..."); err == nil {
		t.Fatal("precondition: a malformed go.mod must make the import graph underivable")
	}

	got, err := CheckFrozenPins(wt, []string{"go/internal/core/frozen_a_test.go", "go/internal/core/frozen_b_test.go"})

	if err != nil || len(got) != 0 {
		t.Fatalf("an underivable graph must fail open: violations=%+v err=%v", got, err)
	}
}
