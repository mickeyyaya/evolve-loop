//go:build acs

// Package cycle1160 materialises the acceptance criteria for the three tasks
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - retire-dead-lifecycle-surface       → 001, 002, 003
//   - document-adr0079-shared-root-risk   → 004, 005
//   - materialize-cycle1158-acs-predicates → 006, 007
//
// `## deferred` and `## dropped` are both empty this cycle, so every predicate
// here binds to committed work (R9.3 floor-binding).
//
// # What is left of the cycle-1156 audit
//
// D1 (BLOCKING: the PASS promote loop early-returned past the residual drain)
// and D2 (`bumpFailureCount` ungated on `systemLevel`, ADR-0072 AC4) landed on
// main in cycle 1157 (`ea1d0006`) and are verified live in this worktree. What
// the audit left open is the cheap half:
//
//   - D3 — `CycleOutcome.LaneIDs` (outcome.go:38) has no reader anywhere in
//     production, and `ReleaseCycleProcessingWithQuarantine` (inboxmover.go:638)
//     has no production caller: `ApplyCycleOutcome`'s FAIL path owns the drain
//     now and calls the unexported `releaseCycleProcessing` core directly. Two
//     public entry points into one lifecycle is exactly the drift the seam was
//     built to prevent (never_duplicate_centralize).
//   - D4 — `ClaimLaneScope` writes into the SHARED inbox root from a per-lane
//     closeout path, while sibling lanes' triage reads that same root with no
//     lane isolation (triage.go:113). At width 3 that is a real, bounded
//     cross-lane miss window, and ADR-0079 does not mention it at all.
//
// # Predicate quality (cycle-85 ban)
//
// No predicate here is satisfiable by adding a magic string to a source file.
// 001 reflects over the real `inboxmover.CycleOutcome` type; 002 asks the Go
// toolchain for the package's actual documented API surface and compiles the
// dependent ACS packages; 003 and 005 drive `ApplyCycleOutcome` /
// `ClaimLaneScope` against real temp trees and assert on where items physically
// land and what their durable failure_count says; 006 and 007 run the cycle-1158
// suite as a subprocess and assert on its exit code and per-test verdicts.
//
// 004 is the single content assertion, and it is legitimate rather than
// degenerate: the ADR text IS the deliverable for that task (there is no
// behavior to call), and 005 pins the behaviour the prose claims so the two
// cannot drift — a doc that names the risk while the code stopped exhibiting it
// fails 005, and code that exhibits it while the ADR stays silent fails 004.
package cycle1160

