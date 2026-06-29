//go:build acs

// Package cycle413 materializes the cycle-413 acceptance criteria for three prompt-optimization tasks:
//   - strip-ondemand-heading-prefix-match (Task A)
//   - compact-prompts-config-enable (Task B)
//   - real-doc-ondemand-strip-guard (Task C)
//
// Goal: activate the doubly-dormant CompactPrompts lever — fix the heading match
// (exact equality → line-anchored prefix) and wire the config knob so ~23 KB of
// on-demand reference tail is stripped from per-cycle agent dispatches.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	strip-ondemand-heading-prefix-match:
//	  AC1 production heading "## Reference Index (Layer 3, on-demand)" triggers strip  → C413_001 (RED)
//	  AC2 inline mention of production heading does NOT trigger strip (negative)        → C413_002 (pre-existing GREEN)
//	  AC3 bare "## Reference Index" heading still stripped after fix (edge/OOD)        → C413_003 (pre-existing GREEN)
//
//	compact-prompts-config-enable:
//	  AC1 RoutingConfig has CompactPrompts bool field                                   → C413_004 (RED)
//	  AC2 config.Load populates CompactPrompts from registry workflow.compact_prompts   → C413_005 (RED)
//	  AC3 no literal CompactPrompts: true in phase constructors (anti-gaming)           → C413_006 (pre-existing GREEN, config-check)
//
//	real-doc-ondemand-strip-guard:
//	  AC1 realdoc_strip_test.go exists and is git-tracked                              → C413_007 (RED)
//	  AC2 StripOnDemandSections on real auditor doc shrinks body ≥ 4096 bytes          → C413_008 (RED)
//	  AC3 tdd-engineer doc returned unchanged (no Reference Index tail, negative)      → C413_009 (pre-existing GREEN)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: production heading as inline mention → C413_002 (no-op must NOT strip);
//	          literal CompactPrompts: true present → C413_006; no-op leaves auditor body unchanged → C413_008.
//	Edge/OOD: bare heading still works after prefix change → C413_003;
//	          tdd-engineer has no heading → C413_009.
//	Semantic:  reflection-based field presence (C413_004) vs. config-load value (C413_005) are distinct behaviors.
//
// Deferred (zero predicates per R9.3): B1 (externalize tdd/triage on-demand content),
// B2 (per-cycle context injection audit).
package cycle413

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task A: strip-ondemand-heading-prefix-match
// ─────────────────────────────────────────────────────────────────────────────

// TestC413_001_StripProductionHeadingWithSuffix asserts that
// prompts.StripOnDemandSections strips the real production heading
// "## Reference Index (Layer 3, on-demand)" from a body.
// RED: current impl uses exact equality (== "## Reference Index") which never
// matches the parenthesized production form — the body is returned unchanged.
func TestC413_001_StripProductionHeadingWithSuffix(t *testing.T) {
	const body = "# Agent\n\nBody content.\n\n## Reference Index (Layer 3, on-demand)\n\n- ref one\n- ref two\n"
	const want = "# Agent\n\nBody content.\n\n"
	got := prompts.StripOnDemandSections(body)
	if got != want {
		t.Errorf("StripOnDemandSections with production heading:\n  got  %q\n  want %q\n  (fix: change exact equality to line-anchored prefix match on '## Reference Index')", got, want)
	}
}

// TestC413_002_InlineProductionHeadingMentionNotStripped asserts that a
// mid-line mention of the production heading does NOT trigger a strip.
// Negative test: a naive strings.Contains impl would strip, breaking mid-body prose.
// pre-existing GREEN: current exact-equality impl never matches mid-line text.
func TestC413_002_InlineProductionHeadingMentionNotStripped(t *testing.T) {
	const body = "See ## Reference Index (Layer 3, on-demand) for details.\nMore content.\n"
	got := prompts.StripOnDemandSections(body)
	if got != body {
		t.Errorf("inline mention of production heading triggered strip (line-anchor guard broken):\n  got  %q\n  want unchanged %q", got, body)
	}
}

