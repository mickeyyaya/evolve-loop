package testaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseline_TargetsAndErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "file.txt"), "not a dir")
	mustWrite(t, filepath.Join(root, "pkg", "bad", "x_test.go"), "package bad\nfunc {")
	mustWrite(t, filepath.Join(root, "pkg", "clean", "clean_test.go"), "package clean\nimport \"testing\"\nfunc TestOK(t *testing.T) { t.Fatal(\"x\") }\n")
	mustWrite(t, filepath.Join(root, "pkg", "weak", "x_test.go"), redTest)

	for _, bad := range []string{"", filepath.Join(root, "pkg"), "..", "../out", "pkg/missing", "pkg/file.txt", "pkg/bad"} {
		if err := Rebaseline(root, bad, Config{}); err == nil {
			t.Errorf("Rebaseline(%q) = nil, want error", bad)
		}
	}
	if err := Rebaseline(root, "pkg/clean", Config{}); err != nil {
		t.Errorf("Rebaseline(clean) = %v", err)
	}
	if err := Rebaseline(root, "pkg/weak", Config{}); err != nil {
		t.Fatalf("Rebaseline(weak) = %v", err)
	}
	if err := Rebaseline(root, "pkg/weak", Config{}); err != nil {
		t.Errorf("Rebaseline(weak unchanged) = %v", err)
	}
	mustWrite(t, filepath.Join(root, "pkg", "weak", "y_test.go"), "package fx\nimport \"testing\"\nfunc TestSecond(t *testing.T) { _ = 1 }\n")
	if err := Rebaseline(root, "pkg/weak", Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestSecond") {
		t.Errorf("Rebaseline(weak growth) = %v, want TestSecond error", err)
	}
}

func TestRebaseline_CentralBaselinesAndErrorPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
	mustWrite(t, centralPath, `{"packages":["pkg/a","pkg/b"],"rows":[{"package":"pkg/a","test":"TestNothing","rule":"no-assertion"},{"package":"pkg/b","test":"TestNothing","rule":"no-assertion"}]}`)
	mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "b", "b_test.go"), redTest)

	if err := Rebaseline(root, "pkg/a", Config{}); err != nil {
		t.Errorf("central a unchanged: %v", err)
	}
	if err := Rebaseline(root, "pkg/b", Config{}); err != nil {
		t.Errorf("central b unchanged: %v", err)
	}
	mustWrite(t, filepath.Join(root, "pkg", "a", "a2_test.go"), "package fx\nimport \"testing\"\nfunc TestGrow(t *testing.T) { _ = 1 }\n")
	if err := Rebaseline(root, "pkg/a", Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestGrow") {
		t.Errorf("central growth = %v, want TestGrow error", err)
	}
	if err := CheckTree(root, Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestGrow") {
		t.Errorf("CheckTree central drift = %v, want TestGrow error", err)
	}

	mustWrite(t, filepath.Join(root, "pkg", "a", shardRelPath), "{")
	if err := Rebaseline(root, "pkg/badjson", Config{}); err == nil {
		t.Error("Rebaseline missing pkg/badjson succeeded")
	}
	if err := CheckTree(root, Config{}); err == nil {
		t.Error("CheckTree accepted malformed local shard")
	}
}

