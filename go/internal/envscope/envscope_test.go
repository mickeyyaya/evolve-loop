package envscope

import "testing"

func TestSkipDir(t *testing.T) {
	for name, want := range map[string]bool{
		"vendor": true, "testdata": true, ".git": true, "node_modules": true,
		".evolve": true, "ipcenv": true, "acs": true,
		"internal": false, "subagent": false,
	} {
		if got := SkipDir(name); got != want {
			t.Errorf("SkipDir(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFile(t *testing.T) {
	for path, want := range map[string]bool{
		"go/internal/subagent/recursion.go":                true,
		"go/internal/subagent/recursion_test.go":           false,
		"go/internal/subagent/README.md":                   false,
		"go/internal/flagregistry/registry_table.go":       false,
		"internal/flagregistry/registry_table.go":          false,
		"go/internal/flagregistry/other_registry_table.go": true,
	} {
		if got := File(path); got != want {
			t.Errorf("File(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestReads(t *testing.T) {
	for path, want := range map[string]bool{
		"demo.go":                                true,
		"go/internal/subagent/recursion.go":      true,
		"go/internal/subagent/recursion_test.go": false,
		"go/acs/cycle9/predicates_test.go":       false,
		"go/acs/cycle9/helper.go":                false,
		"go/internal/ipcenv/ipcenv.go":           false,
		"go/internal/x/testdata/fixture.go":      false,
		"go/vendor/dep/dep.go":                   false,
		"go/internal/acsfoo/acsfoo.go":           true,
	} {
		if got := Reads(path); got != want {
			t.Errorf("Reads(%q) = %v, want %v", path, got, want)
		}
	}
}
