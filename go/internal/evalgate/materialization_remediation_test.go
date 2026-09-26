package evalgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// scoutWorkspaceSelecting names slugs as backticked bullets and in the Decision
// Trace, and writes no eval file.
func scoutWorkspaceSelecting(t *testing.T, slugs ...string) (projectRoot, workspace string) {
	t.Helper()
	projectRoot, workspace = t.TempDir(), t.TempDir()
	var b strings.Builder
	b.WriteString("# Scout Report\n\n## Selected Tasks\n\n")
	for _, s := range slugs {
		b.WriteString("- **Slug:** `" + s + "`\n")
	}
	b.WriteString("\n## Decision Trace\n\n```json\n{\"decisionTrace\":[")
	for i, s := range slugs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("{\"slug\":\"" + s + "\",\"finalDecision\":\"selected\"}")
	}
	b.WriteString("]}\n```\n")
	if err := os.WriteFile(filepath.Join(workspace, scoutReportName), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write scout report: %v", err)
	}
	return projectRoot, workspace
}

func TestMaterializationGate_RemediationNamesTheExactPath(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "judgment-phase-shadow-config", "judgment-verdict-shadow-classifier")
	in := core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws}

	reason, block := (materializationGate{}).check(in)
	if !block {
		t.Fatalf("gate must still BLOCK when selected slugs have no eval file (reason=%q)", reason)
	}
	rem := (materializationGate{}).remediation(in)
	if rem == "" {
		t.Fatalf("gate reported %q but supplied no remediation — the agent gets a slug stem with no directory", reason)
	}
	for _, slug := range []string{"judgment-phase-shadow-config", "judgment-verdict-shadow-classifier"} {
		want := filepath.Join(ws, ".evolve", "evals", slug+".md")
		if !strings.Contains(rem, want) {
			t.Errorf("remediation omits the exact path for %q\n  want substring: %s\n  got: %s", slug, want, rem)
		}
	}
	if !strings.Contains(rem, "[code]") {
		t.Errorf("remediation must state the >=1 [code] grader requirement, or the created file fails the NEXT gate\n  got: %s", rem)
	}
}

func TestMaterializationGate_RemediationReachesTheReviewResult(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "brand-new-slug")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws})

	if res.Approve {
		t.Fatalf("enforce stage must reject a scout with an unmaterialized eval")
	}
	if res.Remediation == "" {
		t.Fatalf("the rejection carries no Remediation — composeCorrection therefore emits the generic "+
			"directive that forbids creating files, and the cycle cannot recover (reason=%q)", res.Reason)
	}
	if !strings.Contains(res.Remediation, filepath.Join(ws, ".evolve", "evals", "brand-new-slug.md")) {
		t.Errorf("Remediation reached the result but does not name the path\n  got: %s", res.Remediation)
	}
}

func TestMaterializationGate_NotWeakened(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "still-missing")
	in := core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws}
	if _, block := (materializationGate{}).check(in); !block {
		t.Error("gate stopped blocking an unmaterialized eval — the gate must not be weakened by this change")
	}
	dir := filepath.Join(ws, ".evolve", "evals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "still-missing.md"), []byte("# eval\n`[code]` go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, block := (materializationGate{}).check(in); block {
		t.Error("gate blocks even though the eval file now exists — false positive")
	}
}

func TestOtherGates_SupplyNoRemediation(t *testing.T) {
	for _, g := range newGatesForTest() {
		if g.name() == "evals-materialized" {
			continue
		}
		if _, ok := g.(remediator); ok {
			t.Errorf("gate %q now advertises a remediation — if that is intentional, add a test pinning its "+
				"text; until then its corrections silently changed", g.name())
		}
	}
}

func TestMaterializationGate_RemediationNamesOnlyTheMissing(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "already-there", "genuinely-missing")
	dir := filepath.Join(ws, ".evolve", "evals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "already-there.md"), []byte("# eval\n`[code]` go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws}

	reason, block := (materializationGate{}).check(in)
	if !block {
		t.Fatalf("one slug is still unmaterialized — the gate must block (reason=%q)", reason)
	}
	if strings.Contains(reason, "already-there") {
		t.Errorf("the reason names a slug whose eval EXISTS: %q", reason)
	}
	rem := (materializationGate{}).remediation(in)
	if !strings.Contains(rem, "genuinely-missing.md") {
		t.Errorf("remediation omits the actually-missing slug\n  got: %s", rem)
	}
	if strings.Contains(rem, "already-there.md") {
		t.Errorf("remediation tells the agent to create an eval that ALREADY EXISTS — it would overwrite "+
			"work the gate already accepted (remediation built from SelectedSlugs, not missingSlugs)\n  got: %s", rem)
	}
}
