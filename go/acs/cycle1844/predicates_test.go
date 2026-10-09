//go:build acs

package cycle1844

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	userSpecName   = "hermetic-probe"
	userSpecJSON   = `{"name":"hermetic-probe","catalog":"on-demand","archetype":"evaluate","agent":"evolve-hermetic-probe","inputs":{"files":[".evolve/runs/cycle-{cycle}/build-report.md"]},"outputs":{"files":[".evolve/runs/cycle-{cycle}/hermetic-probe-report.md"]},"classify":{"require_sections":["Verdict"]}}`
	catalogSpecRef = "plan-review"
	unknownName    = "no-such-phase-zzz"
)

func hermeticProject(t *testing.T) string {
	t.Helper()
	repo := acsassert.RepoRoot(t)
	root := t.TempDir()
	registryRel := filepath.Join("docs", "architecture", "phase-registry.json")
	raw, err := os.ReadFile(filepath.Join(repo, registryRel))
	if err != nil {
		t.Fatalf("read phase registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "architecture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, registryRel), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(root, ".evolve", "phases", userSpecName)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "phase.json"), []byte(userSpecJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		t.Fatalf("fixture catalog does not load: %v", err)
	}
	for _, name := range []string{userSpecName, catalogSpecRef} {
		if _, ok := cat.Get(name); !ok {
			t.Fatalf("fixture catalog lacks %q: the predicate cannot probe what is not there", name)
		}
	}
	return root
}

func TestC1844_001_ResolveRunnerFindsSpecAndBuiltinPhases(t *testing.T) {
	root := hermeticProject(t)
	req := core.PhaseRequest{ProjectRoot: root}
	for _, name := range []string{catalogSpecRef, userSpecName, "tdd", "Plan-Review"} {
		runner, found, err := phasecmd.ResolveRunner(name, req)
		if err != nil || !found || runner == nil {
			t.Errorf("ResolveRunner(%q) = (%v, %v, %v), want a runnable phase", name, runner, found, err)
		}
	}
}

func TestC1844_002_ResolveRunnerRejectsUnknownWithoutError(t *testing.T) {
	root := hermeticProject(t)
	runner, found, err := phasecmd.ResolveRunner(unknownName, core.PhaseRequest{ProjectRoot: root})
	if err != nil || found || runner != nil {
		t.Errorf("ResolveRunner(%q) = (%v, %v, %v), want (nil, false, nil)", unknownName, runner, found, err)
	}
}

func TestC1844_003_CatalogPhaseNamesPartitionsBuiltinAndSpecSets(t *testing.T) {
	root := hermeticProject(t)
	builtins, specs, err := phasecmd.CatalogPhaseNames(root)
	if err != nil {
		t.Fatalf("CatalogPhaseNames: %v", err)
	}
	if !slices.Equal(builtins, registry.Names()) {
		t.Errorf("builtins = %v, want registry names %v", builtins, registry.Names())
	}
	for _, want := range []string{catalogSpecRef, userSpecName} {
		if !slices.Contains(specs, want) {
			t.Errorf("specs %v lack %q", specs, want)
		}
	}
	for _, b := range builtins {
		if slices.Contains(specs, b) {
			t.Errorf("builtin %q also listed in the spec set", b)
		}
	}
	if !slices.IsSorted(specs) {
		t.Errorf("specs %v are not sorted", specs)
	}
}

func TestC1844_004_FormatUnknownPhaseErrorNamesBothSets(t *testing.T) {
	root := hermeticProject(t)
	msg := phasecmd.FormatUnknownPhaseError("evolve phase", unknownName, root)
	for _, want := range []string{"evolve phase: unknown phase", unknownName, "known built-in:", "tdd", "known spec:", catalogSpecRef, userSpecName} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
}

func TestC1844_005_PhaseCommandUnknownNameExits10ListingBothSets(t *testing.T) {
	root := hermeticProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	rc := phasecmd.NewRunPhase(nil, nil)([]string{unknownName}, strings.NewReader(""), &stdout, &stderr)
	if rc != 10 {
		t.Errorf("exit = %d, want 10", rc)
	}
	for _, want := range []string{"unknown phase", "known built-in:", "known spec:", catalogSpecRef} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q lacks %q", stderr.String(), want)
		}
	}
}

func TestC1844_006_PhaseCommandAcceptsSpecPhaseNameBeforeAnyUnknownVerdict(t *testing.T) {
	root := hermeticProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	rc := phasecmd.NewRunPhase(nil, nil)([]string{catalogSpecRef, "--cycle", "1"}, strings.NewReader(""), &stdout, &stderr)
	if strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("spec phase %q reported unknown (exit %d): %s", catalogSpecRef, rc, stderr.String())
	}
	if rc == 10 && strings.Contains(stderr.String(), "known built-in") {
		t.Errorf("spec phase %q treated as an unknown name", catalogSpecRef)
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	repo := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = filepath.Join(repo, "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", err, out)
	}
	return bin
}

func runCompose(t *testing.T, bin, root string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"compose"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+root)
	cmd.Stdin = strings.NewReader("{}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run compose: %v", err)
	}
	return stdout.String(), stderr.String(), code
}

func TestC1844_007_ComposeAcceptsSpecPhaseInSequence(t *testing.T) {
	root := hermeticProject(t)
	bin := buildEvolve(t)
	for _, phases := range []string{catalogSpecRef + ",tdd", userSpecName + ",tdd"} {
		stdout, stderr, code := runCompose(t, bin, root, "--phases", phases, "--dry-run")
		if code != 0 {
			t.Errorf("compose --phases %s --dry-run exit = %d, want 0; stderr: %s", phases, code, stderr)
		}
		if want := "[compose] sequence: " + strings.ReplaceAll(phases, ",", " -> "); !strings.Contains(stdout, want) {
			t.Errorf("stdout %q lacks %q", stdout, want)
		}
	}
}

func TestC1844_008_ComposeUnknownPhaseExits10ListingBothSets(t *testing.T) {
	root := hermeticProject(t)
	bin := buildEvolve(t)
	_, stderr, code := runCompose(t, bin, root, "--phases", catalogSpecRef+","+unknownName, "--dry-run")
	if code != 10 {
		t.Errorf("exit = %d, want 10", code)
	}
	for _, want := range []string{"evolve compose: unknown phase", unknownName, "known built-in:", "known spec:", catalogSpecRef} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q lacks %q", stderr, want)
		}
	}
}

func TestC1844_009_ComposeStillRefusesShipWithoutShipAnywayForSpecSequences(t *testing.T) {
	root := hermeticProject(t)
	bin := buildEvolve(t)
	_, stderr, code := runCompose(t, bin, root, "--phases", catalogSpecRef+",ship", "--dry-run")
	if code != 2 {
		t.Errorf("exit = %d, want 2 (ship refused); stderr: %s", code, stderr)
	}
}

func TestC1844_010_RuntimeReferenceDocumentsSpecPhaseExecution(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	doc := filepath.Join(repo, "docs", "operations", "runtime-reference.md")
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	// acs-predicate: config-check
	body := string(raw)
	for _, want := range []string{"evolve phase plan-review --cycle", "evolve compose --phases plan-review"} {
		if !strings.Contains(body, want) {
			t.Errorf("runtime-reference.md lacks %q", want)
		}
	}
}
