//go:build acs

// Package cycle1141 materialises the cycle-1141 acceptance criteria for the
// three fleet-scoped SSOT/gate tasks pinned to this lane:
//
//   - artifact-name-ssot-retro-backfill      → predicates 001-003
//   - required-roles-ssot                    → predicates 004-005
//   - cycle-docs-floor-architecture-changes  → predicates 006-008
//
// Predicate strategy. Tasks 1 and 2 are SSOT *refactors*: deriving a filename
// from phasecontract.For(phase).ArtifactName produces the SAME string the frozen
// literal produced, so no single behavioural assertion can distinguish "derived"
// from "re-typed". The honest materialisation is therefore a PAIR per caller:
//
//	(a) a behavioural assertion that EXECUTES the caller and compares its output
//	    against the registry value computed at test time (so if the registry ever
//	    moves, the caller must move with it), and
//	(b) an anti-freeze assertion that the raw literal is ABSENT from the caller's
//	    source.
//
// (b) alone would be the banned degenerate form — but it is inverted here: it
// demands the magic string be REMOVED, which cannot be satisfied by pasting text
// in, and can only be satisfied while (a) still passes by actually deriving the
// value. The load-bearing half is always the executed one.
//
// Task 3 is new machinery, so its predicates are pure behaviour: a table-driven
// exercise of the gate's decision function plus a policy round-trip proving the
// stage is config-injected and a wiring proof that the gate is not inert.
//
// Root resolution: acsassert.RepoRoot(t) is the worktree (where Builder writes,
// per worktree isolation). Source-path assertions resolve under it; behavioural
// assertions link the packages directly, so they exercise the worktree's code by
// construction.
package cycle1141

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// acsRepoRoot is the worktree root every source-path assertion resolves under.
func acsRepoRoot(t *testing.T) string {
	t.Helper()
	return acsassert.RepoRoot(t)
}

// acsSubprocess runs a command with cwd = <repoRoot>/go (acsassert.SubprocessOutput
// cannot set a working directory, and both call sites need one).
func acsSubprocess(t *testing.T, name string, args ...string) (stdout, stderr string, code int, err error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Join(acsRepoRoot(t), "go")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if ee, ok := runErr.(*exec.ExitError); ok {
		return out.String(), errBuf.String(), ee.ExitCode(), nil
	}
	if runErr != nil {
		return out.String(), errBuf.String(), -1, runErr
	}
	return out.String(), errBuf.String(), 0, nil
}

// ---------------------------------------------------------------------------
// Task 1: artifact-name-ssot-retro-backfill
// ---------------------------------------------------------------------------

// TestC1141_001_dossier_fail_defect_derives_audit_artifact_name exercises the
// real dossier.Build FAIL path and asserts the synthesized defect points at the
// registry's audit artifact name, then asserts build.go no longer carries the
// raw literal. Executed half first: Build is CALLED and its returned Defect is
// the assertion target, so a source-only edit cannot satisfy this predicate.
func TestC1141_001_dossier_fail_defect_derives_audit_artifact_name(t *testing.T) {
	auditContract, ok := phasecontract.For("audit")
	if !ok {
		t.Fatalf("registry has no audit contract — cannot derive expected artifact name")
	}
	want := auditContract.ArtifactName
	if want == "" {
		t.Fatalf("registry audit contract has empty ArtifactName")
	}

	d, err := dossier.Build(1141, dossier.BuildOpts{
		WorkspacePath: t.TempDir(),
		Goal:          "cycle-1141 acs probe",
		FinalVerdict:  "FAIL",
	})
	if err != nil {
		t.Fatalf("dossier.Build returned error: %v", err)
	}
	if len(d.Defects) == 0 {
		t.Fatalf("FAIL verdict produced no defects — cannot verify artifact-name derivation")
	}
	if !strings.Contains(d.Defects[0].Summary, want) {
		t.Errorf("defect summary %q does not reference the registry audit artifact %q",
			d.Defects[0].Summary, want)
	}

	// Anti-freeze: the derived value must come from the registry, not a re-typed
	// literal sitting in the caller.
	src := filepath.Join(acsRepoRoot(t), "go", "internal", "dossier", "build.go")
	assertNoRawLiteral(t, src, `"audit-report.md"`)
	assertReferencesRegistry(t, src)
}

