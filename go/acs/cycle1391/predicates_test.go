//go:build acs

// Package cycle1391 ports the cycle-1391 ACS predicates for the
// role-scoped-instruction-digest-generator lane (inbox item
// tokenopt-role-scoped-instruction-digests).
//
// Two tasks, one fleet-scoped item:
//
//   - digest-projector-core: a new go/internal/digest package. ProjectDigest
//     scans an SSOT skill/instruction source for
//     "<!-- digest:role=ROLE[,ROLE2,...] -->...<!-- /digest -->" marker pairs
//     and returns the concatenated content of every block whose role list
//     contains the requested role. Untagged content and blocks tagged for
//     other roles are excluded — no hand-maintained duplicate copy.
//   - digest-wire-scout-profile: go/internal/systemprompt.Resolve gains a
//     new profile field "digest_file" (resolved relative to profileDir like
//     system_prompt_file). When set AND the file exists on disk, its content
//     wins over system_prompt_file. When digest_file is unset, or set but the
//     file is absent, the existing 4-tier precedence chain
//     (env > profile.system_prompt > system_prompt_file > "") is unchanged.
package cycle1391

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
)

// --- digest-projector-core -------------------------------------------------

// TestC1391_001_ProjectDigestExtractsOnlyTaggedRoleSections is the primary
// behavioral predicate for AC "Digest projector extracts SSOT-tagged
// sections only; no hand-maintained duplicate copy". It drives the real
// ProjectDigest function (not a source grep) and asserts the output contains
// ONLY the role's own tagged block — not untagged prose, not another role's
// block.
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

// TestC1391_002_ProjectDigestByteReductionAtLeastHalf is the AC "Unit tests
// prove >= 50%% byte reduction on a representative fixture" predicate. The
// fixture pairs a large excluded block (content NOT tagged for the target
// role) against a small tagged block, so a real projection — not a no-op
// pass-through — must shrink the output below half the source size.
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

// TestC1391_003_ProjectDigestRoleWithNoMatchIsEmptyNotFullSource is the
// negative/anti-no-op predicate: a role with zero matching marker blocks
// must get an EMPTY digest, never a silent fallback to the full source. A
// no-op implementation that just returns source verbatim passes test 001's
// happy path trivially but fails this one.
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

// TestC1391_004_ProjectDigestUnterminatedMarkerErrors is the edge/OOD
// predicate: a malformed source with an opening marker but no matching
// "<!-- /digest -->" before EOF must be rejected with a non-nil error, not
// silently truncated or silently ignored.
func TestC1391_004_ProjectDigestUnterminatedMarkerErrors(t *testing.T) {
	source := []byte(`<!-- digest:role=scout -->
This block never closes.
`)

	_, err := digest.ProjectDigest(source, "scout")
	if err == nil {
		t.Errorf("want non-nil error for an unterminated digest marker, got nil")
	}
}

// --- digest-wire-scout-profile ---------------------------------------------

func writeProfileFile(t *testing.T, dir, agent, json string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestC1391_005_ResolvePrefersDigestFileWhenPresent is the primary
// wiring predicate: a profile with BOTH digest_file and system_prompt_file
// set, where the digest file exists on disk, must resolve to the digest
// file's content — proving Resolve actually reads and prefers digest_file,
// not just tolerates the new JSON key.
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

// TestC1391_006_ResolveFallsBackWhenDigestFileMissing is the fallback-path
// regression predicate: digest_file is SET but the file does not exist on
// disk, so Resolve must fall back to system_prompt_file unchanged — not
// silently return empty and not error.
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

// TestC1391_007_ResolveUnchangedWhenDigestFileUnset is the no-digest_shape
// regression predicate: a profile that never sets digest_file at all must
// behave exactly like the pre-cycle-1391 precedence chain (inline
// system_prompt wins, else system_prompt_file, else "").
func TestC1391_007_ResolveUnchangedWhenDigestFileUnset(t *testing.T) {
	dir := t.TempDir()
	writeProfileFile(t, dir, "build", `{"name":"build","system_prompt":"be terse"}`)

	got := systemprompt.Resolve("build", dir, nil)
	if got != "be terse" {
		t.Errorf("Resolve() = %q, want %q (existing inline system_prompt precedence must be unaffected by the digest_file addition)", got, "be terse")
	}
}