// TestC413_003_ExactBareHeadingStillStripped asserts backward compatibility:
// the original bare "## Reference Index" heading (no suffix) still triggers a strip
// after the prefix-match change. Edge/OOD: ensures the fix doesn't break the old form.
// pre-existing GREEN: current exact-equality impl handles this correctly.
func TestC413_003_ExactBareHeadingStillStripped(t *testing.T) {
	const body = "# Agent\n\nBody.\n\n## Reference Index\n\n- ref\n"
	const want = "# Agent\n\nBody.\n\n"
	got := prompts.StripOnDemandSections(body)
	if got != want {
		t.Errorf("bare heading not stripped after fix:\n  got  %q\n  want %q", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task B: compact-prompts-config-enable
// ─────────────────────────────────────────────────────────────────────────────

// TestC413_004_CompactPromptsFieldInRoutingConfig asserts that
// config.RoutingConfig has a CompactPrompts bool field. Uses reflection so the
// package compiles even when the field is absent; the runtime assertion fails.
// RED: RoutingConfig has no CompactPrompts field — reflect.Value is invalid.
func TestC413_004_CompactPromptsFieldInRoutingConfig(t *testing.T) {
	rt := reflect.TypeOf(config.RoutingConfig{})
	field, ok := rt.FieldByName("CompactPrompts")
	if !ok {
		t.Fatalf("RED: config.RoutingConfig missing CompactPrompts field; add it to enable config-driven stripping")
	}
	if field.Type.Kind() != reflect.Bool {
		t.Errorf("CompactPrompts is %v, want bool", field.Type.Kind())
	}
}

// TestC413_005_ConfigLoadPopulatesCompactPrompts asserts that config.Load,
// given a registry with workflow.compact_prompts=true, returns a RoutingConfig
// with CompactPrompts=true. Exercises the full registry-parse → field-populate pipeline.
// RED: (a) field absent → reflect.Value invalid, OR (b) field present but Load
//
//	doesn't parse workflow.compact_prompts → value remains false.
func TestC413_005_ConfigLoadPopulatesCompactPrompts(t *testing.T) {
	regJSON := `{"config":{"dynamic_routing":"enforce","workflow":{"compact_prompts":true}},"phases":[]}`
	f := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(f, []byte(regJSON), 0o644); err != nil {
		t.Fatalf("write temp registry: %v", err)
	}
	cfg, warns := config.Load(f, map[string]string{})
	for _, w := range warns {
		t.Logf("config warn: %s: %s", w.Code, w.Message)
	}
	rv := reflect.ValueOf(cfg)
	field := rv.FieldByName("CompactPrompts")
	if !field.IsValid() {
		t.Fatalf("RED: config.RoutingConfig.CompactPrompts absent; config pipeline cannot populate it")
	}
	if !field.Bool() {
		t.Errorf("RED: config.Load with workflow.compact_prompts=true produced CompactPrompts=%v, want true", field.Bool())
	}
}

// TestC413_006_NoCompactPromptsLiteralInPhaseConstructors asserts that no
// phase constructor hard-codes CompactPrompts: true as a Go struct literal.
// The setting MUST flow from config injection, not be a hard-coded pin.
// acs-predicate: config-check — inherent source invariant: literal pins bypass config.
// Test files are excluded (they pin true intentionally for unit-test setup).
// pre-existing GREEN: field doesn't exist yet; no production literal can exist.
func TestC413_006_NoCompactPromptsLiteralInPhaseConstructors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	phasesDir := filepath.Join(root, "go", "internal", "phases")
	// grep exits 0 if matches found, 1 if not. Filter out _test.go lines.
	stdout, _, code, _ := acsassert.SubprocessOutput("grep", "-rEn", `CompactPrompts:\s*true`, phasesDir)
	if code == 0 {
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
			if line != "" && !strings.Contains(line, "_test.go:") {
				t.Errorf("found literal 'CompactPrompts: true' in production phase source — must flow from config: %s", line)
			}
		}
	}
	// exit 1 = no matches = correct invariant
}

// ─────────────────────────────────────────────────────────────────────────────
// Task C: real-doc-ondemand-strip-guard
// ─────────────────────────────────────────────────────────────────────────────

// TestC413_007_RealDocStripGuardFileExistsAndTracked asserts that the real-doc
// strip guard test file exists on disk and is git-tracked in the worktree.
// RED: go/internal/prompts/realdoc_strip_test.go has not been created yet.
func TestC413_007_RealDocStripGuardFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "prompts", "realdoc_strip_test.go")
	p := filepath.Join(root, rel)
	if !acsassert.FileExists(t, p) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s untracked — may be gitignored (dropped at ship)", rel)
	}
}

// TestC413_008_RealAuditorDocStripsAtLeast4096Bytes asserts that
// StripOnDemandSections applied to the real agents/evolve-auditor.md body
// reduces it by at least 4096 bytes. Exercises the system under test against
// the shipped document (not a fixture), pinning the heading convention.
// RED: current exact-equality impl never matches "## Reference Index (Layer 3, on-demand)"
// → reduction = 0 bytes → 0 < 4096 → FAIL.
func TestC413_008_RealAuditorDocStripsAtLeast4096Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-auditor.md"))
	if err != nil {
		t.Fatalf("read evolve-auditor.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	reduction := len(body) - len(stripped)
	if reduction < 4096 {
		t.Errorf("auditor doc stripped only %d bytes (want ≥4096); heading mismatch or reference tail too small\n  body=%d stripped=%d", reduction, len(body), len(stripped))
	}
}

// TestC413_009_TDDEngineerDocReturnedUnchanged asserts that tdd-engineer.md
// (which has no ## Reference Index tail) is returned byte-for-byte unchanged by
// StripOnDemandSections. Negative/edge: confirms strip is a no-op when heading absent.
// pre-existing GREEN: no heading in tdd-engineer.md; returns unchanged in both impls.
func TestC413_009_TDDEngineerDocReturnedUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if stripped != body {
		t.Errorf("tdd-engineer doc was incorrectly stripped:\n  original=%d bytes\n  stripped=%d bytes\n  (docs with no Reference Index heading must be unchanged)", len(body), len(stripped))
	}
}
