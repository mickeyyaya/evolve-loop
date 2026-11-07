//go:build acs

package cycle2212

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	rungKey         = "last_failure_rung"
	itemID          = "lane-item"
	failureCycle    = 7
	buildFloorFault = "review gate: phase \"build\" deliverable rejected after 2 correction(s): build handoff floor: ./acs/cycle7 (-tags acs): unit tests FAIL"
)

type project struct {
	root, inbox, workspace string
}

func seedProject(t *testing.T, itemBody map[string]any) project {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for _, dir := range []string{inbox, workspace} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	itemBody["id"], itemBody["title"], itemBody["kind"] = itemID, "fixture", "bug"
	writeJSON(t, filepath.Join(inbox, "2026-11-07T00-00-00Z-"+itemID+".json"), itemBody)
	writeJSON(t, filepath.Join(workspace, "triage-decision.json"), map[string]any{"top_n": []map[string]string{{"id": itemID}}})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, itemID, "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	return project{root: root, inbox: inbox, workspace: workspace}
}

func cloneFields(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (p project) sealFail(t *testing.T, class string) {
	t.Helper()
	sentinel := "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"" + class + "\"}} -->\n"
	if err := os.WriteFile(filepath.Join(p.workspace, "build-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(p.workspace, "audit-fail-reason.json"), map[string]any{"schema_version": 1, "phase": "build", "reasons": []string{buildFloorFault}})
}

func (p project) recordLadderRung(t *testing.T, rung string) {
	t.Helper()
	writeJSON(t, filepath.Join(p.workspace, "cycle-state.json"), map[string]any{"cycle_id": 2212, "ladder": map[string]any{"rung": rung}})
}

func (p project) recordLadderlessState(t *testing.T) {
	t.Helper()
	writeJSON(t, filepath.Join(p.workspace, "cycle-state.json"), map[string]any{"cycle_id": 2212})
}

func (p project) closeOut(t *testing.T, ceiling int) {
	t.Helper()
	in := cycleoutcome.FailureInputsFor(p.root, filepath.Join(p.root, ".evolve"), p.workspace, failureCycle, io.Discard)
	in.Ceiling = ceiling
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
}

func itemFields(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(dir, itemID)
	if err != nil {
		t.Fatalf("%s not in %s: %v", itemID, dir, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestC2212_001_AFailedItemRecordsTheRungOfItsTry(t *testing.T) {
	for _, rung := range []string{"R0", "R1", "R2", "R3"} {
		t.Run(rung, func(t *testing.T) {
			p := seedProject(t, map[string]any{})
			p.recordLadderRung(t, rung)
			p.sealFail(t, "code-build-fail")
			p.closeOut(t, 3)
			got := itemFields(t, p.inbox)
			if string(got[rungKey]) != `"`+rung+`"` {
				t.Errorf("%s = %s, want %q: the failure record must carry the rung the item failed at: %v", rungKey, got[rungKey], rung, got)
			}
			if string(got["last_failure_class"]) != `"capability"` {
				t.Errorf("the class stamp beside the rung must survive: %v", got)
			}
		})
	}
}

func TestC2212_002_AnItemParkedAtTheCeilingKeepsItsRung(t *testing.T) {
	p := seedProject(t, map[string]any{})
	p.recordLadderRung(t, "R1")
	p.sealFail(t, "code-build-fail")
	p.closeOut(t, 1)
	got := itemFields(t, filepath.Join(p.inbox, "quarantine"))
	if string(got[rungKey]) != `"R1"` {
		t.Errorf("a quarantined item keeps the rung of its last failure: %v", got)
	}
}

func TestC2212_003_AFailureWithoutLadderStateClearsTheOlderRung(t *testing.T) {
	stale := map[string]any{"failure_count": 1, "last_failure_class": "capability", rungKey: "R2"}
	cases := map[string]func(project, *testing.T){
		"no cycle state at all": func(project, *testing.T) {},
		"a state without a ladder": func(p project, t *testing.T) {
			p.recordLadderlessState(t)
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			p := seedProject(t, cloneFields(stale))
			arrange(p, t)
			p.sealFail(t, "code-build-fail")
			p.closeOut(t, 3)
			got := itemFields(t, p.inbox)
			if string(got["failure_count"]) != "2" {
				t.Fatalf("the failure was not charged: %v", got)
			}
			if raw, kept := got[rungKey]; kept {
				t.Errorf("the record describes the latest failure only, but a rung %s stayed: a stale R2 would send the next item to R2", raw)
			}
		})
	}
}

func TestC2212_004_ARungTheLadderDoesNotKnowIsNeverRecorded(t *testing.T) {
	for _, bad := range []string{"R4", "r1", "deep", "", " R1", "R1 "} {
		t.Run("rung_"+bad, func(t *testing.T) {
			p := seedProject(t, cloneFields(map[string]any{"failure_count": 1, rungKey: "R2"}))
			p.recordLadderRung(t, bad)
			p.sealFail(t, "code-build-fail")
			p.closeOut(t, 3)
			got := itemFields(t, p.inbox)
			if raw, kept := got[rungKey]; kept {
				t.Errorf("rung %q was recorded as %s: an unknown rung must leave no claim, not a guess", bad, raw)
			}
			if string(got["failure_count"]) != "2" {
				t.Errorf("a malformed rung must not stop the failure charge: %v", got)
			}
		})
	}
}

func TestC2212_005_AnInfraFailureLeavesTheRungAlone(t *testing.T) {
	p := seedProject(t, map[string]any{})
	p.recordLadderRung(t, "R1")
	p.sealFail(t, "infrastructure-transient")
	p.closeOut(t, 3)
	got := itemFields(t, p.inbox)
	if raw, stamped := got[rungKey]; stamped {
		t.Errorf("an infra failure is not the item's fault, yet it recorded rung %s", raw)
	}
	if _, charged := got["failure_count"]; charged {
		t.Errorf("an infra failure charged the item: %v", got)
	}
}

func TestC2212_006_ReleasingFromQuarantineClearsTheRung(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	quarantine := filepath.Join(inbox, "quarantine")
	if err := os.MkdirAll(quarantine, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(quarantine, "q.json"), map[string]any{
		"id": "q", "failure_count": 3, "last_failure_reason": "r", "last_failure_class": "capability", "last_failure_fingerprint": "codes:x", rungKey: "R2",
	})
	res, err := lifecycle.New(inbox, nil).ReleaseFromQuarantine("q")
	if err != nil {
		t.Fatalf("ReleaseFromQuarantine: %v", err)
	}
	body, err := os.ReadFile(res.DestPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if raw, kept := doc[rungKey]; kept {
		t.Errorf("a released item starts clean, but rung %s stayed beside a zeroed count", raw)
	}
	if string(doc["failure_count"]) != "0" {
		t.Errorf("the count reset must still hold: %v", doc)
	}
}

type rungReader interface {
	ReadLastFailureRung(taskID string) string
}

func TestC2212_007_TheRecordedRungReadsBackFromAPendingOrClaimedItem(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	for rel, doc := range map[string]map[string]any{
		"a.json":                       {"id": "a", "failure_count": 1, rungKey: "R1"},
		"processing/cycle-3/b.json":    {"id": "b", "failure_count": 2, rungKey: "R2"},
		"c.json":                       {"id": "c", "failure_count": 1},
		"quarantine/q.json":            {"id": "q", rungKey: "R1"},
		"processing/cycle-4/zero.json": {"id": "zero", rungKey: "R0"},
	} {
		path := filepath.Join(inbox, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, path, doc)
	}
	reader, ok := any(lifecycle.New(inbox, nil)).(rungReader)
	if !ok {
		t.Fatal("lifecycle.Mover has no ReadLastFailureRung(taskID string) string: D10 has no way to read the record")
	}
	for id, want := range map[string]string{"a": "R1", "b": "R2", "zero": "R0", "c": "", "q": "", "ghost": ""} {
		if got := reader.ReadLastFailureRung(id); got != want {
			t.Errorf("ReadLastFailureRung(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestC2212_008_TheRungKeyIsALoopStampTheCuratorCannotEdit(t *testing.T) {
	role := inboxbatch.RoleOf(rungKey)
	if !role.IsStamp() || role.Owner != inboxbatch.LoopStampOwned {
		t.Errorf("RoleOf(%q) = %+v, want a loop stamp: an author-owned key lets a curated edit forge the rung", rungKey, role)
	}
	if role.IsCounter() || role.IsGrowOnly() {
		t.Errorf("RoleOf(%q) = %+v, a rung is one text value, not a counter or a list", rungKey, role)
	}
	if !lifecycle.IsMoverWritten(rungKey) {
		t.Errorf("IsMoverWritten(%q) = false: the edit verb would let an operator overwrite the loop's record", rungKey)
	}
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func coverFunc(t *testing.T, pkg string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	run := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = goDir(t)
		cmd.WaitDelay = 5 * time.Second
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, acsassert.OutputTail(string(out), 30))
		}
		return string(out)
	}
	run("test", "-count=1", "-coverprofile="+profile, pkg)
	return run("tool", "cover", "-func="+profile)
}

func underFullCoverage(t *testing.T, coverOut string, keep func(path string) bool) []string {
	t.Helper()
	var short []string
	seen := 0
	for _, line := range strings.Split(coverOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "total:" || !keep(fields[0]) {
			continue
		}
		seen++
		if fields[len(fields)-1] != "100.0%" {
			short = append(short, line)
		}
	}
	if seen == 0 {
		t.Fatal("the cover output lists no function in scope: the scope filter or the package moved")
	}
	return short
}

func TestC2212_009_EveryChangedFunctionOfTheLifecyclePackageIsFullyCovered(t *testing.T) {
	out := coverFunc(t, "./internal/inboxmover/lifecycle")
	if pct, err := acsassert.FuncCoverage(out, "ReadLastFailureRung"); err != nil || pct != 100 {
		t.Errorf("ReadLastFailureRung coverage = %.1f%%, err = %v, want 100%%", pct, err)
	}
	for _, fn := range []string{"bumpWith", "parkAtCeiling", "ReleaseFromQuarantine"} {
		if pct, err := acsassert.FuncCoverage(out, fn); err != nil || pct != 100 {
			t.Errorf("%s coverage = %.1f%%, err = %v, want 100%%", fn, pct, err)
		}
	}
	for _, line := range underFullCoverage(t, out, func(string) bool { return true }) {
		t.Errorf("a lifecycle function is below 100%%: %s", line)
	}
}

func TestC2212_010_TheRungValidatorIsFullyCovered(t *testing.T) {
	out := coverFunc(t, "./internal/retryladder")
	pct, err := acsassert.FuncCoverage(out, "ParseRung")
	if err != nil {
		t.Fatalf("retryladder has no ParseRung: the one validator of a recorded rung is missing: %v", err)
	}
	if pct != 100 {
		t.Errorf("ParseRung coverage = %.1f%%, want 100%%", pct)
	}
}

func TestC2212_011_EveryChangedFunctionOfTheWriterPathIsFullyCovered(t *testing.T) {
	closeout := coverFunc(t, "./internal/cycleoutcome")
	for _, line := range underFullCoverage(t, closeout, func(string) bool { return true }) {
		t.Errorf("a cycleoutcome function is below 100%%: %s", line)
	}
	outcome := coverFunc(t, "./internal/inboxmover")
	for _, line := range underFullCoverage(t, outcome, func(path string) bool {
		return strings.Contains(path, "internal/inboxmover/outcome.go:")
	}) {
		t.Errorf("an inboxmover outcome function is below 100%%: %s", line)
	}
}
