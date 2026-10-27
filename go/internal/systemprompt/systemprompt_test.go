package systemprompt

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if got := Resolve("build", dir); got != "be terse" {
		t.Errorf("got %q, want %q", got, "be terse")
	}
}

func TestResolve_ProfileFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("file rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, dir, "build", `{"name":"build","system_prompt_file":"rules.md"}`)
	if got := Resolve("build", dir); got != "file rules" {
		t.Errorf("got %q, want %q (trailing newline trimmed)", got, "file rules")
	}
}

func TestResolve_InlineWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "rules.md"), []byte("file"), 0o644)
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"inline","system_prompt_file":"rules.md"}`)
	if got := Resolve("build", dir); got != "inline" {
		t.Errorf("got %q, want inline", got)
	}
}

func TestResolve_TheRequestEnvironmentNeverOverridesTheProfile(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"profile"}`)
	t.Setenv("EVOLVE_SYSTEM_PROMPT", "global")
	t.Setenv("EVOLVE_BUILD_SYSTEM_PROMPT", "per-agent")
	if got := Resolve("build", dir); got != "profile" {
		t.Errorf("got %q, want the profile system_prompt", got)
	}
}

func TestResolve_NothingSet(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build"}`)
	if got := Resolve("build", dir); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := Resolve("nope", dir); got != "" {
		t.Errorf("missing profile: got %q, want empty", got)
	}
}

func TestResolve_DigestFileWinsOverSystemPromptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "digest.md"), []byte("digest content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("fallback content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, dir, "scout", `{"name":"scout","digest_file":"digest.md","system_prompt_file":"rules.md"}`)
	if got := Resolve("scout", dir); got != "digest content" {
		t.Errorf("got %q, want digest_file content %q to win over system_prompt_file", got, "digest content")
	}
}

func TestResolve_DigestFileMissingFallsBackToSystemPromptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("fallback content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, dir, "scout", `{"name":"scout","digest_file":"missing.md","system_prompt_file":"rules.md"}`)
	if got := Resolve("scout", dir); got != "fallback content" {
		t.Errorf("got %q, want fallback to system_prompt_file content %q when digest_file is absent on disk", got, "fallback content")
	}
}

func TestResolve_DigestFileUnsetLeavesChainUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"be terse"}`)
	if got := Resolve("build", dir); got != "be terse" {
		t.Errorf("got %q, want %q (digest_file addition must not affect profiles that never set it)", got, "be terse")
	}
}

func TestResolve_ProcessEnvDoesNotOverrideProfile(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "build", `{"name":"build","system_prompt":"from-profile"}`)

	t.Setenv("EVOLVE_SYSTEM_PROMPT", "from-process-env")

	if got := Resolve("build", dir); got != "from-profile" {
		t.Errorf("got %q; want %q (process env must not override profile after ResolveNoOS migration)",
			got, "from-profile")
	}
}