import (
	"encoding/json"
	"fmt"
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
// returns (projectRoot, inboxDir). Every predicate gets its own temp tree: the
// lifecycle is filesystem-shaped, so a shared root would let one predicate's
// moves leak into another's assertions.
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
// failure_count) into dir, mirroring the real .evolve/inbox/ naming convention
// (<timestamp>-<id>.json). Returns the path written.
func writeItem(t *testing.T, dir, id string, failureCount int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{
		"id":       id,
		"title":    "fixture item " + id,
		"kind":     "bug",
		"weight":   0.5,
		"priority": "medium",
	}
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

// testOpts returns inboxmover Options rooted at root with the landing gate
// stubbed to "landed". The real gate shells out to `git merge-base` and is
// fail-open on a non-git dir; stubbing it keeps these predicates asserting the
// LIFECYCLE rather than incidental git behaviour of a temp dir.
func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
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
// as 0 — the same reading bumpFailureCount uses, so "absent" and "0" are
// indistinguishable to the system and must be to the predicate too.
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
// via `go -C`. RepoRoot resolves the WORKTREE (the source root), which is where
// this cycle's edits live — main's copy is stale until ship.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// acsSubprocess wraps the subprocess call so a missing toolchain SKIPs rather
// than red-failing the suite (the ACS runner may execute on a bare export).
// Anything else — including a compile error — surfaces as a non-zero code.
func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code, err
}

// --- Task: retire-dead-lifecycle-surface (audit D3) -------------------------

// AC (RED): "`CycleOutcome.LaneIDs` no longer exists."
//
// The field is declared at outcome.go:38 and read by NOTHING in production — a
// grep finds it only as a struct literal key in go/acs/cycle1156's predicates.
// A field that only tests write is a claim about the lifecycle that the
// lifecycle does not honour: a caller can pass the full menu scope in LaneIDs
// and reasonably expect the uncommitted remainder to be handled, when in fact
// `ApplyCycleOutcome` derives everything from CommittedIDs and the on-disk
// processing/cycle-N/ contents. YAGNI (audit D3, Core Rule 2): delete it.
//
// Reflection over the real type — not a source grep — so a comment-out or a
// renamed-but-still-present field cannot satisfy this.
func TestC1160_001_lane_ids_field_retired_from_cycle_outcome(t *testing.T) {
	typ := reflect.TypeOf(inboxmover.CycleOutcome{})
	if _, found := typ.FieldByName("LaneIDs"); found {
		t.Errorf("inboxmover.CycleOutcome still declares LaneIDs: the field has zero production readers, so it advertises a lane-scope contract ApplyCycleOutcome does not implement (audit D3)")
	}

	// Anti-overcorrection: retiring the dead field must not take the LIVE
	// inputs with it. These four are read on every closeout.
	for _, name := range []string{"Cycle", "Passed", "CommittedIDs", "SystemLevel"} {
		if _, found := typ.FieldByName(name); !found {
			t.Errorf("inboxmover.CycleOutcome lost load-bearing field %q: the retirement must remove only the production-dead surface", name)
		}
	}
}

// AC (RED): "`ReleaseCycleProcessingWithQuarantine` no longer exists" and the
// ACS packages that called it still compile.
//
// It is a two-line wrapper over the unexported `releaseCycleProcessing` core
// that `ApplyCycleOutcome`'s FAIL path calls directly. Exported, it is a second
// public door into the same lifecycle — and the WRONG one: it drains the whole
// processing/cycle-N/ dir with no committed-set filter, which is precisely the
// menu-semantics bug (`wave-lane-task-quarantine-dead`) the seam exists to fix.
// Every remaining caller is a test.
//
// Behavioural by construction: `go doc` reports the package's ACTUAL exported
// API as the toolchain sees it, so deleting the symbol (or unexporting it) is
// the only way to green this — a comment cannot. The second half compiles the
// dependent ACS packages under the `acs` tag, because a retirement that leaves
// cycle1156/cycle1157 uncompilable turns a HARD suite error into the next
// cycle's problem (go/acs/README.md: a predicate package that fails to compile
// is never a silent PASS).
func TestC1160_002_release_with_quarantine_wrapper_retired(t *testing.T) {
	mod := goDir(t)

	stdout, _, code, _ := acsSubprocess(t, "go", "-C", mod, "doc", "./internal/inboxmover", "ReleaseCycleProcessingWithQuarantine")
	if code == 0 && strings.Contains(stdout, "func ReleaseCycleProcessingWithQuarantine") {
		t.Errorf("inboxmover still exports ReleaseCycleProcessingWithQuarantine:\n%s\nit has zero production callers and drains the whole processing/cycle-N/ dir with no committed-set filter — a second public door into the lifecycle ApplyCycleOutcome now owns (audit D3)", stdout)
	}

	// The seam's own entry points must survive the retirement.
	for _, sym := range []string{"ApplyCycleOutcome", "ClaimLaneScope"} {
		if _, _, c, _ := acsSubprocess(t, "go", "-C", mod, "doc", "./internal/inboxmover", sym); c != 0 {
			t.Errorf("inboxmover no longer exports %s (go doc exit %d): the retirement must remove the dead wrapper, not the seam", sym, c)
		}
	}

	_, stderr, code, _ := acsSubprocess(t, "go", "-C", mod, "vet", "-tags", "acs", "./acs/cycle1156", "./acs/cycle1157")
	if code != 0 {
		t.Errorf("`go vet -tags acs ./acs/cycle1156 ./acs/cycle1157` exited %d after the retirement — a predicate package that no longer compiles is a HARD ACS suite error, so the test-only callers must be migrated to ApplyCycleOutcome in the SAME change:\n%s", code, stderr)
	}
}

// AC (anti-overcorrection twin): deleting the wrapper must not delete the drain
// BEHAVIOUR it fronted. The obvious overcorrection — removing the quarantine
// path along with its dead entry point — re-opens `wave-lane-task-quarantine-dead`
// exactly, so this predicate drives `ApplyCycleOutcome` end to end and asserts
// on the filesystem: committed ids bump, uncommitted menu ids do not, the
// ceiling still parks a poison item, and a system-level FAIL still bumps nothing
// (ADR-0072 AC4, the cycle-1157 repair).
//
// Expected pre-existing GREEN at RED time: it is the regression fence around
// Task 1, not a criterion Task 1 introduces.
func TestC1160_003_apply_cycle_outcome_drain_survives_the_retirement(t *testing.T) {
	// Below the ceiling: committed bumps, menu-only does not.
	root, inbox := newInbox(t)
	proc := filepath.Join(inbox, "processing", "cycle-1160")
	writeItem(t, proc, "committed-item", 0)
	writeItem(t, proc, "menu-only-item", 0)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1160,
		Passed:       false,
		CommittedIDs: []string{"committed-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL) returned error: %v", err)
	}
	committed := findItem(t, inbox, "committed-item")
	if committed == "" {
		t.Fatalf("committed-item not released back to the inbox root after a below-ceiling FAIL")
	}
	if got := failureCountOf(t, committed); got != 1 {
		t.Errorf("committed-item failure_count = %d after one FAILed cycle; want 1 — an un-bumped count makes the ADR-0072 S5 ceiling unreachable again (batch-14: four FAILs, zero increments)", got)
	}
	menuOnly := findItem(t, inbox, "menu-only-item")
	if menuOnly == "" {
		t.Fatalf("menu-only-item not released back to the inbox root after a FAIL")
	}
	if got := failureCountOf(t, menuOnly); got != 0 {
		t.Errorf("menu-only-item failure_count = %d; want 0 — triage never committed it, so no phase worked it and it must not accrue task-level failures", got)
	}

	// At the ceiling: the committed id parks in quarantine/ and is not re-seeded.
	root2, inbox2 := newInbox(t)
	writeItem(t, filepath.Join(inbox2, "processing", "cycle-1160"), "poison-item", 1)
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root2, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1160,
		Passed:       false,
		CommittedIDs: []string{"poison-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL at ceiling) returned error: %v", err)
	}
	if findItem(t, filepath.Join(inbox2, "quarantine"), "poison-item") == "" {
		t.Errorf("poison-item not parked in inbox/quarantine/ at the retry ceiling: the quarantine path went out with the retired wrapper")
	}
	if findItem(t, inbox2, "poison-item") != "" {
		t.Errorf("poison-item re-seeded at the inbox root after quarantine: a quarantined task must stop being re-picked every cycle")
	}

	// SystemLevel: no bump at all (ADR-0072 AC4, the cycle-1157 D2 repair).
	root3, inbox3 := newInbox(t)
	writeItem(t, filepath.Join(inbox3, "processing", "cycle-1160"), "sysfail-item", 1)
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root3, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1160,
		Passed:       false,
		CommittedIDs: []string{"sysfail-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
		SystemLevel:  true,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(system-level FAIL) returned error: %v", err)
	}
	sysfail := findItem(t, inbox3, "sysfail-item")
	if sysfail == "" {
		t.Fatalf("sysfail-item not released back to the inbox root on a system-level failure")
	}
	if got := failureCountOf(t, sysfail); got != 1 {
		t.Errorf("sysfail-item failure_count = %d after a SYSTEM-level FAIL; want it unchanged at 1 — a quota/infra storm must not walk healthy committed ids toward the ceiling (ADR-0072 AC4)", got)
	}
	if findItem(t, filepath.Join(inbox3, "quarantine"), "sysfail-item") != "" {
		t.Errorf("sysfail-item quarantined on a SYSTEM-level failure: ADR-0072 S3 precedence forbids it")
	}
}

