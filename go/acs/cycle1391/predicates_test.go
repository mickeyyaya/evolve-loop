//go:build acs

package cycle1391

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
)

func TestC1391_001_ProjectDigestExtractsOnlyTaggedRoleSections(t *testing.T) {
	source := []byte(`Intro line — never inside any digest marker.
<!-- digest:role=scout -->
Scout-only body line one.
Scout-only body line two.
<!-- /digest -->
<!-- digest:role=build -->
Build-only body line one.
<!-- /digest -->
Trailing line — never inside any digest marker.
`)

	got, err := digest.ProjectDigest(source, "scout")
	if err != nil {
		t.Fatalf("ProjectDigest(scout) returned error: %v", err)
	}
	out := string(got)

	if !strings.Contains(out, "Scout-only body line one.") {
		t.Errorf("digest for role=scout missing its own tagged content; got %q", out)
	}
	if strings.Contains(out, "Build-only body line one.") {
		t.Errorf("digest for role=scout leaked role=build content — cross-role isolation broken; got %q", out)
	}
	if strings.Contains(out, "Intro line") || strings.Contains(out, "Trailing line") {
		t.Errorf("digest for role=scout leaked untagged prose — projector is copying the whole doc, not projecting; got %q", out)
	}
}

func TestC1391_002_ProjectDigestByteReductionAtLeastHalf(t *testing.T) {
	excluded := strings.Repeat("This line is cross-cutting ship-gate detail scout never acts on.\n", 40)
	source := []byte("<!-- digest:role=build -->\n" + excluded + "<!-- /digest -->\n" +
		"<!-- digest:role=scout -->\nScout needs only this one line.\n<!-- /digest -->\n")

	got, err := digest.ProjectDigest(source, "scout")
	if err != nil {
		t.Fatalf("ProjectDigest(scout) returned error: %v", err)
	}
	if len(got) >= len(source)/2 {
		t.Errorf("byte reduction target missed: len(digest)=%d, len(source)=%d, want digest < 50%% of source", len(got), len(source))
	}
	if len(got) == 0 {
		t.Errorf("digest for role=scout must not be empty — its own tagged block exists in the fixture")
	}
}

func TestC1391_003_ProjectDigestRoleWithNoMatchIsEmptyNotFullSource(t *testing.T) {
	source := []byte(`<!-- digest:role=scout -->
Scout-only content.
<!-- /digest -->
`)

	got, err := digest.ProjectDigest(source, "ship")
	if err != nil {
		t.Fatalf("ProjectDigest(ship) returned error: %v", err)
	}
	if len(strings.TrimSpace(string(got))) != 0 {
		t.Errorf("role=ship has no tagged block in the fixture, want empty digest, got %q", got)
	}
	if len(got) >= len(source) {
		t.Errorf("digest for unmatched role must be smaller than source (empty), got len=%d >= source len=%d", len(got), len(source))
	}
}

func TestC1391_004_ProjectDigestUnterminatedMarkerErrors(t *testing.T) {
	source := []byte(`<!-- digest:role=scout -->
This block never closes.
`)

	_, err := digest.ProjectDigest(source, "scout")
	if err == nil {
		t.Errorf("want non-nil error for an unterminated digest marker, got nil")
	}
}

func writeProfileFile(t *testing.T, dir, agent, json string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestC1391_005_ResolvePrefersDigestFileWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "digest.md"), []byte("digest content here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("fallback content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfileFile(t, dir, "scout", `{"name":"scout","digest_file":"digest.md","system_prompt_file":"rules.md"}`)

	got := systemprompt.Resolve("scout", dir, nil)
	if got != "digest content here" {
		t.Errorf("Resolve() = %q, want digest_file content %q (digest_file must win over system_prompt_file when present)", got, "digest content here")
	}
}

func TestC1391_006_ResolveFallsBackWhenDigestFileMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("fallback content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProfileFile(t, dir, "scout", `{"name":"scout","digest_file":"missing-digest.md","system_prompt_file":"rules.md"}`)

	got := systemprompt.Resolve("scout", dir, nil)
	if got != "fallback content" {
		t.Errorf("Resolve() = %q, want fallback to system_prompt_file content %q when digest_file is absent on disk", got, "fallback content")
	}
}

func TestC1391_007_ResolveUnchangedWhenDigestFileUnset(t *testing.T) {
	dir := t.TempDir()
	writeProfileFile(t, dir, "build", `{"name":"build","system_prompt":"be terse"}`)

	got := systemprompt.Resolve("build", dir, nil)
	if got != "be terse" {
		t.Errorf("Resolve() = %q, want %q (existing inline system_prompt precedence must be unaffected by the digest_file addition)", got, "be terse")
	}
}
