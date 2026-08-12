//go:build acs

// Package cycle1442 materialises the cycle-1442 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	schema-aligned-salvage-layer → fix-salvage-content-persistence
//
// The defect (cycle-1441 audit H1, HIGH). salvageVerdictWith re-verifies the
// REPAIRED bytes and, when that re-verify passes, returns a Result built by
// struct-copying the ORIGINAL res and flipping only OK/Violations — the
// `repaired` string is validated and then dropped on the floor. Every
// downstream consumer that reads Result.Content, or re-reads the artifact from
// ArtifactPath, still sees the malformed bytes while the gate reports OK=true.
// The gate approves a byte stream that is not the byte stream it approved.
//
// Predicate strategy — each predicate drives the REAL production seam and
// asserts on an observable effect (returned bytes, on-disk bytes), never a
// source-grep of the fix (the cycle-85 degenerate-predicate ban):
//
//   - 001 is the in-memory crux: it takes the Result the production Verify
//     entry point actually produces for a malformed-but-recoverable audit
//     report, salvages it, and then re-verifies the RETURNED Content through
//     the same production verifier. Clean bytes verify OK; the original
//     malformed bytes do not — so this predicate cannot pass unless Content
//     really carries the repaired bytes. A no-op fix that only flips OK (today's
//     code) fails it.
//   - 002 is the wiring/reachability proof through the production CALLER
//     (Reviewer.Review, go/internal/deliverable/reviewer.go:138 — which discards
//     the salvaged Result entirely, making the on-disk write the ONLY channel by
//     which repaired bytes can reach a downstream phase). It runs the real gate
//     over a real workspace and asserts the artifact ON DISK re-verifies clean
//     afterwards. A fix that sets Content but never persists fails it.
//   - 003 is the negative/refusal invariant: a REFUSED salvage must leave both
//     the returned Content and the on-disk artifact byte-identical. This is the
//     anti-overreach guard on 001/002 — it is expected PRE-EXISTING GREEN and
//     must stay green, so the persistence fix cannot be implemented as an
//     unconditional write.
//   - 004 is the package regression floor: the one named package the fix touches
//     must stay green (single named package, no `./...` sweep — flaky-shape rule).
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

// recoverableFencedVerdict is the canonical single-candidate shape salvage
// exists for: the verdict payload is present and unambiguous, but it is fenced
// JSON rather than a canonical evolve-verdict sentinel, so the strict parse
// raises bad_verdict as the SOLE violation. The "## Verdict" heading is
// required — without it the report also fails missing_section, which is the
// multi-violation shape salvage refuses outright (cycle-1392).
const recoverableFencedVerdict = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

// absentVerdict carries no verdict of any kind: unrecoverable, so salvage must
// refuse it and change nothing.
const absentVerdict = "## Verdict\n\nno verdict of any kind here\n"

// verifyContent runs the PRODUCTION verifier over `content` by writing it to a
// throwaway workspace as the audit deliverable and asking the same entry point
// the gate uses whether it is well-formed. This is the oracle the predicates
// judge bytes with: it is the gate's own definition of "clean", not a private
// re-implementation.
func verifyContent(t *testing.T, content string) (deliverable.Result, error) {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws},
		phasecontract.BuiltinResolver{}, config.StageEnforce)
}

// verifiedResultFor produces the Result the production Verify actually emits
// for `content` in a real workspace — path, content and violations all real,
// never a hand-built struct that could drift from what the gate sees.
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

// TestC1442_001_SalvagedResultCarriesTheRepairedBytes is the crux.
//
// It asserts the returned Result.Content is bytes that VERIFY CLEAN through the
// production verifier. The original malformed bytes provably do not (asserted
// as an in-test negative control), so the only way to pass is to actually
// thread `repaired` into the returned Result — the exact line cycle-1441 audit
// H1 says is missing.
func TestC1442_001_SalvagedResultCarriesTheRepairedBytes(t *testing.T) {
	ws := t.TempDir()
	res := verifiedResultFor(t, ws, recoverableFencedVerdict)

	// Precondition: the fixture reaches salvage at all — bad_verdict ALONE.
	if len(res.Violations) != 1 || res.Violations[0].Code != deliverable.CodeBadVerdict {
		t.Fatalf("precondition: fixture must fail for bad_verdict ALONE so salvage is reached; got %+v", res.Violations)
	}
	// Negative control: the ORIGINAL bytes do not verify clean. This is what
	// makes the assertion below load-bearing rather than vacuous.
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

// TestC1442_002_SalvagePersistsRepairedBytesToTheArtifact is the CALLER proof.
//
// It drives Reviewer.Review — the real production gate, the only in-tree caller
// of salvageVerdictWith — over a real workspace, then re-reads the artifact
// FROM DISK and re-verifies it. reviewer.go:138 discards the salvaged Result
// (`if _, applied := ...`), so the on-disk write is the only channel through
// which a downstream phase can ever observe the bytes the gate approved. A fix
// that sets Content in memory but skips the ArtifactPath write leaves the next
// reader looking at malformed bytes and fails here.
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

// TestC1442_003_RefusedSalvageMutatesNothing is the negative / anti-overreach
// invariant: when salvage REFUSES, neither the returned Content nor the on-disk
// artifact may change. Expected PRE-EXISTING GREEN — it exists to forbid
// implementing 001/002 as an unconditional write, which would let salvage
// rewrite reports it explicitly declined to approve.
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

// TestC1442_004_DeliverablePackageStaysGreen is the regression floor for the one
// package the fix touches. Single NAMED package by design (flaky-shape rule: no
// `./...` sweep, no multi-package invocation) — whole-repo staleness is the
// regression suite's job, not a cycle predicate's.
func TestC1442_004_DeliverablePackageStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "./internal/deliverable")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/deliverable failed: %v\n%s", err, out)
	}
}