// --- Task: document-adr0079-shared-root-risk (audit D4) ---------------------

// AC (RED): "ADR-0079 records the ClaimLaneScope shared-inbox-root mutation as
// an accepted risk, naming the sibling-lane miss window and BOTH bounding
// mechanisms."
//
// The ADR argues (correctly) that claiming at outcome time rather than at
// dispatch avoids starving triage. What it never says is the cost of that
// placement: the claim moves files OUT of the shared inbox root while sibling
// lanes are live, and triage reads that root with no lane isolation
// (triage.go:113). At standing width 3 a sibling's triage can miss an item for
// one cycle. The audit offered "acknowledge (accepted risk) or guard"; guarding
// adds locking to a self-healing, bounded window, so the decision is to
// document.
//
// The `## Verification` requirement is what stops this from being a one-liner:
// a risk paragraph that names no mechanism is not an accepted risk, it is a
// shrug. 005 pins the behaviour these words describe.
func TestC1160_004_adr0079_documents_shared_root_mutation_risk(t *testing.T) {
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

	// A dedicated section, not a sentence smuggled into Consequences.
	if !regexp.MustCompile(`(?mi)^#{2,4} .*risk`).MatchString(body) {
		t.Errorf("ADR-0079 has no heading naming a risk: the D4 acknowledgement must be a locatable section, not an aside")
	}
	if !strings.Contains(lower, "accepted risk") {
		t.Errorf(`ADR-0079 never says "accepted risk": D4 asked for an explicit acceptance, so a later reader can tell a considered trade-off from an oversight`)
	}

	// The subject: what mutates what.
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

	// Both bounding mechanisms — this is what makes it ACCEPTED rather than
	// merely admitted.
	if !regexp.MustCompile(`(?i)(residual )?drain`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the residual drain as a bounding mechanism: it is what self-heals the claim after one cycle")
	}
	if !regexp.MustCompile(`(?i)double-move`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the dest-exists double-move guard (inboxmover.go:706-710) as a bounding mechanism: it is what stops a concurrent release from clobbering the root copy")
	}
}

