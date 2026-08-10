//go:build acs

// Package cycle1420 encodes the cycle-1420 ACS predicates for
// verify-disposition-contract-fix-and-retire-inbox-item.
//
// The task is a VERIFICATION-AND-CONSUME step (the cycle-1410 shape) for the
// inbox item defect-disposition-contract-unsatisfiable: cycles 1397/1399/1400
// FAILed because agents authoring <workspace>/defect-dispositions.json had only
// a PROSE schema to work from, guessed `evidence` as a JSON array against a
// `string`-typed field, and the gate could not read the claims it was grading.
// PR #422 (5f405e92) and PR #426 (59579452) landed the three-part fix: a
// literal legal example in agents/evolve-auditor.md single-sourced with
// docs/architecture/continuation-defect-ledger.md, tolerant string-OR-array
// evidence unmarshal, and fail-closed rejection of every other shape.
//
// Predicates 001/002 re-prove that behavior live by driving the production
// reader (they are pre-existing GREEN by design — the fix is already merged and
// this cycle's job is to VERIFY, not re-implement). Predicates 003/004/005 are
// the RED contract: the durable consumed record carrying the verification
// evidence does not exist yet, and the item is still drawable from the live
// inbox.
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

// auditPkg is the ONE named package holding the disposition contract's reader
// and its regression pins. Deliberately not `./...` and not `./internal/core`:
// a multi-package sweep or a known-slow suite inside a cycle predicate is the
// banned flaky shape (cycles 1173/1175/1178 false-REDs under fleet load). Every
// invocation below is additionally narrowed with -run.
const auditPkg = "./internal/phases/audit"

// itemID is the inbox item this cycle retires.
const itemID = "defect-disposition-contract-unsatisfiable"

// itemFile is its filename in the live inbox root.
const itemFile = "2026-08-09T15-55-00Z-defect-disposition-contract-unsatisfiable.json"

// trackedSibling is an in-scope-adjacent item that must SURVIVE this cycle: it
// is a different, still-open defect in the same subsystem and the same
// filename neighbourhood. A `rm .evolve/inbox/*disposition*` that retires the
// target by sweeping its neighbours fails here.
const trackedSibling = "2026-08-06T03-40-00Z-continuation-disposition-producer-duty.json"

// priorConsumed is one pre-existing tracked consumed record. Consuming this
// cycle's item must ADD a record, never clobber the corpus.
const priorConsumed = "2026-07-29-pipeline-defect-pipeline-blocker.json"

// goTest runs one narrowed `go test` in the worktree's module dir and returns
// combined output plus the exit code. cmd.Dir is set explicitly: a bare
// invocation resolves the module from process cwd, which differs between the
// main tree, the worktree, and each fleet lane.
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

// requireRan asserts each named test actually executed. Without it, deleting or
// renaming a regression pin greens the predicate via an empty -run match — a
// passing suite that proves nothing.
func requireRan(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "=== RUN   "+name) {
			t.Errorf("regression pin %s did not execute — it was deleted or renamed, so the contract it pinned is unguarded:\n%s", name, out)
		}
	}
}

// stateRoot resolves the STATE root (MAIN, even from a worktree — issue #12),
// where .evolve/ runtime data such as the live inbox lives. The acs suite
// exports EVOLVE_PROJECT_ROOT for exactly this. Absent it, a live-inbox
// assertion would read the worktree's committed copy and pass vacuously, so
// this skips loudly rather than asserting on the wrong tree.
func stateRoot(t *testing.T) string {
	t.Helper()
	r := strings.TrimSpace(os.Getenv("EVOLVE_PROJECT_ROOT"))
	if r == "" {
		t.Skip("EVOLVE_PROJECT_ROOT unset — the live-inbox retirement is a STATE assertion and is only meaningful against the state root; asserting against the worktree would pass vacuously")
	}
	return r
}

// TestC1420_001_AuditorPromptExampleIsReadableByTheProductionGate is the
// verification leg for the root cause: the literal example the auditor persona
// tells an agent to COPY must be a document the gate's own reader accepts, and
// must be the same document the architecture doc carries. Both pins are driven
// through the production reader (readDispositions), not grepped for a magic
// string — a persona that merely mentions "defect-dispositions.json" without a
// legal fenced example fails here.
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

// TestC1420_002_EvidenceShapeToleranceAndItsNegatives pins BOTH halves of the
// tolerant unmarshal. Tolerance alone is a hazard: a reader that accepts every
// shape grades nothing. The negative pins (unresolvable array, empty array on
// FIXED, object shape, mixed-type array, null, whitespace-only) must still
// BLOCK, and each must be observed executing.
func TestC1420_002_EvidenceShapeToleranceAndItsNegatives(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, code := goTest(t, root, "-v", "-run", `^TestClassify_DispositionEvidence`, auditPkg)
	if code != 0 {
		t.Errorf("the evidence-shape contract is RED (exit=%d) — string-or-array tolerance and its fail-closed negatives are the fix PR #422 landed:\n%s", code, out)
	}
	requireRan(t, out,
		// Tolerance: the shape the agent guessed must now be read.
		"TestClassify_DispositionEvidenceStringShapeAccepted",
		"TestClassify_DispositionEvidenceArrayShapeAccepted",
		// Negatives: tolerance must not become permissiveness.
		"TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks",
		"TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks",
		"TestClassify_DispositionEvidenceObjectShapeStillBlocks",
		"TestClassify_DispositionEvidenceMixedTypeArrayFailsClosed",
		"TestClassify_DispositionEvidenceNullOnFixedStillBlocks",
		"TestClassify_DispositionEvidenceWhitespaceOnlyStillBlocks",
	)
}

