package phasecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createFixtureProject writes a minimal registry so collision checks against
// built-ins are exercised, and returns the project root.
func createFixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{"phases":[{"name":"scout"},{"name":"build"},{"name":"audit"},{"name":"ship"}]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

const validCreateSpec = `{
  "name": "threat-model", "kind": "llm", "optional": true, "archetype": "evaluate",
  "description": "Lightweight per-change STRIDE pass.",
  "when_to_use": "security-sensitive diffs (auth, input handling, crypto).",
  "categories": ["security"],
  "outputs": { "files": ["threat-model-report.md"] },
  "classify": { "require_sections": ["Threats", "Mitigations"], "verdict_on_pass": "PASS" }
}`

// envelope mirrors the machine-parseable create contract.
type createEnvelope struct {
	OK               bool     `json:"ok"`
	Phase            string   `json:"phase"`
	Artifact         string   `json:"artifact,omitempty"`
	RequiredSections []string `json:"required_sections,omitempty"`
	EmitsVerdict     bool     `json:"emits_verdict,omitempty"`
	PhaseJSON        string   `json:"phase_json,omitempty"`
	Persona          string   `json:"persona,omitempty"`
	InventoryRebuilt bool     `json:"inventory_rebuilt,omitempty"`
	Errors           []string `json:"errors,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Hint             string   `json:"hint,omitempty"`
}

func parseEnvelope(t *testing.T, out string) createEnvelope {
	t.Helper()
	var env createEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not a single JSON envelope: %v\nstdout=%q", err, out)
	}
	return env
}

func runCreate(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := RunPhases(append([]string{"create"}, args...), strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestPhasesCreate_HappyPathFromStdin(t *testing.T) {
	root := createFixtureProject(t)
	persona := filepath.Join(root, "persona.md")
	if err := os.WriteFile(persona, []byte("You red-team the diff with STRIDE.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errb := runCreate(t, validCreateSpec, "--spec", "-", "--persona", persona)
	if code != 0 {
		t.Fatalf("exit = %d (stderr=%q stdout=%q)", code, errb, out)
	}
	env := parseEnvelope(t, out)
	if !env.OK || env.Phase != "threat-model" {
		t.Fatalf("envelope = %+v", env)
	}
	if env.Artifact != "threat-model-report.md" || !env.EmitsVerdict {
		t.Errorf("derived contract wrong: %+v", env)
	}
	if !env.InventoryRebuilt {
		t.Error("create must force-rebuild the phase inventory")
	}

	// Files on disk: phase.json under the default root, persona under agents/.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases", "threat-model", "phase.json")); err != nil {
		t.Errorf("phase.json not written: %v", err)
	}
	personaPath := filepath.Join(root, "agents", "evolve-threat-model.md")
	body, err := os.ReadFile(personaPath)
	if err != nil {
		t.Fatalf("persona not written: %v", err)
	}
	if !strings.Contains(string(body), "STRIDE") {
		t.Errorf("persona body lost: %q", body)
	}

	// Inventory actually contains the phase.
	inv, err := os.ReadFile(filepath.Join(root, ".evolve", "phase-inventory.json"))
	if err != nil {
		t.Fatalf("inventory missing: %v", err)
	}
	if !strings.Contains(string(inv), `"threat-model"`) {
		t.Error("inventory does not list the created phase")
	}
}

func TestPhasesCreate_FloorViolationWritesNothing(t *testing.T) {
	root := createFixtureProject(t)
	bad := `{"name": "sneaky", "optional": false}`

	code, out, _ := runCreate(t, bad, "--spec", "-")
	if code != 2 {
		t.Fatalf("floor violation must exit 2; got %d", code)
	}
	env := parseEnvelope(t, out)
	if env.OK {
		t.Error("envelope must be ok:false")
	}
	joined := strings.Join(env.Errors, " ")
	if !strings.Contains(joined, "optional") {
		t.Errorf("errors must name the floor violation: %v", env.Errors)
	}
	if env.Hint == "" {
		t.Error("envelope must carry a self-correction hint")
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases", "sneaky")); !os.IsNotExist(err) {
		t.Error("no files may be written on validation failure")
	}
}

func TestPhasesCreate_CollisionWithBuiltinRefused(t *testing.T) {
	createFixtureProject(t)
	spec := `{"name": "audit", "optional": true}`

	code, out, _ := runCreate(t, spec, "--spec", "-")
	if code != 2 {
		t.Fatalf("builtin collision must exit 2; got %d", code)
	}
	env := parseEnvelope(t, out)
	if env.OK || !strings.Contains(strings.Join(env.Errors, " "), "built-in") {
		t.Errorf("envelope = %+v", env)
	}
}

func TestPhasesCreate_CollisionWithExistingUserPhase(t *testing.T) {
	root := createFixtureProject(t)
	writeUserPhase(t, root, "threat-model", `{"name":"threat-model","optional":true}`)

	code, out, _ := runCreate(t, validCreateSpec, "--spec", "-")
	if code != 2 {
		t.Fatalf("user-phase collision must exit 2; got %d", code)
	}
	if env := parseEnvelope(t, out); env.OK {
		t.Error("envelope must be ok:false")
	}
}

func TestPhasesCreate_RefusesPersonaOverwrite(t *testing.T) {
	root := createFixtureProject(t)
	agentsDir := filepath.Join(root, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "evolve-threat-model.md"), []byte("existing persona"), 0o644); err != nil {
		t.Fatal(err)
	}
	persona := filepath.Join(root, "p.md")
	if err := os.WriteFile(persona, []byte("new persona"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := runCreate(t, validCreateSpec, "--spec", "-", "--persona", persona)
	if code != 2 {
		t.Fatalf("persona overwrite must be refused (exit 2); got %d", code)
	}
	if env := parseEnvelope(t, out); env.OK {
		t.Error("envelope must be ok:false")
	}
	got, _ := os.ReadFile(filepath.Join(agentsDir, "evolve-threat-model.md"))
	if string(got) != "existing persona" {
		t.Error("existing persona must be untouched")
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases", "threat-model")); !os.IsNotExist(err) {
		t.Error("phase.json must not be left behind when the persona is refused")
	}
}

func TestPhasesCreate_MintPromotion(t *testing.T) {
	root := createFixtureProject(t)
	mint := `{
  "name": "context-condense",
  "prompt": "Summarize the event history, keeping the first two events verbatim.",
  "tier": "balanced",
  "cli": "claude",
  "writes_source": false
}`
	code, out, errb := runCreate(t, mint, "--mint", "-")
	if code != 0 {
		t.Fatalf("exit = %d (stderr=%q stdout=%q)", code, errb, out)
	}
	env := parseEnvelope(t, out)
	if !env.OK || env.Phase != "context-condense" {
		t.Fatalf("envelope = %+v", env)
	}
	body, err := os.ReadFile(filepath.Join(root, "agents", "evolve-context-condense.md"))
	if err != nil {
		t.Fatalf("mint persona not written: %v", err)
	}
	if !strings.Contains(string(body), "first two events verbatim") {
		t.Errorf("mint prompt must become the persona body; got %q", body)
	}
	var spec map[string]any
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "phases", "context-condense", "phase.json"))
	if err != nil {
		t.Fatalf("mint phase.json not written: %v", err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("phase.json malformed: %v", err)
	}
	if spec["optional"] != true || spec["model"] != "balanced" {
		t.Errorf("derived spec = %v", spec)
	}
}

func TestPhasesCreate_RejectsTraversalAgentName(t *testing.T) {
	root := createFixtureProject(t)
	// A crafted agent field must not escape agents/ (path traversal).
	spec := `{"name": "sneaky-agent", "optional": true, "agent": "../../outside/evil"}`
	persona := filepath.Join(root, "p.md")
	if err := os.WriteFile(persona, []byte("evil"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := runCreate(t, spec, "--spec", "-", "--persona", persona)
	if code != 2 {
		t.Fatalf("traversal agent must be refused (exit 2); got %d", code)
	}
	if env := parseEnvelope(t, out); env.OK {
		t.Error("envelope must be ok:false")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); !os.IsNotExist(err) {
		t.Fatal("file escaped the project tree")
	}
}

func TestPhasesCreate_RejectsRootOutsideProjectAndRoots(t *testing.T) {
	root := createFixtureProject(t)
	outside := t.TempDir()

	code, _, errb := runCreate(t, validCreateSpec, "--spec", "-", "--root", outside)
	if code != 10 {
		t.Fatalf("unconfigured absolute --root must be a usage error (10); got %d (stderr=%q)", code, errb)
	}
	if _, err := os.Stat(filepath.Join(outside, "threat-model")); !os.IsNotExist(err) {
		t.Error("nothing may be written outside configured roots")
	}
	// But a configured plugin root IS a valid target.
	pluginRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	polJSON := `{"paths":{"phase_roots":".evolve/phases:` + pluginRoot + `"}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(polJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runCreate(t, validCreateSpec, "--spec", "-", "--root", pluginRoot)
	if code != 0 {
		t.Fatalf("configured root must be accepted; got %d (stderr=%q stdout=%q)", code, errb, out)
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "threat-model", "phase.json")); err != nil {
		t.Errorf("phase.json not written to the configured root: %v", err)
	}
	_ = root
}

func TestPhasesCreate_RollbackPreservesPreexistingDir(t *testing.T) {
	root := createFixtureProject(t)
	// A directory with the phase's name exists but has no phase.json (invisible
	// to the collision check) and holds unrelated content.
	dir := filepath.Join(root, ".evolve", "phases", "threat-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keep, []byte("operator notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force the persona write to fail AFTER phase.json is written: pre-create
	// agents/ as a FILE so the persona's parent mkdir fails.
	if err := os.WriteFile(filepath.Join(root, "agents"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	persona := filepath.Join(root, "p.md")
	if err := os.WriteFile(persona, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, _ := runCreate(t, validCreateSpec, "--spec", "-", "--persona", persona)
	if code == 0 {
		t.Fatal("persona write should have failed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("rollback must not delete pre-existing operator files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "phase.json")); !os.IsNotExist(err) {
		t.Error("the half-written phase.json itself must be rolled back")
	}
}

func TestPhasesCreate_UsageErrors(t *testing.T) {
	createFixtureProject(t)
	if code, _, _ := runCreate(t, ""); code != 10 {
		t.Errorf("missing --spec/--mint must be usage error 10")
	}
	if code, _, _ := runCreate(t, "x", "--spec", "-", "--mint", "-"); code != 10 {
		t.Errorf("--spec and --mint are mutually exclusive (10)")
	}
}

// TestPhasesCreate_VerifyRoundtrip — test-plan P0 #3 (leg A→B consistency):
// a phase minted by `phases create` must be RESOLVABLE and VERIFIABLE by
// `evolve phase verify` against the same project root — generation and
// check may never disagree about where the contract lives or what it
// requires. Would catch any drift between the create-side spec write and
// the verify-side merged-catalog discovery (registry path, phase roots,
// FromSpec derivation).
func TestPhasesCreate_VerifyRoundtrip(t *testing.T) {
	root := createFixtureProject(t)
	code, out, errb := runCreate(t, validCreateSpec, "--spec", "-")
	if code != 0 {
		t.Fatalf("phases create exit=%d stderr=%s", code, errb)
	}
	env := parseEnvelope(t, out)
	if !env.OK {
		t.Fatalf("create envelope not OK: %+v", env)
	}

	// 1. The minted phase must RESOLVE: a missing artifact is exit 1 (a
	// confirmed violation naming the expected path), never exit 10
	// (unknown phase — resolution drift between create and verify).
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, verr := runVerify(t, "threat-model", "--workspace="+ws)
	if code != 1 {
		t.Fatalf("verify(minted phase, no artifact) exit=%d want 1; stderr=%s", code, verr)
	}

	// 2. A conforming artifact (the sections + verdict sentinel the create
	// envelope advertised) must verify clean: exit 0.
	report := "## Threats\n- spoofing\n\n## Mitigations\n- authn\n\n" +
		"<!-- evolve-verdict: {\"phase\":\"threat-model\",\"verdict\":\"PASS\"} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "threat-model-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, verr = runVerify(t, "threat-model", "--workspace="+ws)
	if code != 0 {
		t.Fatalf("verify(minted phase, conforming artifact) exit=%d want 0; stderr=%s", code, verr)
	}
}
