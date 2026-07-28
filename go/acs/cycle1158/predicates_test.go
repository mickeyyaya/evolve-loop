//go:build acs

// Package cycle1158 materialises
// .evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md — the eval
// cycle 1158 authored for the four defects the cycle-1156 audit raised against
// `inboxmover.ApplyCycleOutcome`, the single cycle-outcome lifecycle seam.
//
// Cycle 1158 ended WARN on an unrelated `debugger` phase failure before it could
// write this file, so all seven `score_cap` entries have been pointing at a Go
// package that did not exist: every cap was live against a missing target and
// therefore unenforceable. This package closes that gap. Predicate numbering is
// fixed by the eval's evidence commands (TestC1158_001..007) — a rename leaves
// the corresponding cap unenforceable again.
//
//	001 — D1 (BLOCKING): a PASS-path promote error still drains residual claims
//	002 — D1: the promote loop attempts EVERY committed id after a failure
//	003 — D1 aggravator: the ship phase never claims a drain it did not complete
//	004 — D2: a system-level FAIL never bumps the durable failure_count (AC4)
//	005 — D2 twin: a task-level FAIL still bumps it (the S5 ceiling stays live)
//	006 — D3: the production-dead lifecycle surface is retired
//	007 — D4: ADR-0079 records the ClaimLaneScope shared-root mutation risk
//
// # Predicate quality (cycle-85 ban)
//
// None of these is satisfiable by adding a magic string to a source file. 001,
// 002, 004 and 005 drive the real `ApplyCycleOutcome` over temp trees and assert
// on where items physically land and what their durable `failure_count` says;
// 003 runs the ship package's own regression tests as a subprocess and asserts
// on their per-test verdicts; 006 reflects over the real `CycleOutcome` type and
// asks the Go toolchain for the package's actual exported API. 007 is the single
// content assertion and is legitimate: the ADR prose IS the deliverable for D4,
// and `go/acs/cycle1160`'s predicate 005 pins the behaviour that prose describes
// so the two cannot drift.
package cycle1158

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- fixture helpers --------------------------------------------------------

// newInbox builds an isolated project root with an empty .evolve/inbox/ and
// returns (projectRoot, inboxDir). The lifecycle is filesystem-shaped, so every
// predicate gets its own tree — a shared root would let one predicate's moves
// leak into another's assertions.
func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

// writeItem drops an inbox item JSON carrying id (and an optional pre-existing
// failure_count) into dir, mirroring the real .evolve/inbox/ naming convention.
func writeItem(t *testing.T, dir, id string, failureCount int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{"id": id, "title": "fixture item " + id, "kind": "bug"}
	if failureCount > 0 {
		doc["failure_count"] = failureCount
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	path := filepath.Join(dir, "2026-07-28T00-00-00Z-"+id+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
	return path
}

// testOpts returns Options rooted at root with the landing gate stubbed to
// "landed": the real gate shells out to git, which is noise for a temp dir.
func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

// blockDir writes a regular FILE where a directory is needed, so the
// destination MkdirAll inside Promote fails — the infrastructure non-delivery
// ADR-0079 made loud.
func blockDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("block %s: %v", path, err)
	}
}

// findItem returns the path of the file directly under dir whose JSON .id == id,
// or "". Non-recursive by design: each lifecycle destination is a flat dir.
func findItem(t *testing.T, dir, id string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, rErr := os.ReadFile(path)
		if rErr != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &doc) == nil && doc.ID == id {
			return path
		}
	}
	return ""
}

// failureCountOf reads the durable failure_count off an item JSON. Absent reads
// as 0 — the same reading bumpFailureCount uses.
func failureCountOf(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item %s: %v", path, err)
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse item %s: %v", path, err)
	}
	return doc.FailureCount
}

// goDir returns <repo>/go, the module root every toolchain subprocess runs in
// via `go -C`. RepoRoot resolves the WORKTREE — where this cycle's edits live.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// acsSubprocess wraps the subprocess call so a missing toolchain SKIPs rather
// than red-failing the suite (the ACS runner may execute on a bare export).
func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code, err
}

// --- D1: the PASS path must never early-return past the drain ---------------

