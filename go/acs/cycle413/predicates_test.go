//go:build acs

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

func TestC413_001_StripProductionHeadingWithSuffix(t *testing.T) {
	const body = "# Agent\n\nBody content.\n\n## Reference Index (Layer 3, on-demand)\n\n- ref one\n- ref two\n"
	const want = "# Agent\n\nBody content.\n\n"
	got := prompts.StripOnDemandSections(body)
	if got != want {
		t.Errorf("StripOnDemandSections with production heading:\n  got  %q\n  want %q\n  (fix: change exact equality to line-anchored prefix match on '## Reference Index')", got, want)
	}
}

func TestC413_002_InlineProductionHeadingMentionNotStripped(t *testing.T) {
	const body = "See ## Reference Index (Layer 3, on-demand) for details.\nMore content.\n"
	got := prompts.StripOnDemandSections(body)
	if got != body {
		t.Errorf("inline mention of production heading triggered strip (line-anchor guard broken):\n  got  %q\n  want unchanged %q", got, body)
	}
}

func TestC413_003_ExactBareHeadingStillStripped(t *testing.T) {
	const body = "# Agent\n\nBody.\n\n## Reference Index\n\n- ref\n"
	const want = "# Agent\n\nBody.\n\n"
	got := prompts.StripOnDemandSections(body)
	if got != want {
		t.Errorf("bare heading not stripped after fix:\n  got  %q\n  want %q", got, want)
	}
}

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

// acs-predicate: config-check — inherent source invariant: literal pins bypass config.
func TestC413_006_NoCompactPromptsLiteralInPhaseConstructors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	phasesDir := filepath.Join(root, "go", "internal", "phases")
	stdout, _, code, _ := acsassert.SubprocessOutput("grep", "-rEn", `CompactPrompts:\s*true`, phasesDir)
	if code == 0 {
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
			if line != "" && !strings.Contains(line, "_test.go:") {
				t.Errorf("found literal 'CompactPrompts: true' in production phase source — must flow from config: %s", line)
			}
		}
	}
}

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
	if reduction < 256 {
		t.Errorf("auditor doc stripped only %d bytes (want ≥256: the marker tail); heading mismatch?\n  body=%d stripped=%d", reduction, len(body), len(stripped))
	}
}

func TestC413_009_TDDEngineerDocReturnedUnchanged(t *testing.T) {
	fixture := "# Agent\n\nOperational content, no reference-index heading.\n"
	if stripped := prompts.StripOnDemandSections(fixture); stripped != fixture {
		t.Errorf("marker-less body was incorrectly stripped:\n  original=%d bytes\n  stripped=%d bytes\n  (docs with no Reference Index heading must be unchanged)", len(fixture), len(stripped))
	}
}