func TestRebaseline_RefusesASwapAtEqualCount(t *testing.T) {
	t.Parallel()
	clean := "package fx\nimport \"testing\"\nfunc TestNothing(t *testing.T) { t.Fatal(\"ok\") }\n"
	swap := "package fx\nimport \"testing\"\nfunc TestSwapped(t *testing.T) { _ = 1 }\n"

	t.Run("local", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), redTest)
		if err := Rebaseline(root, "pkg/a", Config{}); err != nil {
			t.Fatalf("seed local: %v", err)
		}
		mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), clean)
		mustWrite(t, filepath.Join(root, "pkg", "a", "b_test.go"), swap)
		if err := Rebaseline(root, "pkg/a", Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestSwapped") {
			t.Errorf("local swap = %v, want refusal naming TestSwapped", err)
		}
	})

	t.Run("central", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
		mustWrite(t, centralPath, `{"packages":["pkg/a"],"rows":[{"package":"pkg/a","test":"TestNothing","rule":"no-assertion"}]}`)
		mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), clean)
		mustWrite(t, filepath.Join(root, "pkg", "a", "b_test.go"), swap)
		if err := Rebaseline(root, "pkg/a", Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestSwapped") {
			t.Errorf("central swap = %v, want refusal naming TestSwapped", err)
		}
	})

	t.Run("central-zero-to-n", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
		mustWrite(t, centralPath, `{"packages":["pkg/a"],"rows":[]}`)
		mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), swap)
		if err := Rebaseline(root, "pkg/a", Config{}); err == nil || !strings.Contains(fmt.Sprint(err), "TestSwapped") {
			t.Errorf("central 0->N = %v, want refusal naming TestSwapped", err)
		}
	})
}

func TestCheckTree_MalformedInputsAndRelativeHelpers(t *testing.T) {
	t.Parallel()
	if got := relSlash("/a", "b"); got != "b" {
		t.Errorf("relSlash fallback = %q, want b", got)
	}
	if got := resolvePkgDir(".", "pkg/x"); got != filepath.FromSlash("pkg/x") {
		t.Errorf("resolvePkgDir(.) = %q", got)
	}

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "ok", "ok_test.go"), "package ok\nimport \"testing\"\nfunc TestOK(t *testing.T) { t.Fatal(\"x\") }\n")
	mustWrite(t, filepath.Join(root, "pkg", "ok", shardRelPath), "{")
	if err := Rebaseline(root, "pkg/ok", Config{}); err == nil {
		t.Error("Rebaseline accepted malformed local shard")
	}

	badRoot := t.TempDir()
	mustWrite(t, filepath.Join(badRoot, "pkg", "suite", "bad.go"), "package suite\nfunc {")
	cfg := Config{Fakes: []Fake{{Tool: "git", SuiteDir: "pkg/suite", RealSide: "x"}}}
	if _, err := packageFindings(badRoot, "pkg/suite", cfg); err == nil {
		t.Error("analyzeSingle accepted broken suite dir")
	}
	if err := CheckTree(badRoot, cfg); err == nil {
		t.Error("CheckTree accepted broken suite dir")
	}
	suiteHitRoot := t.TempDir()
	mustWrite(t, filepath.Join(suiteHitRoot, "pkg", "suite", "ok_test.go"), "package suite\nimport \"testing\"\nfunc TestOK(t *testing.T) { t.Fatal(\"x\") }\n")
	if got, err := packageFindings(suiteHitRoot, "pkg/suite", cfg); err != nil || len(got) != 1 || got[0].Rule != ruleUnverifiedFake {
		t.Errorf("analyzeSingle unverified fake = %v, %v", got, err)
	}
	if _, err := collectTreeBaselines(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("collectTreeBaselines accepted missing root")
	}

	badCentral := t.TempDir()
	mustWrite(t, filepath.Join(badCentral, "pkg", "x", "x_test.go"), redTest)
	mustWrite(t, filepath.Join(badCentral, filepath.FromSlash(centralBaselinesRelPath)), "{")
	if err := Rebaseline(badCentral, "pkg/x", Config{}); err == nil {
		t.Error("Rebaseline accepted malformed central file")
	}
	if err := CheckTree(badCentral, Config{}); err == nil {
		t.Error("CheckTree accepted malformed central file")
	}

	dirCentral := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dirCentral, filepath.FromSlash(centralBaselinesRelPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readCentralBaselines(filepath.Join(dirCentral, filepath.FromSlash(centralBaselinesRelPath))); err == nil {
		t.Error("readCentralBaselines accepted a directory")
	}
	if _, err := findModuleRoot(t.TempDir()); err == nil {
		t.Error("findModuleRoot accepted a directory without go.mod")
	}
}
