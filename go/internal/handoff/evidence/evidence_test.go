package evidence_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
)

const fixtureCycle = 1950

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go/internal/core/blocker.go", "package core\n\nfunc Blocker() {}\n")
	writeFile(t, root, "go/internal/core/blocker_test.go", "package core\n\nimport \"testing\"\n\nfunc TestBlockerTrips(t *testing.T) {}\n\nfunc FuzzBlockerInput(f *testing.F) {}\n\nfunc helperNotATest(t *testing.T) {}\n")
	writeFile(t, root, "go/internal/other/other_test.go", "package other\n\nimport \"testing\"\n\nfunc TestOtherOnly(t *testing.T) {}\n")
	writeFile(t, root, "go/internal/broken/broken_test.go", "package broken\n\nfunc TestBroken(\n")
	writeFile(t, root, "go/acs/cycle1950/predicates_test.go", "//go:build acs\n\npackage cycle1950\n\nimport \"testing\"\n\nfunc TestC1950_001_Declared(t *testing.T) {}\n")
	writeFile(t, root, "go/acs/cycle1949/predicates_test.go", "//go:build acs\n\npackage cycle1949\n\nimport \"testing\"\n\nfunc TestC1949_001_OtherCycle(t *testing.T) {}\n")
	writeFile(t, root, "empty.txt", "")
	writeFile(t, root, "no-trailing-newline.txt", "one\ntwo")
	return root
}

func TestResolve(t *testing.T) {
	root := fixtureRoot(t)
	src := evidence.Source{
		Root:     root,
		Cycle:    fixtureCycle,
		Commands: []string{"go test ./internal/core", "go vet ./..."},
	}
	cases := []struct {
		name     string
		kind     string
		ref      string
		resolves bool
	}{
		{"path first line", "path", "go/internal/core/blocker.go:1", true},
		{"path last line", "path", "go/internal/core/blocker.go:3", true},
		{"path last line of file without trailing newline", "path", "no-trailing-newline.txt:2", true},
		{"path line past end of file", "path", "go/internal/core/blocker.go:4", false},
		{"path line far past end of file", "path", "go/internal/core/blocker.go:9999", false},
		{"path line past end of file without trailing newline", "path", "no-trailing-newline.txt:3", false},
		{"path line one of empty file", "path", "empty.txt:1", false},
		{"path line zero", "path", "go/internal/core/blocker.go:0", false},
		{"path negative line", "path", "go/internal/core/blocker.go:-1", false},
		{"path without line", "path", "go/internal/core/blocker.go", false},
		{"path with non-numeric line", "path", "go/internal/core/blocker.go:abc", false},
		{"path with empty line", "path", "go/internal/core/blocker.go:", false},
		{"path missing file", "path", "go/internal/core/gone.go:1", false},
		{"path is a directory", "path", "go/internal/core:1", false},
		{"path escapes root", "path", "../outside.go:1", false},
		{"path is absolute", "path", filepath.Join(root, "go/internal/core/blocker.go") + ":1", false},
		{"empty ref", "path", "", false},

		{"test declared by bare name", "test", "TestBlockerTrips", true},
		{"fuzz declared by bare name", "test", "FuzzBlockerInput", true},
		{"test declared in another package", "test", "TestOtherOnly", true},
		{"test declared in the cited file", "test", "go/internal/core/blocker_test.go:TestBlockerTrips", true},
		{"test no _test.go file declares", "test", "TestNobodyDeclaresThis", false},
		{"test cited in a file that does not declare it", "test", "go/internal/other/other_test.go:TestBlockerTrips", false},
		{"test cited in a file that does not exist", "test", "go/internal/core/gone_test.go:TestBlockerTrips", false},
		{"helper without a Test prefix", "test", "helperNotATest", false},
		{"test name only in a file that does not parse", "test", "TestBroken", false},
		{"test name only in a non-test file", "test", "Blocker", false},
		{"test cited in a non-test file", "test", "go/internal/core/blocker.go:TestBlockerTrips", false},
		{"test cited in a file outside the root", "test", "../outside_test.go:TestBlockerTrips", false},
		{"test with empty ref", "test", "", false},
		{"test with empty name after file", "test", "go/internal/core/blocker_test.go:", false},

		{"predicate declared by the cycle package", "predicate", "go/acs/cycle1950/predicates_test.go:TestC1950_001_Declared", true},
		{"predicate declared, bare name", "predicate", "TestC1950_001_Declared", true},
		{"predicate the cycle package does not declare", "predicate", "go/acs/cycle1950/predicates_test.go:TestC1950_002_Missing", false},
		{"predicate declared only by another cycle package", "predicate", "go/acs/cycle1949/predicates_test.go:TestC1949_001_OtherCycle", false},
		{"predicate bare name declared only by another cycle package", "predicate", "TestC1949_001_OtherCycle", false},
		{"predicate declared only outside any cycle package", "predicate", "TestBlockerTrips", false},
		{"predicate cites the right name in the wrong file", "predicate", "go/internal/core/blocker_test.go:TestBlockerTrips", false},
		{"predicate with empty ref", "predicate", "", false},

		{"command result recorded", "command", "go test ./internal/core", true},
		{"second command result recorded", "command", "go vet ./...", true},
		{"command result absent", "command", "go test ./internal/policy", false},
		{"command result differs by one character", "command", "go test ./internal/core ", false},
		{"command with empty ref", "command", "", false},

		{"unknown kind", "screenshot", "go/internal/core/blocker.go:1", false},
		{"empty kind", "", "go/internal/core/blocker.go:1", false},
		{"kind is case sensitive", "Path", "go/internal/core/blocker.go:1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := evidence.Resolve(src, handoff.Evidence{Kind: tc.kind, Ref: tc.ref})
			if tc.resolves {
				if err != nil {
					t.Fatalf("Resolve(%s %q) = %v, want nil", tc.kind, tc.ref, err)
				}
				return
			}
			if !errors.Is(err, evidence.ErrUnresolved) {
				t.Fatalf("Resolve(%s %q) = %v, want ErrUnresolved", tc.kind, tc.ref, err)
			}
		})
	}
}