// TestC1141_002_gc_discover_markers_track_registry_artifacts exercises the real
// gc.Discover over a synthetic .evolve tree: EVERY artifact the registry declares
// required must, on its own, evidence a run dir. This is the live-derivation
// guarantee — if the registry's required-artifact set ever grows, a frozen
// runMarkers list stops discovering the new marker and this predicate fails.
//
// Includes the negative axis: a directory holding only an unrelated file must NOT
// be discovered, so a "discover everything" no-op cannot pass.
func TestC1141_002_gc_discover_markers_track_registry_artifacts(t *testing.T) {
	required := phasecontract.RequiredArtifacts()
	if len(required) == 0 {
		t.Fatalf("registry RequiredArtifacts() is empty — nothing to bind")
	}

	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	runsDir := filepath.Join(evolveDir, "runs")
	for i, name := range required {
		dir := filepath.Join(runsDir, "run-"+itoa(i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write marker %s: %v", name, err)
		}
	}
	// Negative control: no marker file at all.
	decoy := filepath.Join(runsDir, "run-decoy")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decoy, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	found, err := gc.Discover(evolveDir, gc.DiscoverOptions{})
	if err != nil {
		t.Fatalf("gc.Discover returned error: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range found {
		seen[filepath.Base(r.Path)] = true
	}
	for i, name := range required {
		if !seen["run-"+itoa(i)] {
			t.Errorf("run dir evidenced only by registry artifact %q was NOT discovered — marker set is not derived from the registry", name)
		}
	}
	if seen["run-decoy"] {
		t.Errorf("run-decoy (no marker file) was discovered — Discover is not discriminating")
	}

	src := filepath.Join(acsRepoRoot(t), "go", "internal", "gc", "discover.go")
	for _, lit := range []string{`"scout-report.md"`, `"build-report.md"`, `"audit-report.md"`} {
		assertNoRawLiteral(t, src, lit)
	}
	assertReferencesRegistry(t, src)
}

// TestC1141_003_lanescope_scout_report_name_derived covers the third
// representative caller. core/lanescope.go exposes no exported entry point, so
// the executed half runs the package's OWN behavioural tests for the scout-report
// read path as a subprocess and asserts exit 0 — the refactor must preserve real
// behaviour — paired with the anti-freeze literal-absence check.
func TestC1141_003_lanescope_scout_report_name_derived(t *testing.T) {
	src := filepath.Join(acsRepoRoot(t), "go", "internal", "core", "lanescope.go")
	assertNoRawLiteral(t, src, `"scout-report.md"`)
	assertReferencesRegistry(t, src)

	// Executed half: the scout-report read/normalize path must still work.
	stdout, stderr, code, err := acsSubprocess(t, "go", "test", "-count=1",
		"-run", "TestNormalizeScoutGoalHash", "./internal/core/")
	if err != nil {
		t.Fatalf("running core lanescope tests: %v", err)
	}
	if code != 0 {
		t.Errorf("go test -run TestNormalizeScoutGoalHash ./internal/core/ exited %d after the SSOT refactor\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}
}

// ---------------------------------------------------------------------------
// Task 2: required-roles-ssot
// ---------------------------------------------------------------------------

// TestC1141_004_routing_mandatory_no_regression is the no-regression half of the
// audit: whatever the task decides (derive or document), the LIVE routing config
// must still name every registry-required phase plus the non-report "ship" phase,
// and must not acquire junk phases the registry does not know.
func TestC1141_004_routing_mandatory_no_regression(t *testing.T) {
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "absent-registry.json"), map[string]string{})
	if len(cfg.Mandatory) == 0 {
		t.Fatalf("routing config Mandatory is empty — routing completeness floor lost")
	}
	have := map[string]bool{}
	for _, p := range cfg.Mandatory {
		have[p] = true
	}
	for _, role := range phasecontract.RequiredRoles() {
		phase := phaseForRole(t, role)
		if !have[phase] {
			t.Errorf("registry-required role %q (phase %q) is missing from routing Mandatory %v",
				role, phase, cfg.Mandatory)
		}
	}
	if !have["ship"] {
		t.Errorf(`routing Mandatory %v dropped "ship" — the routing-only phase the registry does not cover`, cfg.Mandatory)
	}
	for _, p := range cfg.Mandatory {
		if _, ok := phasecontract.For(p); !ok {
			t.Errorf("routing Mandatory names %q which is not a registered phase — vocabulary drift", p)
		}
	}
}

// TestC1141_005_mandatory_divergence_derived_or_documented is the audit-outcome
// predicate: the Mandatory declaration site must record a DECISION. Either it
// derives from phasecontract (an import plus a registry reference in config.go),
// or it carries an explicit comment naming the registry and saying why the two
// vocabularies stay separate. A silent unchanged literal — the pre-cycle state —
// fails.
func TestC1141_005_mandatory_divergence_derived_or_documented(t *testing.T) {
	src := filepath.Join(acsRepoRoot(t), "go", "internal", "config", "config.go")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	text := string(b)

	derived := strings.Contains(text, "phasecontract")
	documented := hasDivergenceComment(text)
	if !derived && !documented {
		t.Errorf("config.go Mandatory neither derives from phasecontract nor carries an explicit divergence comment naming the registry — the audit produced no recorded decision")
	}
}

// ---------------------------------------------------------------------------
// Task 3: cycle-docs-floor-architecture-changes
// ---------------------------------------------------------------------------

// TestC1141_006_docsfloor_warns_on_undocumented_architecture_change is the crux:
// an architecture-labeled change touching zero docs/ADR files must WARN.
func TestC1141_006_docsfloor_warns_on_undocumented_architecture_change(t *testing.T) {
	v := docsfloor.Evaluate(
		docsfloor.Config{Stage: "enforce"},
		docsfloor.Input{
			ArchitectureLabeled: true,
			ChangedFiles:        []string{"go/internal/core/cyclerun.go", "go/internal/policy/policy.go"},
		},
	)
	if v.Status != docsfloor.StatusWarn {
		t.Errorf("architecture-labeled change with no docs touch: got status %q, want %q",
			v.Status, docsfloor.StatusWarn)
	}
	if strings.TrimSpace(v.Reason) == "" {
		t.Errorf("WARN verdict carries an empty Reason — an unexplained warning is unactionable")
	}
}

// TestC1141_007_docsfloor_decision_table covers the remaining rows, including the
// negative and edge axes: a documented architecture change PASSES, a
// non-architecture change is SKIPPED entirely, an off stage never fires, and an
// empty change set is not judged.
func TestC1141_007_docsfloor_decision_table(t *testing.T) {
	cases := []struct {
		name string
		cfg  docsfloor.Config
		in   docsfloor.Input
		want string
	}{
		{
			name: "architecture change touching docs passes",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/core/cyclerun.go", "docs/architecture/adr-0077-docs-floor.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "architecture change touching any docs file passes",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/policy/policy.go", "docs/operations/operating-policy.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "non-architecture change is skipped",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: false, ChangedFiles: []string{
				"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "stage off never fires",
			cfg:  docsfloor.Config{Stage: "off"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "empty change set is not judged",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: nil},
			want: docsfloor.StatusSkip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docsfloor.Evaluate(tc.cfg, tc.in)
			if got.Status != tc.want {
				t.Errorf("got status %q, want %q (reason %q)", got.Status, tc.want, got.Reason)
			}
		})
	}
}

// TestC1141_008_docsfloor_config_injected_and_wired proves the gate is neither
// hardcoded nor inert: the stage round-trips through a real .evolve/policy.json
// read (config-injected, SpineFloor-style), the compiled default is enforce when
// the block is absent, and at least one PRODUCTION file outside the gate's own
// package calls it.
func TestC1141_008_docsfloor_config_injected_and_wired(t *testing.T) {
	// (a) compiled default when the policy block is absent.
	dir := t.TempDir()
	bare := filepath.Join(dir, "bare-policy.json")
	if err := os.WriteFile(bare, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write bare policy: %v", err)
	}
	p, err := policy.Load(bare)
	if err != nil {
		t.Fatalf("policy.Load(bare): %v", err)
	}
	if got := p.DocsFloorConfig().Stage; got != "enforce" {
		t.Errorf("absent docs_floor block: compiled default stage = %q, want %q", got, "enforce")
	}

	// (b) policy.json override actually reaches the gate.
	overridden := filepath.Join(dir, "override-policy.json")
	if err := os.WriteFile(overridden, []byte(`{"docs_floor":{"stage":"off"}}`), 0o644); err != nil {
		t.Fatalf("write override policy: %v", err)
	}
	p2, err := policy.Load(overridden)
	if err != nil {
		t.Fatalf("policy.Load(override): %v", err)
	}
	if got := p2.DocsFloorConfig().Stage; got != "off" {
		t.Errorf("policy.json docs_floor.stage override did not reach the gate: got %q, want %q", got, "off")
	}

	// (c) wiring proof: a real caller outside internal/docsfloor invokes it.
	if !hasProductionCaller(t, "docsfloor.Evaluate") {
		t.Errorf("no production (non-test) file outside internal/docsfloor calls docsfloor.Evaluate — the gate is inert")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// docPrefixes are the paths that count as a documentation touch. Kept here only
// to document the intent of predicates 006/007; the gate owns the real list.
var docPrefixes = []string{"docs/"}

// assertNoRawLiteral fails when the caller still carries a re-typed report-name
// literal. This is the anti-freeze half of an SSOT pair — never load-bearing on
// its own, and inverted (absence, not presence) so it cannot be satisfied by
// adding a magic string.
func assertNoRawLiteral(t *testing.T, path, literal string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(b), literal) {
		t.Errorf("%s still hardcodes %s — not derived from phasecontract", path, literal)
	}
}

// assertReferencesRegistry fails when the caller does not reach the registry at
// all: removing the literal without deriving from phasecontract (e.g. moving it
// to a local const) would otherwise slip through.
func assertReferencesRegistry(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(b), "phasecontract") {
		t.Errorf("%s does not reference phasecontract — the name is not derived from the registry SSOT", path)
	}
}

// hasDivergenceComment reports whether config.go carries an explicit,
// registry-naming comment justifying a separate Mandatory vocabulary.
func hasDivergenceComment(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		low := strings.ToLower(trimmed)
		if !strings.Contains(low, "phasecontract") && !strings.Contains(low, "requiredroles") {
			continue
		}
		if strings.Contains(low, "mandatory") || strings.Contains(low, "diverg") || strings.Contains(low, "deliberately") {
			return true
		}
	}
	return false
}

// phaseForRole maps a registry AgentName back to its phase key.
func phaseForRole(t *testing.T, role string) string {
	t.Helper()
	for _, c := range phasecontract.Contracts() {
		if c.AgentName == role {
			return c.Phase
		}
	}
	t.Fatalf("no registry contract has AgentName %q", role)
	return ""
}

// hasProductionCaller greps the Go tree for a non-test call site outside the
// gate's own package — the anti-inert wiring proof.
func hasProductionCaller(t *testing.T, call string) bool {
	t.Helper()
	stdout, _, _, err := acsSubprocess(t, "grep", "-rl", "--include=*.go", call,
		filepath.Join(acsRepoRoot(t), "go", "internal"),
		filepath.Join(acsRepoRoot(t), "go", "cmd"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "_test.go") {
			continue
		}
		if strings.Contains(line, string(filepath.Separator)+"docsfloor"+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	out := ""
	for i > 0 {
		out = string(rune('0'+i%10)) + out
		i /= 10
	}
	return out
}

var _ = docPrefixes