// Criterion (cap 9/10, BLOCKING): "A PASS-path promote error still drains
// residual claims back to the inbox root (no early return)."
//
// The cycle-1156 defect was a bare `return` inside the PASS promote loop. One
// unwritable processed/cycle-N/ stranded not just the failing id but every item
// already parked in processing/cycle-N/, reintroducing the cross-cycle orphan
// shape of cycles 124/265/294/295/308 that promoteInbox's own invariant comment
// forbids. The contract is BOTH halves at once: the error still reaches the
// caller, AND the drain has already run by the time it does.
func TestC1158_001_pass_promote_error_still_drains_residual_claims(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1158")
	writeItem(t, procDir, "committed-task", 0)
	writeItem(t, procDir, "residual-task", 0)
	blockDir(t, filepath.Join(inbox, "processed"))

	or, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1158,
		Passed:       true,
		CommittedIDs: []string{"committed-task"},
		CommitSHA:    "cafef00dab",
	})

	if !errors.Is(err, inboxmover.ErrMvFailed) {
		t.Fatalf("ApplyCycleOutcome err = %v; want an ErrMvFailed-wrapping error — a non-delivery must still reach the caller", err)
	}
	// The half that regressed: the drain ran anyway.
	if findItem(t, inbox, "residual-task") == "" {
		t.Errorf("residual-task was not drained back to the inbox root: an early return on the first failed promote strands every claimed item, which is the cross-cycle orphan shape (124/265/294/295/308) the drain exists to prevent")
	}
	if findItem(t, procDir, "residual-task") != "" {
		t.Errorf("residual-task left behind in processing/cycle-1158/: the drain must run even when a promote failed")
	}
	for _, id := range or.Promoted {
		if id == "committed-task" {
			t.Errorf("committed-task reported in OutcomeResult.Promoted although its promote failed: a stranded task must never be reported as delivered")
		}
	}
}

// Criterion (cap 8/10): "The PASS promote loop attempts every committed id even
// after an earlier one fails."
//
// 001 proves the drain still runs; this proves the LOOP itself continued. The
// distinction matters because `break`-then-drain would green 001 while silently
// skipping every id after the first failure. The joined error must therefore
// name BOTH ids — errors.Join over per-id `promote %q` wrappers is the observable
// that only a completed loop can produce.
func TestC1158_002_pass_promote_attempts_every_committed_id(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1158")
	writeItem(t, procDir, "first-task", 0)
	writeItem(t, procDir, "second-task", 0)
	blockDir(t, filepath.Join(inbox, "processed"))

	_, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1158,
		Passed:       true,
		CommittedIDs: []string{"first-task", "second-task"},
		CommitSHA:    "cafef00dab",
	})
	if err == nil {
		t.Fatalf("ApplyCycleOutcome err = nil; want an error naming both failed promotes")
	}
	msg := err.Error()
	for _, id := range []string{"first-task", "second-task"} {
		if !strings.Contains(msg, id) {
			t.Errorf("the returned error never names %q: the promote loop stopped at the first failure instead of attempting every committed id, so the remaining ids were silently skipped\nerr: %s", id, msg)
		}
	}
}

// Criterion (cap 8/10): "The ship phase never logs 'inbox lifecycle drain
// complete' when the drain did not run."
//
// The aggravator half of D1: postship.go appended the OK line unconditionally,
// right after the WARN for the failure that stopped the drain, so an operator
// read success from a cycle whose lifecycle transition demonstrably did not
// complete. The behaviour lives in an internal package, so this predicate runs
// that package's own regression tests as a subprocess and asserts on their
// per-test verdicts — the eval's third score_cap names exactly this command.
func TestC1158_003_ship_never_claims_a_drain_it_did_not_complete(t *testing.T) {
	mod := goDir(t)
	names := []string{
		"TestPromoteInbox_DrainFailure_NoFalseSuccessLog",
		"TestPromoteInbox_PromoteError_StillDrainsResidualClaims",
	}
	stdout, stderr, code, _ := acsSubprocess(t, "go", "-C", mod, "test", "-count=1", "-v",
		"-run", strings.Join(names, "|"), "./internal/phases/ship/")
	if code != 0 {
		t.Errorf("the ship-side false-success regression tests exited %d; want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, name := range names {
		if !regexp.MustCompile(`(?m)^\s*--- PASS: ` + name).MatchString(stdout) {
			t.Errorf("%s did not report PASS: the eval's score_cap evidence command names it exactly, so a missing or renamed test leaves the false-success-log cap unenforceable\nstdout:\n%s", name, stdout)
		}
	}
}

// --- D2: ADR-0072 AC4, and its anti-overcorrection twin ---------------------

// Criterion (cap 7/10): "A system-level FAIL never increments the durable
// failure_count (ADR-0072 AC4)."
//
// This seam is what first makes bumpFailureCount reachable for wave lanes, so an
// ungated bump lets the documented recurring quota-storm class (cycles
// 1077-1096) walk healthy committed ids toward TaskRetryCeiling — after which a
// single later task-level FAIL quarantines a backlog that never failed on its
// own merits. systemLevel gates the BUMP, not merely the quarantine decision.
//
// The fixture is the exact boundary: failure_count 1 against ceiling 2, so an
// ungated bump would both increment AND quarantine. Asserting on the durable
// count (not just "did it quarantine") is what separates a real AC4 gate from a
// gate applied only at the quarantine branch.
func TestC1158_004_system_level_failure_never_bumps_failure_count(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1158")
	writeItem(t, procDir, "storm-task", 1)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1158,
		Passed:       false,
		CommittedIDs: []string{"storm-task"},
		Ceiling:      2,
		SystemLevel:  true,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(system-level FAIL): %v", err)
	}

	released := findItem(t, inbox, "storm-task")
	if released == "" {
		t.Fatalf("storm-task is not back at the inbox root after a system-level FAIL: an S3 halt releases, it never parks")
	}
	if got := failureCountOf(t, released); got != 1 {
		t.Errorf("storm-task failure_count = %d; want 1 (unchanged) — a system-level failure is the infrastructure's fault, not the task's, and must not walk healthy ids toward the retry ceiling (ADR-0072 AC4)", got)
	}
	if findItem(t, filepath.Join(inbox, "quarantine"), "storm-task") != "" {
		t.Errorf("storm-task was quarantined on a system-level failure: S3 precedence is unconditional")
	}
}