// consumedRecord is the subset of the consumed inbox-item schema this cycle
// contracts on.
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

// findConsumedRecord locates the NEW consumed record for this cycle's item,
// selected by its `id` field rather than by filename so a rename cannot hide it
// and a filename alone cannot fake it.
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
			// A neighbour with a different schema is not this cycle's concern.
			continue
		}
		if rec.ID == itemID {
			return p, rec
		}
	}
	t.Fatalf("no consumed record with id %q found in %s — the item was not retired with a durable verification record", itemID, dir)
	return "", consumedRecord{}
}

// TestC1420_003_ConsumedRecordCarriesVerificationEvidence is the RED contract:
// retiring the item requires a durable record stating HOW the already-merged
// fix was verified (commit, timestamp, live test evidence) — not a bare delete,
// and not at the cost of any existing record. A deletion satisfies "gone from
// the inbox" and fails here, which is what makes this the anti-no-op axis.
func TestC1420_003_ConsumedRecordCarriesVerificationEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, ".evolve", "inbox", "consumed")

	// Negative axis: consuming must ADD, never clobber the corpus.
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
	// Edge/OOD axis: a placeholder is worse than nothing — it launders an
	// unverified claim into the audit trail.
	for _, placeholder := range []string{"TODO", "TBD", "n/a", "N/A", "FIXME", "<fill", "pending"} {
		if strings.Contains(ev, placeholder) {
			t.Errorf("%s: verification.evidence contains placeholder %q (%q) — an unverified claim must never be recorded as verification", base, placeholder, ev)
		}
	}
	if !strings.Contains(ev, "internal/phases/audit") {
		t.Errorf("%s: verification.evidence=%q does not name internal/phases/audit — the suite run that proves the disposition contract is readable must be cited", base, ev)
	}
	// The record must state WHICH contract it closes so a forensics sweep can
	// tell this item apart from the two sibling disposition items still open.
	if !strings.Contains(rec.Notes, "defect-dispositions.json") && !strings.Contains(ev, "defect-dispositions.json") {
		t.Errorf("%s: neither notes nor verification.evidence names defect-dispositions.json — the record does not identify the contract it retires", base)
	}
}

// TestC1420_004_VerificationCommitIsMergedAncestorOfHead drives git itself: the
// SHA the record claims must resolve to a real commit that references the PR
// that landed the fix AND be an ancestor of HEAD. A hand-typed, aspirational,
// or unmerged SHA fails here — this is what separates "verified" from "asserted".
func TestC1420_004_VerificationCommitIsMergedAncestorOfHead(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, rec := findConsumedRecord(t, root)

	raw := strings.TrimSpace(rec.Verification.Commit)
	if raw == "" {
		t.Fatalf("verification.commit is empty — nothing to resolve; the record must name the merged commit that carries the fix")
	}
	// Take the first whitespace-delimited token so a "59579452 (#426)" style
	// value still resolves.
	sha := strings.Fields(raw)[0]

	show := exec.Command("git", "-C", root, "log", "--format=%H %s", "-1", sha)
	out, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("verification.commit %q does not resolve in this repo: %v\n%s", sha, err, out)
	}
	// Either PR of the two-part fix is an acceptable citation.
	if !strings.Contains(string(out), "#422") && !strings.Contains(string(out), "#426") {
		t.Errorf("commit %q references neither PR #422 nor #426 — the recorded commit is not the disposition-contract fix:\n%s", sha, out)
	}

	if err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", sha, "HEAD").Run(); err != nil {
		t.Errorf("commit %q is not an ancestor of HEAD — the fix the record claims verified is not actually merged into this lane's base: %v", sha, err)
	}
}

// TestC1420_005_ItemRetiredFromLiveInboxAndSiblingSurvives closes the loop on
// the STATE root: a record in consumed/ that leaves the item drawable from the
// live inbox retires nothing — the next lane picks it up again. The sibling
// assertion is the edge axis: two other disposition-named items in the same
// directory are still open and out of this lane's fleet_scope, so a glob-delete
// that catches them fails even though it "retires" the target.
func TestC1420_005_ItemRetiredFromLiveInboxAndSiblingSurvives(t *testing.T) {
	root := stateRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")

	// Precondition: we must be looking at a real, populated inbox. Without this
	// a wrong/empty root would satisfy the absence assertion vacuously.
	sibling := filepath.Join(inbox, trackedSibling)
	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("sibling item %s is missing from the live inbox at %s — it is a distinct still-open defect outside this lane's fleet_scope and must survive; a glob-delete of *disposition* retires more than the assigned item: %v", trackedSibling, inbox, err)
	}

	if _, err := os.Stat(filepath.Join(inbox, itemFile)); err == nil {
		t.Errorf("%s is still in the live inbox root at %s — the item is still drawable by the next lane, so it is not retired regardless of any consumed record", itemFile, inbox)
	}
}
