//go:build acs

package cycle1442

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const recoverableFencedVerdict = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

const absentVerdict = "## Verdict\n\nno verdict of any kind here\n"

func verifyContent(t *testing.T, content string) (deliverable.Result, error) {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws},
		phasecontract.BuiltinResolver{}, config.StageEnforce)
}

func verifiedResultFor(t *testing.T, ws, content string) deliverable.Result {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws},
		phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("production Verify errored on the fixture: %v", err)
	}
	return res
}

func TestC1442_001_SalvagedResultCarriesTheRepairedBytes(t *testing.T) {
	ws := t.TempDir()
	res := verifiedResultFor(t, ws, recoverableFencedVerdict)

	if len(res.Violations) != 1 || res.Violations[0].Code != deliverable.CodeBadVerdict {
		t.Fatalf("precondition: fixture must fail for bad_verdict ALONE so salvage is reached; got %+v", res.Violations)
	}
	if orig, err := verifyContent(t, res.Content); err == nil && orig.OK {
		t.Fatalf("negative control broke: the pre-salvage bytes already verify clean, so this predicate proves nothing")
	}

	got, applied := deliverable.SalvageVerdict(res)
	if !applied {
		t.Fatalf("want applied=true for the canonical single-candidate fenced-JSON shape, got false")
	}
	if !got.OK || len(got.Violations) != 0 {
		t.Fatalf("salvaged Result must be approved with zero Violations; got OK=%v Violations=%+v", got.OK, got.Violations)
	}

	if got.Content == res.Content {
		t.Errorf("RED (cycle-1441 audit H1): salvage approved but Result.Content is byte-identical to the malformed input — the repaired bytes it re-verified were discarded")
	}
	recheck, err := verifyContent(t, got.Content)
	if err != nil {
		t.Fatalf("re-verify of the salvaged Content errored: %v", err)
	}
	if !recheck.OK {
		t.Errorf("RED: the gate reports OK=true over Content that does NOT re-verify clean — approved bytes diverge from verified bytes; violations=%+v", recheck.Violations)
	}
}

func TestC1442_002_SalvagePersistsRepairedBytesToTheArtifact(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "audit-report.md")
	if err := os.WriteFile(artifact, []byte(recoverableFencedVerdict), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := deliverable.NewReviewerWithCatalogStageReportSize(
		config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	out := r.Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: t.TempDir(),
	})
	if !out.Approve {
		t.Fatalf("precondition: the production gate must salvage-approve this shape; got Approve=false reason=%q", out.Reason)
	}

	after, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("re-read the artifact the gate just approved: %v", err)
	}
	if string(after) == recoverableFencedVerdict {
		t.Errorf("RED (cycle-1441 audit H1): the gate approved via salvage but left the on-disk artifact byte-identical and malformed — the next phase to read %s sees bytes the gate never approved", artifact)
	}
	recheck, err := verifyContent(t, string(after))
	if err != nil {
		t.Fatalf("re-verify of the persisted artifact errored: %v", err)
	}
	if !recheck.OK {
		t.Errorf("RED: the persisted artifact does not re-verify clean; violations=%+v", recheck.Violations)
	}
}

func TestC1442_003_RefusedSalvageMutatesNothing(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "audit-report.md")
	res := verifiedResultFor(t, ws, absentVerdict)

	got, applied := deliverable.SalvageVerdict(res)
	if applied {
		t.Fatalf("precondition: a genuinely absent verdict is unrecoverable and must be REFUSED; got applied=true (%+v)", got)
	}
	if got.OK != res.OK || got.Content != res.Content || len(got.Violations) != len(res.Violations) {
		t.Errorf("a refused salvage must return res UNCHANGED; got OK=%v content-changed=%v violations=%d, want OK=%v content-changed=false violations=%d",
			got.OK, got.Content != res.Content, len(got.Violations), res.OK, len(res.Violations))
	}
	after, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("re-read artifact: %v", err)
	}
	if string(after) != absentVerdict {
		t.Errorf("a refused salvage must never touch the artifact on disk; %s was rewritten", artifact)
	}
}

func TestC1442_004_DeliverablePackageStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "./internal/deliverable")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/deliverable failed: %v\n%s", err, out)
	}
}
