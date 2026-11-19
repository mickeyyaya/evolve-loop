//go:build acs

package cycle2539

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	deliverablePkg  = "internal/deliverable"
	deliverableTest = "TestHostEffects_TheRegistryDeclaresTriagesDerivation"
	routingPkg      = "cmd/evolve"
	routingTest     = "TestRealTreeRoutingOrder_AnchoredPhasesFollowTheirAnchors"
	corePkg         = "internal/core"
	coreTest        = "TestJudgmentTeachingPhases_CoverEveryPhaseThatCanStateItsOwnVerdict"
	skipRule        = "skip-hides-error"
	registryRel     = "docs/architecture/phase-registry.json"
	corruptRegistry = "{ this is not a registry"
)

type testRun struct {
	output string
	code   int
}

func (r testRun) skipped() bool { return strings.Contains(r.output, "--- SKIP") }

func (r testRun) failed() bool { return r.code != 0 && strings.Contains(r.output, "--- FAIL") }

func (r testRun) passed() bool { return r.code == 0 && strings.Contains(r.output, "--- PASS") }

func compileTestBinary(t *testing.T, pkg string) string {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	bin := filepath.Join(t.TempDir(), filepath.Base(pkg)+".test")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", bin, "./"+pkg)
	cmd.Dir = moduleDir
	cmd.WaitDelay = 5 * time.Second
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile test binary of %s: %v\n%s", pkg, err, acsassert.OutputTail(string(out), 20))
	}
	return bin
}

func runInFixtureTree(t *testing.T, bin, pkg, testName string, files map[string]string, dirs ...string) testRun {
	t.Helper()
	fixtureRoot := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(fixtureRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range dirs {
		if err := os.MkdirAll(filepath.Join(fixtureRoot, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(fixtureRoot, "go", filepath.FromSlash(pkg))
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, "-test.run", "^"+testName+"$", "-test.v", "-test.count=1")
	cmd.Dir = cwd
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s in fixture: %v", testName, err)
	}
	return testRun{output: string(out), code: code}
}

func realRegistry(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(registryRel)))
	if err != nil {
		t.Fatalf("read real registry: %v", err)
	}
	return string(raw)
}

func requireFails(t *testing.T, label string, r testRun) {
	t.Helper()
	if r.skipped() {
		t.Errorf("%s: the test skipped instead of failing\n%s", label, acsassert.OutputTail(r.output, 12))
	}
	if !r.failed() {
		t.Errorf("%s: want a failing test (exit non-zero, --- FAIL), got exit %d\n%s", label, r.code, acsassert.OutputTail(r.output, 12))
	}
}

func TestC2539_001_DeliverableRegistryTestFailsOnCorruptRegistry(t *testing.T) {
	bin := compileTestBinary(t, deliverablePkg)

	corrupt := runInFixtureTree(t, bin, deliverablePkg, deliverableTest, map[string]string{registryRel: corruptRegistry})
	requireFails(t, "corrupt registry", corrupt)

	missing := runInFixtureTree(t, bin, deliverablePkg, deliverableTest, nil)
	requireFails(t, "missing registry", missing)
}

func TestC2539_002_DeliverableRegistryTestStillPassesOnTheRealRegistry(t *testing.T) {
	bin := compileTestBinary(t, deliverablePkg)

	valid := runInFixtureTree(t, bin, deliverablePkg, deliverableTest, map[string]string{registryRel: realRegistry(t)})

	if !valid.passed() {
		t.Errorf("a valid registry must keep the test green, got exit %d\n%s", valid.code, acsassert.OutputTail(valid.output, 12))
	}
}

func TestC2539_003_RoutingOrderTestFailsWhenTheRegistryOrSpecsCannotBeLoaded(t *testing.T) {
	bin := compileTestBinary(t, routingPkg)

	requireFails(t, "corrupt registry",
		runInFixtureTree(t, bin, routingPkg, routingTest, map[string]string{registryRel: corruptRegistry}))
	requireFails(t, "missing registry",
		runInFixtureTree(t, bin, routingPkg, routingTest, nil))
	requireFails(t, "valid registry but no user specs discovered",
		runInFixtureTree(t, bin, routingPkg, routingTest, map[string]string{registryRel: realRegistry(t)}))
}

func TestC2539_004_JudgmentCatalogTestFailsWhenThePhaseCatalogIsAbsentOrEmpty(t *testing.T) {
	bin := compileTestBinary(t, corePkg)

	requireFails(t, "phase catalog absent",
		runInFixtureTree(t, bin, corePkg, coreTest, nil))
	requireFails(t, "phase catalog empty",
		runInFixtureTree(t, bin, corePkg, coreTest, nil, ".evolve/phases"))
}

func TestC2539_005_NoSkipHidesErrorFindingRemainsForTheThreeTests(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit config: %v", err)
	}
	targets := []struct{ pkg, test string }{
		{deliverablePkg, deliverableTest},
		{routingPkg, routingTest},
		{corePkg, coreTest},
	}
	for _, target := range targets {
		dir := filepath.Join(moduleDir, filepath.FromSlash(target.pkg))

		found, err := testaudit.Analyze(dir, cfg)
		if err != nil {
			t.Fatalf("analyze %s: %v", target.pkg, err)
		}
		for _, f := range found {
			if f.Test == target.test && f.Rule == skipRule {
				t.Errorf("%s: %s still reports %s", target.pkg, target.test, skipRule)
			}
		}
		if row := baselineRow(t, dir, target.test); row != "" {
			t.Errorf("%s: baseline still holds %s", target.pkg, row)
		}
	}
}

func baselineRow(t *testing.T, pkgDir, test string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(pkgDir, "testdata", "testaudit-baseline.json"))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	var shard struct {
		Rows []testaudit.Finding `json:"rows"`
	}
	if err := json.Unmarshal(raw, &shard); err != nil {
		t.Fatalf("parse baseline of %s: %v", pkgDir, err)
	}
	for _, row := range shard.Rows {
		if row.Test == test && row.Rule == skipRule {
			return row.Package + " " + row.Test + " " + row.Rule
		}
	}
	return ""
}
