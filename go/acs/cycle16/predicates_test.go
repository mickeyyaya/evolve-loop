//go:build acs

package cycle16

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func TestC16_001_ProfileDirFlagRoutesValidate(t *testing.T) {
	_, defaultDir := cycle16TempProject(t)
	writeProfile16(t, defaultDir, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)

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

func TestC16_002_PersonaOverrideFlagWiresCheckCoherence(t *testing.T) {
	root, profilesDir := cycle16TempProject(t)
	agentsDir := filepath.Join(root, "agents")

	if err := os.WriteFile(filepath.Join(agentsDir, "evolve-widget.md"),
		[]byte("---\nname: evolve-widget\ndescription: fixture\ntools: [\"Read\"]\n---\n\n# widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile16(t, profilesDir, "widget",
		`{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`)

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

func TestC16_003_ProfileDirEnvNoLongerHonoredByPhasecmd(t *testing.T) {
	_, defaultDir := cycle16TempProject(t)

	writeProfile16(t, defaultDir, "stamped",
		`{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)

	altDir := t.TempDir()
	writeProfile16(t, altDir, "ghost",
		`{"name":"ghost","role":"ghost","cli":"claude-tmux","model_tier_default":"sonnet"}`)

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

func TestC16_004_ProcessEnvDoesNotOverrideSystemprompt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.json"),
		[]byte(`{"name":"build","system_prompt":"from-profile"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EVOLVE_SYSTEM_PROMPT", "from-process-env")

	got := systemprompt.Resolve("build", dir, nil)
	if got != "from-profile" {
		t.Errorf("RED: systemprompt.Resolve with process env set returned %q; want \"from-profile\".\n"+
			"Builder must update systemprompt.go to call envchain.ResolveNoOS (drops os.Getenv tier).\n"+
			"Currently returns the process env value because envchain.Resolve includes os.Getenv.",
			got)
	}
}

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