func TestResolve_UnresolvedErrorNamesTheEvidence(t *testing.T) {
	src := evidence.Source{Root: fixtureRoot(t), Cycle: fixtureCycle}
	err := evidence.Resolve(src, handoff.Evidence{Kind: "test", Ref: "TestNobodyDeclaresThis"})
	if err == nil || !stringsContainAll(err.Error(), "test", "TestNobodyDeclaresThis") {
		t.Fatalf("error must name the kind and the ref, got %v", err)
	}
}

func TestResolve_PredicateNeedsTheCyclePackage(t *testing.T) {
	root := fixtureRoot(t)
	ev := handoff.Evidence{Kind: "predicate", Ref: "go/acs/cycle1950/predicates_test.go:TestC1950_001_Declared"}
	if err := evidence.Resolve(evidence.Source{Root: root, Cycle: fixtureCycle}, ev); err != nil {
		t.Fatalf("declared predicate of the current cycle: %v", err)
	}
	if err := evidence.Resolve(evidence.Source{Root: root, Cycle: 1949}, ev); !errors.Is(err, evidence.ErrUnresolved) {
		t.Fatalf("same predicate against another cycle = %v, want ErrUnresolved", err)
	}
	if err := evidence.Resolve(evidence.Source{Root: root, Cycle: 7777}, ev); !errors.Is(err, evidence.ErrUnresolved) {
		t.Fatalf("cycle without a package = %v, want ErrUnresolved", err)
	}
}

func TestResolve_RootWithoutGoTreeResolvesNothing(t *testing.T) {
	src := evidence.Source{Root: filepath.Join(t.TempDir(), "absent"), Cycle: fixtureCycle}
	for _, ev := range []handoff.Evidence{
		{Kind: "test", Ref: "TestBlockerTrips"},
		{Kind: "predicate", Ref: "TestC1950_001_Declared"},
		{Kind: "path", Ref: "go/internal/core/blocker.go:1"},
		{Kind: "command", Ref: "go test ./internal/core"},
	} {
		if err := evidence.Resolve(src, ev); !errors.Is(err, evidence.ErrUnresolved) {
			t.Errorf("Resolve(%s %q) = %v, want ErrUnresolved", ev.Kind, ev.Ref, err)
		}
	}
}

func stringsContainAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
