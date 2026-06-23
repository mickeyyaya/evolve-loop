//go:build acs

// Package cycle16 materializes the cycle-16 acceptance criteria for two
// committed top_n tasks:
//
//	phases-cli-flags — convert EVOLVE_PROFILE_DIR and EVOLVE_PERSONA_OVERRIDE
//	os.Getenv reads in phasecmd/phases.go to explicit CLI flags (--profile-dir,
//	--persona-override) parsed via flag.NewFlagSet at the top of RunPhases.
//
//	systemprompt-profile-ssot — remove the os.Getenv tier for EVOLVE_SYSTEM_PROMPT
//	in systemprompt.go by adding envchain.ResolveNoOS (3-tier: reqEnv → profile → def).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	phases-cli-flags:
//	  AC1  --profile-dir flag routes validate to alt profiles dir         → C16_001 (behavioral)
//	  AC2  --persona-override flag wires into check-coherence             → C16_002 (behavioral)
//	  AC3  EVOLVE_PROFILE_DIR env no longer honored without flag (neg)    → C16_003 (behavioral, negative)
//	  AC4  unknown flag exits non-zero                                    → PRE-EXISTING GREEN (default case returns 10)
//	  AC5  os.Getenv calls absent from phases.go                          → C16_005 (config-check, waiver)
//	  AC6  full phasecmd suite                                            → manual+checklist
//	  AC7  ship/commitgate no regression                                  → manual+checklist
//
//	systemprompt-profile-ssot:
//	  AC8  process env alone no longer overrides EVOLVE_SYSTEM_PROMPT     → C16_004 (behavioral)
//	  AC9  reqEnv still wins over profile                                 → manual+checklist (existing test green)
//	  AC10 profile returned when reqEnv absent                            → manual+checklist (existing test green)
//	  AC11 full systemprompt suite                                        → manual+checklist
//	  AC12 envchain no regression                                         → manual+checklist
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C16_003 (env var IGNORED — setting EVOLVE_PROFILE_DIR without
//	           --profile-dir flag no longer redirects validate) +
//	           C16_004 (process env IGNORED — EVOLVE_SYSTEM_PROMPT in process
//	           env no longer overrides profile when nil reqEnv).
//	Edge/OOD:  C16_003 uses default project dir (no alt) while env points elsewhere.
//	Lexical:   RunPhases / Resolve / FileNotContains — three distinct verbs.
//	Semantic:  CLI flag routing (C16_001), persona wiring (C16_002), env rejection
//	           (C16_003), tier removal (C16_004), source absence (C16_005).
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (phases-cli-flags, systemprompt-profile-ssot). No deferred tasks.
package cycle16

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolveloop/go/internal/systemprompt"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// cycle16TempProject creates a minimal project skeleton for phasecmd tests.
// Sets EVOLVE_PROJECT_ROOT and neutralizes env vars that redirect dir resolution.
func cycle16TempProject(t *testing.T) (root, profilesDir string) {
	t.Helper()
	root = t.TempDir()
	profilesDir = filepath.Join(root, ".evolve", "profiles")
	for _, d := range []string{
		filepath.Join(root, "agents"),
		profilesDir,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	for _, v := range []string{
		"EVOLVE_PLUGIN_ROOT", "EVOLVE_PROFILES_DIR_OVERRIDE", "EVOLVE_PROMPTS_DIR",
		"EVOLVE_PROFILE_DIR", "EVOLVE_PERSONA_OVERRIDE",
	} {
		t.Setenv(v, "")
	}
	return root, profilesDir
}

func writeProfile16(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestC16_001_ProfileDirFlagRoutesValidate verifies that passing --profile-dir
// as a CLI flag before the "validate" verb redirects the provenance check to
// the specified alternate profiles directory instead of the project default.
//
// BEHAVIORAL: calls phasecmd.RunPhases with the flag before the subcommand verb.
//
// RED: args[0]="--profile-dir" matches no case in the current switch → default
// case returns exit 10 "unknown subcommand". Builder must parse a flag.FlagSet
// in RunPhases before the switch dispatch so the flag is consumed and "validate"
// remains as args[0] after parsing.
func TestC16_001_ProfileDirFlagRoutesValidate(t *testing.T) {
	_, defaultDir := cycle16TempProject(t)
	// Stamped profile in default dir → validate is clean without --profile-dir.
	writeProfile16(t, defaultDir, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)

	// Unstamped profile in alt dir → WARN when --profile-dir routes there.
	altDir := t.TempDir()
	writeProfile16(t, altDir, "scout",
		`{"name":"scout","role":"scout","cli":"claude-tmux","model_tier_default":"sonnet"}`)

	var out, errb bytes.Buffer
	rc := phasecmd.RunPhases([]string{"--profile-dir", altDir, "validate"}, nil, &out, &errb)
	if rc != 0 {
		t.Errorf("RED: RunPhases(--profile-dir %s validate) exit %d; want 0.\n"+
			"Builder must parse --profile-dir via flag.NewFlagSet before the dispatch switch.\n"+
			"Stderr: %s", altDir, rc, errb.String())
		return
	}
	combined := out.String() + errb.String()
	if !strings.Contains(combined, "missing") || !strings.Contains(combined, "generated_from") {
		t.Errorf("RED: --profile-dir must surface unstamped 'scout' profile WARN.\n"+
			"Want 'missing' and 'generated_from' in output, got:\n%s", combined)
	}
	if !strings.Contains(combined, "scout") {
		t.Errorf("RED: --profile-dir output must name the unstamped profile 'scout', got:\n%s", combined)
	}
}

// TestC16_002_PersonaOverrideFlagWiresCheckCoherence verifies that passing
// --persona-override <path>:<name> as a CLI flag before "check-coherence"
// substitutes the named persona with the override file and surfaces drift.
//
// BEHAVIORAL: calls phasecmd.RunPhases with the flag before the subcommand verb.
//
// RED: args[0]="--persona-override" hits the default case → exit 10 "unknown
// subcommand". Builder must parse a flag.FlagSet before the switch dispatch.
func TestC16_002_PersonaOverrideFlagWiresCheckCoherence(t *testing.T) {
	root, profilesDir := cycle16TempProject(t)
	agentsDir := filepath.Join(root, "agents")

	// On-disk persona: clean (tools = ["Read"]).
	if err := os.WriteFile(filepath.Join(agentsDir, "evolve-widget.md"),
		[]byte("---\nname: evolve-widget\ndescription: fixture\ntools: [\"Read\"]\n---\n\n# widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile16(t, profilesDir, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`)

	// Override file: adds git-commit → introduces a drift violation.
	overrideFile := filepath.Join(t.TempDir(), "widget-drift.md")
	if err := os.WriteFile(overrideFile,
		[]byte("---\nname: evolve-widget\ndescription: fixture\ntools: [\"Read\", \"git-commit\"]\n---\n\n# widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	rc := phasecmd.RunPhases(
		[]string{"--persona-override", overrideFile + ":widget", "check-coherence"},
		nil, &out, &errb)
	if rc != 0 {
		t.Errorf("RED: RunPhases(--persona-override ...:widget check-coherence) exit %d; want 0.\n"+
			"Builder must parse --persona-override via flag.NewFlagSet before the dispatch switch.\n"+
			"Stderr: %s", rc, errb.String())
		return
	}
	s := out.String()
	if !strings.Contains(s, "contradiction") && !strings.Contains(s, "mismatch") && !strings.Contains(s, "disallowed") {
		t.Errorf("RED: override drift must surface vocabulary (contradiction|mismatch|disallowed).\n"+
			"Got:\n%s", s)
	}
	if !strings.Contains(s, "git-commit") {
		t.Errorf("RED: override drift must name the contradicting tool 'git-commit'.\n"+
			"Got:\n%s", s)
	}
}

// TestC16_003_ProfileDirEnvNoLongerHonoredByPhasecmd verifies the negative
// contract: after the --profile-dir CLI flag migration, setting EVOLVE_PROFILE_DIR
// in the process environment WITHOUT the --profile-dir flag must be IGNORED.
//
// This is the anti-gaming test: it passes only when the env var is not read.
//
// BEHAVIORAL: calls RunPhases(["validate"], ...) without --profile-dir but with
// EVOLVE_PROFILE_DIR pointing to an alt dir containing an unstamped profile.
//
// RED: current phasesValidate reads os.Getenv("EVOLVE_PROFILE_DIR") → finds the
// unstamped "ghost" profile in the alt dir → WARN includes "ghost".
// GREEN: env var is ignored → only default dir is scanned → "ghost" absent from output.
func TestC16_003_ProfileDirEnvNoLongerHonoredByPhasecmd(t *testing.T) {
	_, defaultDir := cycle16TempProject(t)

	// Default dir: all stamped → no WARN under correct behavior.
	writeProfile16(t, defaultDir, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)

	// Alt dir: unstamped "ghost" profile → WARN appears only if env var is honored.
	altDir := t.TempDir()
	writeProfile16(t, altDir, "ghost",
		`{"name":"ghost","role":"ghost","cli":"claude-tmux","model_tier_default":"sonnet"}`)

	// Set env to alt dir — must be ignored after the --profile-dir migration.
	t.Setenv("EVOLVE_PROFILE_DIR", altDir)

	var out, errb bytes.Buffer
	rc := phasecmd.RunPhases([]string{"validate"}, nil, &out, &errb)
	if rc != 0 {
		t.Fatalf("validate exit %d; want 0 (advisory mode); stderr=%s", rc, errb.String())
	}
	combined := out.String() + errb.String()
	if strings.Contains(combined, "ghost") {
		t.Errorf("RED: EVOLVE_PROFILE_DIR env still honored — 'ghost' profile found in output.\n"+
			"After --profile-dir flag migration, os.Getenv(\"EVOLVE_PROFILE_DIR\") must be removed from phasecmd.\n"+
			"Got:\n%s", combined)
	}
}

// TestC16_004_ProcessEnvDoesNotOverrideSystemprompt verifies that after the
// envchain.ResolveNoOS migration, EVOLVE_SYSTEM_PROMPT in the process environment
// does NOT override the profile-sourced system prompt when reqEnv is nil.
//
// BEHAVIORAL: calls systemprompt.Resolve with nil reqEnv after setting the
// process env; asserts the profile value wins.
//
// RED: current envchain.Resolve includes the os.Getenv tier → returns
// "from-process-env". Builder must update systemprompt.go to call
// envchain.ResolveNoOS for EVOLVE_SYSTEM_PROMPT (skips the os.Getenv tier).
func TestC16_004_ProcessEnvDoesNotOverrideSystemprompt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.json"),
		[]byte(`{"name":"build","system_prompt":"from-profile"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Process env set — must NOT win after ResolveNoOS migration.
	t.Setenv("EVOLVE_SYSTEM_PROMPT", "from-process-env")

	got := systemprompt.Resolve("build", dir, nil) // nil reqEnv — only process env + profile
	if got != "from-profile" {
		t.Errorf("RED: systemprompt.Resolve with process env set returned %q; want \"from-profile\".\n"+
			"Builder must update systemprompt.go to call envchain.ResolveNoOS (drops os.Getenv tier).\n"+
			"Currently returns the process env value because envchain.Resolve includes os.Getenv.",
			got)
	}
}

// TestC16_005_OsGetenvCallsAbsentFromPhasecmd verifies that all 5 os.Getenv
// calls for EVOLVE_PROFILE_DIR (3 sites) and EVOLVE_PERSONA_OVERRIDE (2 sites)
// have been removed from go/internal/cli/phasecmd/phases.go.
//
// // acs-predicate: config-check — os.Getenv ABSENCE is the structural contract.
//
// RED: 3× os.Getenv("EVOLVE_PROFILE_DIR") and 2× os.Getenv("EVOLVE_PERSONA_OVERRIDE")
// are present in phases.go before the migration.
func TestC16_005_OsGetenvCallsAbsentFromPhasecmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	phasesFile := filepath.Join(root, "go", "internal", "cli", "phasecmd", "phases.go")
	for _, envRead := range []string{
		`os.Getenv("EVOLVE_PROFILE_DIR")`,
		`os.Getenv("EVOLVE_PERSONA_OVERRIDE")`,
	} {
		if !acsassert.FileNotContains(t, phasesFile, envRead) {
			t.Errorf("RED: phases.go still contains %q — os.Getenv call not yet migrated to CLI flag.\n"+
				"Builder must replace all 5 os.Getenv sites with parsed flag values.\n"+
				"File: %s", envRead, phasesFile)
		}
	}
}
