package main

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
	t.Setenv("EVOLVE_PHASE_ROOTS", "")
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
	code := runPhases(append([]string{"create"}, args...), strings.NewReader(stdin), &out, &errb)
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

func TestPhasesCreate_UsageErrors(t *testing.T) {
	createFixtureProject(t)
	if code, _, _ := runCreate(t, ""); code != 10 {
		t.Errorf("missing --spec/--mint must be usage error 10")
	}
	if code, _, _ := runCreate(t, "x", "--spec", "-", "--mint", "-"); code != 10 {
		t.Errorf("--spec and --mint are mutually exclusive (10)")
	}
}