// AC (behavioural twin of 004): the prose must describe the code that exists.
//
// This is the anti-drift half: 004 could be greened by prose alone, so 005
// drives the two mechanisms the prose is required to name. If a later change
// makes ClaimLaneScope stop mutating the shared root, or removes the drain's
// self-heal, this predicate fails and the ADR's accepted risk becomes stale
// documentation — which is the failure mode `doc_stewardship_policy` exists to
// prevent.
func TestC1160_005_shared_root_risk_and_its_bounds_are_real(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "lane-item", 0)

	// Mechanism 0 (the RISK): the claim moves the item OUT of the shared root,
	// where a sibling lane's triage would have read it.
	claimed, err := inboxmover.ClaimLaneScope(testOpts(root, io.Discard), 1160, []string{"lane-item"})
	if err != nil {
		t.Fatalf("ClaimLaneScope returned error: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("ClaimLaneScope claimed %d id(s) (%v); want 1", len(claimed), claimed)
	}
	if findItem(t, inbox, "lane-item") != "" {
		t.Fatalf("lane-item still at the shared inbox root after ClaimLaneScope — the documented risk premise (a lane-scoped call mutating the shared root) no longer holds; ADR-0079's accepted-risk section is now stale")
	}
	if findItem(t, filepath.Join(inbox, "processing", "cycle-1160"), "lane-item") == "" {
		t.Fatalf("lane-item not in processing/cycle-1160/ after ClaimLaneScope")
	}

	// Mechanism 1 (the self-heal): the residual drain returns the claim to the
	// shared root, so the sibling miss window closes after ONE cycle.
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:  1160,
		Passed: true, // nothing committed: this cycle only drains its residual claims
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(PASS, no committed ids) returned error: %v", err)
	}
	restored := findItem(t, inbox, "lane-item")
	if restored == "" {
		t.Errorf("lane-item not drained back to the shared inbox root: without the self-heal the sibling miss window is permanent, not one cycle, and the risk is no longer acceptable")
	}
	if got := failureCountOf(t, restored); got != 0 {
		t.Errorf("lane-item failure_count = %d after a PASS drain; want 0 — a residual claim released on PASS must not accrue a task-level failure", got)
	}

	// Mechanism 2 (the double-move guard): if a concurrent release already put
	// the file back at the root, the drain must skip rather than clobber it.
	root2, inbox2 := newInbox(t)
	base := filepath.Base(writeItem(t, filepath.Join(inbox2, "processing", "cycle-1160"), "raced-item", 0))
	rootCopy := filepath.Join(inbox2, base)
	if err := os.WriteFile(rootCopy, []byte(`{"id":"raced-item","title":"already released by the sibling"}`), 0o644); err != nil {
		t.Fatalf("seed racing root copy: %v", err)
	}
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root2, io.Discard), inboxmover.CycleOutcome{
		Cycle:  1160,
		Passed: true,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(PASS) with a raced root copy returned error: %v", err)
	}
	after, err := os.ReadFile(rootCopy)
	if err != nil {
		t.Fatalf("root copy of raced-item disappeared: %v", err)
	}
	if !strings.Contains(string(after), "already released by the sibling") {
		t.Errorf("the drain overwrote a root file that a concurrent release had already landed: the dest-exists double-move guard is what bounds the shared-root risk ADR-0079 accepts")
	}
}

