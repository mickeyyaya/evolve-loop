package systemprompt

import (
	"os"
	"path/filepath"
	"testing"
)

// systemprompt_test.go — precedence resolution for the launch-time
// system-prompt/rules (facet B):
//   EVOLVE_<AGENT>_SYSTEM_PROMPT > EVOLVE_SYSTEM_PROMPT
//     > profile.system_prompt > read(profile.system_prompt_file) > ""

func writeProfile(t *testing.T, dir, agent, json string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_ProfileInline(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"be terse"}`)
	if got := Resolve("build", dir, nil); got != "be terse" {
		t.Errorf("got %q, want %q", got, "be terse")
	}
}

func TestResolve_ProfileFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("file rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, dir, "build", `{"name":"build","system_prompt_file":"rules.md"}`)
	if got := Resolve("build", dir, nil); got != "file rules" {
		t.Errorf("got %q, want %q (trailing newline trimmed)", got, "file rules")
	}
}

func TestResolve_InlineWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "rules.md"), []byte("file"), 0o644)
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"inline","system_prompt_file":"rules.md"}`)
	if got := Resolve("build", dir, nil); got != "inline" {
		t.Errorf("got %q, want inline", got)
	}
}

func TestResolve_GlobalEnvOverridesProfile(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"profile"}`)
	reqEnv := map[string]string{"EVOLVE_SYSTEM_PROMPT": "from-global-env"}
	if got := Resolve("build", dir, reqEnv); got != "from-global-env" {
		t.Errorf("got %q, want from-global-env", got)
	}
}

func TestResolve_PerAgentEnvWins(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"profile"}`)
	reqEnv := map[string]string{
		"EVOLVE_SYSTEM_PROMPT":       "global",
		"EVOLVE_BUILD_SYSTEM_PROMPT": "per-agent",
	}
	if got := Resolve("build", dir, reqEnv); got != "per-agent" {
		t.Errorf("got %q, want per-agent", got)
	}
}

func TestResolve_NothingSet(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build"}`)
	if got := Resolve("build", dir, nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// Missing profile is also empty, not an error.
	if got := Resolve("nope", dir, nil); got != "" {
		t.Errorf("missing profile: got %q, want empty", got)
	}
}

func TestResolve_ProcessEnvDoesNotOverrideProfile(t *testing.T) {
	// After the envchain.ResolveNoOS migration, EVOLVE_SYSTEM_PROMPT in the
	// process environment must NOT win over a profile-set system_prompt when
	// reqEnv is nil. The 4-tier chain drops tier-2 (os.Getenv) for this key,
	// becoming: reqEnv → profile → def.
	//
	// RED: current Resolve calls envchain.Resolve which includes os.Getenv tier
	// → returns "from-process-env". Builder must switch to envchain.ResolveNoOS.
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"from-profile"}`)

	t.Setenv("EVOLVE_SYSTEM_PROMPT", "from-process-env")

	if got := Resolve("build", dir, nil); got != "from-profile" {
		t.Errorf("got %q; want %q (process env must not override profile after ResolveNoOS migration)",
			got, "from-profile")
	}
}
