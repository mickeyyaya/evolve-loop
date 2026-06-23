// cmd_phases_coherence_test.go — RED tests for migration step 4
// (projection-generation-and-meta-gates; cycle-239 retry of cycle-238, test
// contract salvaged verbatim from 878df21 per intent non-goal "salvage,
// don't rewrite"). Three CLI surfaces (architecture blueprint B3/B8/B9/B12):
//
//  1. `evolve phases validate [--strict-provenance]` — profiles missing
//     `generated_from` emit `WARN: profile <name> missing generated_from`
//     (eval profile-provenance-field C4 greps `missing.*generated_from`);
//     advisory exit 0 by default, --strict-provenance ⇒ exit 2. Profile dir
//     resolution: EVOLVE_PROFILE_DIR env → paths default
//     (<project>/.evolve/profiles).
//  2. `evolve phases check-coherence [--strict]` — persona tools: frontmatter
//     vs profile allowed_tools; WARN advisory exit 0, --strict ⇒ exit 2;
//     EVOLVE_PERSONA_OVERRIDE="<path>:<name>" substitutes one persona file
//     (eval persona-tools-coherence-gate C4).
//  3. `evolve phases check-artifact-coherence [--strict]` — persona
//     output-format: artifact basename vs profile output_artifact basename.
//
// Layout: agents at <project>/agents/evolve-<name>.md, profiles at
// <project>/.evolve/profiles/<name>.json, project = EVOLVE_PROJECT_ROOT.
package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newCoherenceProject builds a temp project root with agents/ and
// .evolve/profiles/ dirs, points EVOLVE_PROJECT_ROOT at it, and neutralizes
// the env overrides that would redirect dir resolution.
func newCoherenceProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{
		filepath.Join(root, "agents"),
		filepath.Join(root, ".evolve", "profiles"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	for _, v := range []string{"EVOLVE_PLUGIN_ROOT", "EVOLVE_PROFILES_DIR_OVERRIDE",
		"EVOLVE_PROFILE_DIR", "EVOLVE_PERSONA_OVERRIDE", "EVOLVE_PROMPTS_DIR"} {
		t.Setenv(v, "")
	}
	return root
}

func writeCoherencePersona(t *testing.T, root, name string, frontmatter ...string) string {
	t.Helper()
	body := "---\nname: evolve-" + name + "\ndescription: fixture\n"
	for _, l := range frontmatter {
		body += l + "\n"
	}
	body += "---\n\n# " + name + "\n"
	p := filepath.Join(root, "agents", "evolve-"+name+".md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return body
}

func writeCoherenceProfile(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, ".evolve", "profiles", name+".json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Task 1: provenance check wired into `phases validate` (B3) ---

func TestRunPhases_ProvenanceWarnsByDefault(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherenceProfile(t, root, "naked",
		`{"name":"naked","role":"naked","cli":"claude-tmux","model_tier_default":"sonnet"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"validate"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0 (default mode is advisory); stderr=%s", rc, errb.String())
	}
	combined := out.String() + errb.String()
	// Eval C4 greps `missing.*generated_from`.
	if !strings.Contains(combined, "missing") || !strings.Contains(combined, "generated_from") {
		t.Errorf("validate output must match missing.*generated_from, got:\n%s", combined)
	}
	if !strings.Contains(combined, "naked") {
		t.Errorf("validate output must name the unstamped profile %q, got:\n%s", "naked", combined)
	}
}

func TestRunPhases_ProvenanceStrictExitsTwoOnMissing(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherenceProfile(t, root, "naked",
		`{"name":"naked","role":"naked","cli":"claude-tmux","model_tier_default":"sonnet"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"validate", "--strict-provenance"}, nil, &out, &errb); rc != 2 {
		t.Errorf("rc = %d, want 2 (--strict-provenance with unstamped profile); out=%s stderr=%s",
			rc, out.String(), errb.String())
	}
}

func TestRunPhases_ProvenanceStrictPassesWhenAllStamped(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherenceProfile(t, root, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"validate", "--strict-provenance"}, nil, &out, &errb); rc != 0 {
		t.Errorf("rc = %d, want 0 (all profiles stamped); out=%s stderr=%s",
			rc, out.String(), errb.String())
	}
	if s := out.String() + errb.String(); strings.Contains(s, "generated_from") {
		t.Errorf("stamped tree must not WARN about generated_from, got:\n%s", s)
	}
}

func TestRunPhases_ProvenanceHonorsProfileDir(t *testing.T) {
	// --profile-dir flag routes validate to the alternate profiles dir;
	// the unstamped profile there must be detected even though the
	// project-root profiles dir is clean.
	// (Renamed from TestRunPhases_ProvenanceHonorsProfileDirEnv — cycle-16
	// migrates EVOLVE_PROFILE_DIR to the --profile-dir CLI flag.)
	root := newCoherenceProject(t)
	writeCoherenceProfile(t, root, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)
	alt := t.TempDir()
	if err := os.WriteFile(filepath.Join(alt, "scout.json"),
		[]byte(`{"name":"scout","role":"scout","cli":"claude-tmux","model_tier_default":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pass --profile-dir as a CLI flag before the subcommand verb (not env var).
	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"--profile-dir", alt, "validate"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr=%s", rc, errb.String())
	}
	combined := out.String() + errb.String()
	if !strings.Contains(combined, "missing") || !strings.Contains(combined, "generated_from") || !strings.Contains(combined, "scout") {
		t.Errorf("--profile-dir alt dir not scanned — want missing-generated_from WARN for scout, got:\n%s", combined)
	}
}

func TestRunPhases_UnknownFlagReturnsNonZero(t *testing.T) {
	// Passing an unknown flag must return exit != 0 (usage error).
	// PRE-EXISTING GREEN: current default case already returns exit 10 for
	// unrecognised args; after the flag.FlagSet migration it returns 10 via
	// flag parse error. Both paths exit non-zero.
	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"--unknown-flag-xyz"}, nil, &out, &errb); rc == 0 {
		t.Errorf("RunPhases([--unknown-flag-xyz]) exit 0; want non-zero (usage error)")
	}
}

// --- Task 2: `phases check-coherence` (B8/B9) ---

func TestRunPhases_CheckCoherenceCleanExitsZero(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "widget", `tools: ["Read", "Bash"]`)
	writeCoherenceProfile(t, root, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","Bash(scripts/x:*)"]}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-coherence"}, nil, &out, &errb); rc != 0 {
		t.Errorf("rc = %d, want 0; out=%s stderr=%s", rc, out.String(), errb.String())
	}
	if strings.Contains(out.String()+errb.String(), "WARN") {
		t.Errorf("clean pair must not WARN, got:\n%s%s", out.String(), errb.String())
	}
}

func TestRunPhases_CheckCoherenceReportsDriftAsWarn(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "widget", `tools: ["Read", "Edit"]`)
	writeCoherenceProfile(t, root, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-coherence"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0 (default is advisory); stderr=%s", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{"WARN", "widget", "Edit"} {
		if !strings.Contains(s, want) {
			t.Errorf("check-coherence stdout missing %q:\n%s", want, s)
		}
	}
}

func TestRunPhases_CheckCoherenceStrictExitsTwoOnDrift(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "widget", `tools: ["Read", "Edit"]`)
	writeCoherenceProfile(t, root, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-coherence", "--strict"}, nil, &out, &errb); rc != 2 {
		t.Errorf("rc = %d, want 2 (--strict with drift); out=%s", rc, out.String())
	}
}

func TestRunPhases_CheckCoherencePersonaOverride(t *testing.T) {
	// --persona-override <path>:<name> substitutes the named persona's file.
	// On-disk pair is clean; the override adds a contradicting tool → WARN.
	// (Renamed from TestRunPhases_CheckCoherencePersonaOverrideEnv — cycle-16
	// migrates EVOLVE_PERSONA_OVERRIDE to the --persona-override CLI flag.)
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "widget", `tools: ["Read"]`)
	writeCoherenceProfile(t, root, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`)
	override := filepath.Join(t.TempDir(), "widget-drift.md")
	if err := os.WriteFile(override,
		[]byte("---\nname: evolve-widget\ndescription: fixture\ntools: [\"Read\", \"git-commit\"]\n---\n\n# widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pass --persona-override as a CLI flag before the subcommand verb (not env var).
	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"--persona-override", override + ":widget", "check-coherence"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr=%s", rc, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "contradiction") && !strings.Contains(s, "mismatch") && !strings.Contains(s, "disallowed") {
		t.Errorf("override drift must surface eval vocabulary (contradiction|mismatch|disallowed), got:\n%s", s)
	}
	if !strings.Contains(s, "git-commit") {
		t.Errorf("override drift must name the contradicting tool git-commit, got:\n%s", s)
	}
}

// --- Task 3: `phases check-artifact-coherence` (B12) ---

func TestRunPhases_CheckArtifactCoherenceCleanExitsZero(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "builder",
		`output-format: "build-report.md — Design Decision, Files Changed"`)
	writeCoherenceProfile(t, root, "builder",
		`{"name":"builder","role":"builder","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-artifact-coherence"}, nil, &out, &errb); rc != 0 {
		t.Errorf("rc = %d, want 0; out=%s stderr=%s", rc, out.String(), errb.String())
	}
	if strings.Contains(out.String()+errb.String(), "WARN") {
		t.Errorf("matched pair must not WARN, got:\n%s%s", out.String(), errb.String())
	}
}

func TestRunPhases_CheckArtifactCoherenceReportsMismatch(t *testing.T) {
	root := newCoherenceProject(t)
	// I-3(d) incident replica.
	writeCoherencePersona(t, root, "plan-review",
		`output-format: "plan-review.md — ## Findings, ## Verdict"`)
	writeCoherenceProfile(t, root, "plan-review",
		`{"name":"plan-review","role":"plan-review","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/plan-review-report.md"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-artifact-coherence"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0 (default is advisory); stderr=%s", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{"WARN", "plan-review.md", "plan-review-report.md"} {
		if !strings.Contains(s, want) {
			t.Errorf("check-artifact-coherence stdout missing %q:\n%s", want, s)
		}
	}
}

func TestRunPhases_CheckArtifactCoherenceStrictExitsTwoOnMismatch(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherencePersona(t, root, "plan-review",
		`output-format: "plan-review.md — ## Findings"`)
	writeCoherenceProfile(t, root, "plan-review",
		`{"name":"plan-review","role":"plan-review","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/plan-review-report.md"}`)

	var out, errb bytes.Buffer
	if rc := RunPhases([]string{"check-artifact-coherence", "--strict"}, nil, &out, &errb); rc != 2 {
		t.Errorf("rc = %d, want 2 (--strict with mismatch); out=%s", rc, out.String())
	}
}