// Criterion (cap 7/10, anti-overcorrection twin of 004): "A task-level FAIL
// still increments failure_count (the S5 ceiling stays reachable)."
//
// The cheap way to green 004 is to delete the bump — which re-opens
// `wave-lane-task-quarantine-dead` exactly, the defect this whole seam exists to
// close. Same fixture, same boundary, systemLevel false: the count must reach
// the ceiling and the item must park in quarantine/ rather than return to the
// root where the next triage would re-pick it.
func TestC1158_005_task_level_failure_still_bumps_failure_count(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1158")
	writeItem(t, procDir, "poison-task", 1)
	// A committed id that is NOT at the ceiling: it must bump and release, so
	// this predicate also proves the bump is per-item, not a blanket park.
	writeItem(t, procDir, "healthy-task", 0)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1158,
		Passed:       false,
		CommittedIDs: []string{"poison-task", "healthy-task"},
		Ceiling:      2,
		SystemLevel:  false,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(task-level FAIL): %v", err)
	}

	quarantined := findItem(t, filepath.Join(inbox, "quarantine"), "poison-task")
	if quarantined == "" {
		t.Fatalf("poison-task was not quarantined at failure #2 of ceiling 2: deleting the bump to satisfy the AC4 gate re-opens wave-lane-task-quarantine-dead — the S5 ceiling must stay reachable")
	}
	if got := failureCountOf(t, quarantined); got != 2 {
		t.Errorf("quarantined poison-task failure_count = %d; want 2 — the durable count is the diagnostic an operator reads to decide whether to revive it", got)
	}
	healthy := findItem(t, inbox, "healthy-task")
	if healthy == "" {
		t.Fatalf("healthy-task is not back at the inbox root: below the ceiling a committed id releases for retry")
	}
	if got := failureCountOf(t, healthy); got != 1 {
		t.Errorf("healthy-task failure_count = %d; want 1 — a task-level FAIL bumps every committed id, and only the one AT the ceiling parks", got)
	}
}

// --- D3 / D4: the cheap half of the audit -----------------------------------