// --- Task: materialize-cycle1158-acs-predicates -----------------------------

// AC (RED): "`go test -tags acs -count=1 ./acs/cycle1158/` — all 7 predicates
// TestC1158_001..007 PASS."
//
// Cycle 1158 authored the eval
// (.evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md) but ended
// WARN on an unrelated `debugger` phase failure before materialising it, so the
// whole cycle-1156 repair — D1's collected-not-early-returned promote errors,
// D2's systemLevel gate, and now D3/D4 — has an eval score-cap pointing at a Go
// package that does not exist. Every one of those `max_if_missing` caps is
// therefore live against a missing file.
//
// Asserting on the per-test verdicts rather than just the exit code is what
// rejects the cheap green: an empty package with a single trivial test also
// exits 0.
func TestC1160_006_cycle1158_predicates_exist_and_pass(t *testing.T) {
	mod := goDir(t)
	pkgDir := filepath.Join(mod, "acs", "cycle1158")
	if !acsassert.FileExists(t, filepath.Join(pkgDir, "predicates_test.go")) {
		t.Fatalf("go/acs/cycle1158/predicates_test.go missing: the cycle-1158 eval's seven score_cap entries point at a package that does not exist")
	}

	stdout, stderr, code, _ := acsSubprocess(t, "go", "-C", mod, "test", "-tags", "acs", "-count=1", "-v", "./acs/cycle1158/")
	if code != 0 {
		t.Errorf("`go test -tags acs -count=1 ./acs/cycle1158/` exited %d; want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for i := 1; i <= 7; i++ {
		name := fmt.Sprintf("TestC1158_%03d", i)
		if !regexp.MustCompile(`(?m)^\s*--- PASS: ` + name).MatchString(stdout) {
			t.Errorf("%s did not report PASS in the cycle-1158 suite: the eval's score_cap evidence commands name TestC1158_001..007 exactly, so a missing or renamed predicate leaves that cap unenforceable\nstdout:\n%s", name, stdout)
		}
	}
}

// AC (negative / tag correctness): the cycle-1158 package must be EXCLUDED from
// the normal suite.
//
// ACS predicates are state assertions, not unit tests — "the ADR documents X"
// is false mid-edit — so a package that leaks into `go test ./...` red-fails CI
// on correct work. `acssuite.TestAllACSPredicatesAreTagged` enforces the tag's
// presence in the normal suite; this predicate enforces its EFFECT, which is
// the property that actually matters and the one a stray second file in the
// package could break without touching predicates_test.go.
func TestC1160_007_cycle1158_predicates_excluded_without_the_acs_tag(t *testing.T) {
	mod := goDir(t)
	if !acsassert.FileExists(t, filepath.Join(mod, "acs", "cycle1158", "predicates_test.go")) {
		t.Fatalf("go/acs/cycle1158/predicates_test.go missing — nothing to check for tag exclusion")
	}

	stdout, stderr, code, _ := acsSubprocess(t, "go", "-C", mod, "test", "-count=1", "-v", "./acs/cycle1158/")
	if code != 0 {
		t.Errorf("untagged `go test ./acs/cycle1158/` exited %d; want 0 (the package must build away to nothing without the acs tag)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "--- PASS: TestC1158_") || strings.Contains(stdout, "--- FAIL: TestC1158_") {
		t.Errorf("cycle-1158 predicates RAN without -tags acs: they are environment assertions and will red-fail CI mid-edit — the file needs //go:build acs\nstdout:\n%s", stdout)
	}
}
