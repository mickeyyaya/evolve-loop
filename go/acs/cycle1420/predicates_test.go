//go:build acs

package cycle1420

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit"

const itemID = "defect-disposition-contract-unsatisfiable"

const itemFile = "2026-08-09T15-55-00Z-defect-disposition-contract-unsatisfiable.json"

const trackedSibling = "2026-08-06T03-40-00Z-continuation-disposition-producer-duty.json"

const priorConsumed = "2026-07-29-pipeline-defect-pipeline-blocker.json"

func goTest(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"test", "-count=1"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return string(out), code
}

func requireRan(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "=== RUN   "+name) {
			t.Errorf("regression pin %s did not execute — it was deleted or renamed, so the contract it pinned is unguarded:\n%s", name, out)
		}
	}
}

func stateRoot(t *testing.T) string {
	t.Helper()
	r := strings.TrimSpace(os.Getenv("EVOLVE_PROJECT_ROOT"))
	if r == "" {
		t.Skip("EVOLVE_PROJECT_ROOT unset — the live-inbox retirement is a STATE assertion and is only meaningful against the state root; asserting against the worktree would pass vacuously")
	}
	return r
}

func TestC1420_001_AuditorPromptExampleIsReadableByTheProductionGate(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, code := goTest(t, root, "-v", "-run", `^TestAuditorPrompt`, auditPkg)
	if code != 0 {
		t.Errorf("the disposition contract's doc-sync pins are RED (exit=%d) — the persona example an agent copies is either rejected by the gate's reader or has drifted from the architecture doc, which is the cycle-1397/1399/1400 root cause reopening:\n%s", code, out)
	}
	requireRan(t, out,
		"TestAuditorPromptDispositionExampleIsAcceptedByProductionReader",
		"TestAuditorPromptAndArchDocDispositionExamplesAgree",
	)
}

func TestC1420_002_EvidenceShapeToleranceAndItsNegatives(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, code := goTest(t, root, "-v", "-run", `^TestClassify_DispositionEvidence`, auditPkg)
	if code != 0 {
		t.Errorf("the evidence-shape contract is RED (exit=%d) — string-or-array tolerance and its fail-closed negatives are the fix PR #422 landed:\n%s", code, out)
	}
	requireRan(t, out,
		"TestClassify_DispositionEvidenceStringShapeAccepted",
		"TestClassify_DispositionEvidenceArrayShapeAccepted",
		"TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks",
		"TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks",
		"TestClassify_DispositionEvidenceObjectShapeStillBlocks",
		"TestClassify_DispositionEvidenceMixedTypeArrayFailsClosed",
		"TestClassify_DispositionEvidenceNullOnFixedStillBlocks",
		"TestClassify_DispositionEvidenceWhitespaceOnlyStillBlocks",
	)
}

type consumedRecord struct {
	ID           string `json:"id"`
	Notes        string `json:"notes"`
	Verification struct {
		Commit     string `json:"commit"`
		PR         any    `json:"pr"`
		VerifiedAt string `json:"verified_at"`
		Evidence   string `json:"evidence"`
	} `json:"verification"`
}

func findConsumedRecord(t *testing.T, root string) (string, consumedRecord) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatalf("read %s: %v", e.Name(), rerr)
		}
		var rec consumedRecord
		if jerr := json.Unmarshal(raw, &rec); jerr != nil {
			continue
		}
		if rec.ID == itemID {
			return p, rec
		}
	}
	t.Fatalf("no consumed record with id %q found in %s — the item was not retired with a durable verification record", itemID, dir)
	return "", consumedRecord{}
}

func TestC1420_003_ConsumedRecordCarriesVerificationEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, ".evolve", "inbox", "consumed")

	if _, err := os.Stat(filepath.Join(dir, priorConsumed)); err != nil {
		t.Fatalf("pre-existing consumed record %s was removed or renamed — retiring this cycle's item must add a record, never rewrite the consumed corpus: %v", priorConsumed, err)
	}

	path, rec := findConsumedRecord(t, root)
	base := filepath.Base(path)

	if strings.TrimSpace(rec.Verification.VerifiedAt) == "" {
		t.Errorf("%s: verification.verified_at is empty — the record must timestamp WHEN the live verification run happened, not merely that it was claimed", base)
	}

	ev := strings.TrimSpace(rec.Verification.Evidence)
	if ev == "" {
		t.Errorf("%s: verification.evidence is empty — cite the live %s run that re-proved the contract", base, auditPkg)
	}
	for _, placeholder := range []string{"TODO", "TBD", "n/a", "N/A", "FIXME", "<fill", "pending"} {
		if strings.Contains(ev, placeholder) {
			t.Errorf("%s: verification.evidence contains placeholder %q (%q) — an unverified claim must never be recorded as verification", base, placeholder, ev)
		}
	}
	if !strings.Contains(ev, "internal/phases/audit") {
		t.Errorf("%s: verification.evidence=%q does not name internal/phases/audit — the suite run that proves the disposition contract is readable must be cited", base, ev)
	}
	if !strings.Contains(rec.Notes, "defect-dispositions.json") && !strings.Contains(ev, "defect-dispositions.json") {
		t.Errorf("%s: neither notes nor verification.evidence names defect-dispositions.json — the record does not identify the contract it retires", base)
	}
}

func TestC1420_004_VerificationCommitIsMergedAncestorOfHead(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, rec := findConsumedRecord(t, root)

	raw := strings.TrimSpace(rec.Verification.Commit)
	if raw == "" {
		t.Fatalf("verification.commit is empty — nothing to resolve; the record must name the merged commit that carries the fix")
	}
	sha := strings.Fields(raw)[0]

	show := exec.Command("git", "-C", root, "log", "--format=%H %s", "-1", sha)
	out, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("verification.commit %q does not resolve in this repo: %v\n%s", sha, err, out)
	}
	if !strings.Contains(string(out), "#422") && !strings.Contains(string(out), "#426") {
		t.Errorf("commit %q references neither PR #422 nor #426 — the recorded commit is not the disposition-contract fix:\n%s", sha, out)
	}

	if err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", sha, "HEAD").Run(); err != nil {
		t.Errorf("commit %q is not an ancestor of HEAD — the fix the record claims verified is not actually merged into this lane's base: %v", sha, err)
	}
}

func TestC1420_005_ItemRetiredFromLiveInboxAndSiblingSurvives(t *testing.T) {
	root := stateRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")

	sibling := filepath.Join(inbox, trackedSibling)
	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("sibling item %s is missing from the live inbox at %s — it is a distinct still-open defect outside this lane's fleet_scope and must survive; a glob-delete of *disposition* retires more than the assigned item: %v", trackedSibling, inbox, err)
	}

	if _, err := os.Stat(filepath.Join(inbox, itemFile)); err == nil {
		t.Errorf("%s is still in the live inbox root at %s — the item is still drawable by the next lane, so it is not retired regardless of any consumed record", itemFile, inbox)
	}
}
