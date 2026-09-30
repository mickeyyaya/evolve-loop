//go:build acs

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

func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

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

func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

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

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code, err
}

func TestC1160_001_lane_ids_field_retired_from_cycle_outcome(t *testing.T) {
	typ := reflect.TypeOf(inboxmover.CycleOutcome{})
	if _, found := typ.FieldByName("LaneIDs"); found {
		t.Errorf("inboxmover.CycleOutcome still declares LaneIDs: the field has zero production readers, so it advertises a lane-scope contract ApplyCycleOutcome does not implement (audit D3)")
	}

	for _, name := range []string{"Cycle", "Passed", "CommittedIDs", "SystemLevel"} {
		if _, found := typ.FieldByName(name); !found {
			t.Errorf("inboxmover.CycleOutcome lost load-bearing field %q: the retirement must remove only the production-dead surface", name)
		}
	}
}

func TestC1160_002_release_with_quarantine_wrapper_retired(t *testing.T) {
	mod := goDir(t)

	stdout, _, code, _ := acsSubprocess(t, "go", "-C", mod, "doc", "./internal/inboxmover", "ReleaseCycleProcessingWithQuarantine")
	if code == 0 && strings.Contains(stdout, "func ReleaseCycleProcessingWithQuarantine") {
		t.Errorf("inboxmover still exports ReleaseCycleProcessingWithQuarantine:\n%s\nit has zero production callers and drains the whole processing/cycle-N/ dir with no committed-set filter — a second public door into the lifecycle ApplyCycleOutcome now owns (audit D3)", stdout)
	}

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

func TestC1160_003_apply_cycle_outcome_drain_survives_the_retirement(t *testing.T) {
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

	if !regexp.MustCompile(`(?mi)^#{2,4} .*risk`).MatchString(body) {
		t.Errorf("ADR-0079 has no heading naming a risk: the D4 acknowledgement must be a locatable section, not an aside")
	}
	if !strings.Contains(lower, "accepted risk") {
		t.Errorf(`ADR-0079 never says "accepted risk": D4 asked for an explicit acceptance, so a later reader can tell a considered trade-off from an oversight`)
	}

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

	if !regexp.MustCompile(`(?i)(residual )?drain`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the residual drain as a bounding mechanism: it is what self-heals the claim after one cycle")
	}
	if !regexp.MustCompile(`(?i)double-move`).MatchString(body) {
		t.Errorf("ADR-0079 does not cite the dest-exists double-move guard (inboxmover.go:706-710) as a bounding mechanism: it is what stops a concurrent release from clobbering the root copy")
	}
}

func TestC1160_005_shared_root_risk_and_its_bounds_are_real(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "lane-item", 0)

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

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:  1160,
		Passed: true,
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
