package evalgate

// materialization_remediation_test.go — Gate A must say HOW to satisfy it.
//
// The gate knows exactly which paths it probed: evalFilePath builds
// <projectRoot>/.evolve/evals/<slug>.md and <workspace>/.evolve/evals/<slug>.md
// and then discards them, reporting only bare slug stems. The agent is told a
// filename with no directory, and the generic correction directive then tells it
// not to create files at all. Every scout|gate-block failure in recorded history
// is this gate (1471, 1476, 1504, 1531), each "rejected after 2 correction(s)".
//
// Precedent: audit's gates already inline their remedy (e.g. "Run `evolve skills
// generate`"), which floorVerdictError joins so the recorded reason names the
// fix. Gate A is the outlier.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// scoutWorkspaceSelecting builds a workspace whose scout-report selects slugs,
// with no eval files anywhere — the exact cycle-1531 shape.
func scoutWorkspaceSelecting(t *testing.T, slugs ...string) (projectRoot, workspace string) {
	t.Helper()
	projectRoot, workspace = t.TempDir(), t.TempDir()
	// Mirrors the REAL scout-report shape (cycle-1531): slugs reach the gate via
	// the "## Decision Trace" JSON, which is the path production actually uses.
	// Note the prose fallback (slugLineRE) does NOT match the persona's own
	// backticked "- **Slug:** `x`" form, so the trace is the only live source.
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

// TestMaterializationGate_RemediationNamesTheExactPath: the gate must name where
// the file goes, per missing slug.
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

// TestMaterializationGate_RemediationReachesTheReviewResult is the WIRING proof:
// a remediation the reviewer never attaches is dead code. This is what makes the
// correction directive class-aware in production.
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

// TestMaterializationGate_NotWeakened: the gate still blocks. A "fix" that made
// the gate accept an unmaterialized eval would pass the tests above while
// silently removing the anti-specification-gaming surface.
func TestMaterializationGate_NotWeakened(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "still-missing")
	in := core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws}
	if _, block := (materializationGate{}).check(in); !block {
		t.Error("gate stopped blocking an unmaterialized eval — the gate must not be weakened by this change")
	}
	// And a materialized eval still passes.
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

// TestOtherGates_SupplyNoRemediation: the optional capability must stay optional.
// A gate that does not know how to fix its violation must leave Remediation
// empty so its correction directive is byte-identical to today.
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

// TestMaterializationGate_RemediationNamesOnlyTheMissing closes a mutation the
// rest of this file cannot see. Every other fixture has ALL selected slugs
// missing, so "all selected" and "all missing" are indistinguishable — a
// remediation built from SelectedSlugs instead of missingSlugs passes them all
// while telling the agent to create a file that already exists, and to overwrite
// work the gate already accepted.
func TestMaterializationGate_RemediationNamesOnlyTheMissing(t *testing.T) {
	root, ws := scoutWorkspaceSelecting(t, "already-there", "genuinely-missing")
	// Materialize exactly one of the two.
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