// Criterion (cap 5/10): "The production-dead lifecycle surface
// (CycleOutcome.LaneIDs, ReleaseCycleProcessingWithQuarantine) is retired."
//
// `LaneIDs` had zero production readers, so it advertised a lane-scope contract
// ApplyCycleOutcome does not implement: a caller could pass the full menu scope
// and reasonably expect the uncommitted remainder to be handled, when in fact
// everything is derived from CommittedIDs plus the on-disk processing/cycle-N/
// contents. `ReleaseCycleProcessingWithQuarantine` had zero production callers
// and drained the whole dir with no committed-set filter — a second public door
// into the lifecycle ApplyCycleOutcome now owns (never_duplicate_centralize).
//
// Reflection over the real type and `go doc` on the real package — not a source
// grep — so neither a comment-out nor a renamed-but-present symbol satisfies it.
func TestC1158_006_dead_lifecycle_surface_retired(t *testing.T) {
	typ := reflect.TypeOf(inboxmover.CycleOutcome{})
	if _, found := typ.FieldByName("LaneIDs"); found {
		t.Errorf("inboxmover.CycleOutcome still declares LaneIDs: a field only tests write is a claim about the lifecycle the lifecycle does not honour (audit D3)")
	}
	// The fields the seam actually reads must survive the retirement — the
	// obvious overcorrection is to prune the struct too far.
	for _, f := range []string{"Cycle", "Passed", "CommittedIDs", "CommitSHA", "Reason", "Ceiling", "SystemLevel"} {
		if _, found := typ.FieldByName(f); !found {
			t.Errorf("inboxmover.CycleOutcome lost load-bearing field %q: the retirement must remove the dead surface, not the seam's real inputs", f)
		}
	}

	mod := goDir(t)
	stdout, _, code, _ := acsSubprocess(t, "go", "-C", mod, "doc", "./internal/inboxmover", "ReleaseCycleProcessingWithQuarantine")
	if code == 0 && strings.Contains(stdout, "func ReleaseCycleProcessingWithQuarantine") {
		t.Errorf("inboxmover still exports ReleaseCycleProcessingWithQuarantine:\n%s\nzero production callers, no committed-set filter — a second public door into the lifecycle ApplyCycleOutcome owns (audit D3)", stdout)
	}

	// The retirement is only real if the packages that used the retired surface
	// still COMPILE against the migrated API: an ACS package that fails to build
	// is a hard suite error, never a silent PASS.
	for _, pkg := range []string{"./acs/cycle1156", "./acs/cycle1157"} {
		_, stderr, vetCode, _ := acsSubprocess(t, "go", "-C", mod, "vet", "-tags", "acs", pkg)
		if vetCode != 0 {
			t.Errorf("go vet -tags acs %s failed (exit %d): the test-only callers of the retired surface must migrate to ApplyCycleOutcome in the SAME change\nstderr:\n%s", pkg, vetCode, stderr)
		}
	}
}

// Criterion (cap 4/10): "ADR-0079 records the ClaimLaneScope shared-inbox-root
// mutation as an accepted risk."
//
// ADR-0079 argues (correctly) that claiming at outcome time rather than at
// dispatch avoids starving triage. What it never said is the cost: the claim
// moves files OUT of the shared inbox root while sibling lanes are live, and
// triage reads that root with no lane isolation (triage.go:113). At the standing
// fleet width of 3 a sibling's triage can miss an item for one cycle.
//
// The bounding-mechanism requirement is what stops this from being a one-liner:
// a risk paragraph naming no mechanism is not an accepted risk, it is a shrug.
func TestC1158_007_adr0079_documents_shared_root_mutation_risk(t *testing.T) {
	adr := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "adr",
		"0079-cycle-outcome-inbox-lifecycle-seam.md")
	if !acsassert.FileExists(t, adr) {
		t.Fatalf("ADR-0079 missing at %s", adr)
	}
	raw, err := os.ReadFile(adr)
	if err != nil {
		t.Fatalf("read ADR-0079: %v", err)
	}
	body := string(raw)
	lower := strings.ToLower(body)

	// A locatable section, not a sentence smuggled into Consequences.
	if !regexp.MustCompile(`(?mi)^#{2,4} .*risk`).MatchString(body) {
		t.Errorf("ADR-0079 has no heading naming a risk: the D4 acknowledgement must be a locatable section, not an aside")
	}
	if !strings.Contains(lower, "accepted risk") {
		t.Errorf(`ADR-0079 never says "accepted risk": D4 asked for an explicit acceptance, so a later reader can tell a considered trade-off from an oversight`)
	}

	// What mutates what.
	for _, needle := range []struct{ term, why string }{
		{"claimlanescope", "the function that performs the shared-root mutation"},
		{"inbox root", "the shared surface it mutates"},
		{"triage", "the sibling-lane reader with no lane isolation (triage.go:113)"},
	} {
		if !strings.Contains(lower, needle.term) {
			t.Errorf("ADR-0079 never mentions %q (%s): the accepted risk must name what mutates what", needle.term, needle.why)
		}
	}
	if !regexp.MustCompile(`(?i)(sibling|other|concurrent|parallel)\s+lane`).MatchString(body) {
		t.Errorf("ADR-0079 does not describe the exposure to sibling lanes: at standing width 3 the concurrency IS the risk")
	}
	if !regexp.MustCompile(`(?i)(miss|starv|invisib|window)`).MatchString(body) {
		t.Errorf("ADR-0079 does not characterise the miss window: an accepted risk with no stated blast radius cannot be re-evaluated later")
	}

	// Both bounding mechanisms — what makes it ACCEPTED rather than merely admitted.
	if !regexp.MustCompile(`(?i)(residual )?drain`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the residual drain as a bounding mechanism: it is what self-heals the claim after one cycle")
	}
	if !regexp.MustCompile(`(?i)double-move`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the dest-exists double-move guard as a bounding mechanism: it is what stops a concurrent release from clobbering the root copy")
	}
}
