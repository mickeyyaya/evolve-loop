//go:build acs

package cycle2668

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

const cycleUnderTest = 2668

const heldSnapshot = "snapshot-held-by-the-item"

type lane struct {
	root      string
	evolveDir string
	workspace string
	inbox     string
}

func newLane(t *testing.T, pin []string, bystanders ...string) lane {
	t.Helper()
	root := inboxtest.EmptyRoot(t)
	l := lane{
		root:      root,
		evolveDir: filepath.Join(root, ".evolve"),
		workspace: filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycleUnderTest)),
		inbox:     filepath.Join(root, ".evolve", "inbox"),
	}
	if err := os.MkdirAll(l.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range append(append([]string{}, pin...), bystanders...) {
		inboxtest.WriteItem(t, l.inbox, id)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: cycleUnderTest, ItemIDs: pin}); err != nil {
		t.Fatal(err)
	}
	return l
}

func (l lane) recordPhases(t *testing.T, entries ...phasetiming.Entry) {
	t.Helper()
	body, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	inboxtest.WriteFile(t, phasetiming.Path(l.workspace), string(body))
}

func (l lane) closeOut(t *testing.T) {
	t.Helper()
	var stderr bytes.Buffer
	in := cycleoutcome.FailureInputsFor(l.root, l.evolveDir, l.workspace, cycleUnderTest, &stderr)
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v\n%s", err, stderr.String())
	}
}

func (l lane) item(t *testing.T, id string) map[string]any {
	t.Helper()
	loc, err := lifecycle.Locate(l.inbox, id)
	if err != nil {
		t.Fatalf("item %q is nowhere under %s: %v", id, l.inbox, err)
	}
	return inboxtest.ItemFields(t, loc.Path)
}

func (l lane) failureCount(t *testing.T, id string) int {
	t.Helper()
	count, _ := l.item(t, id)["failure_count"].(float64)
	return int(count)
}

func passed(phase string) phasetiming.Entry {
	return phasetiming.Entry{Phase: phase, Verdict: "PASS"}
}

func refused(phase, code string) phasetiming.Entry {
	return phasetiming.Entry{Phase: phase, Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{
		{Severity: cyclestate.SeverityError, Code: code, Message: "phase " + phase + " failed: " + code},
	}}
}

func TestC2668_001_ARealPreBuildFailureChargesThePinnedItem(t *testing.T) {
	cases := []struct {
		name   string
		phases []phasetiming.Entry
		want   int
	}{
		{"a coded scout failure", []phasetiming.Entry{refused("scout", "SCOUT_REPORT_REJECTED")}, 1},
		{"a coded triage failure after a passed scout", []phasetiming.Entry{passed("scout"), refused("triage", "TRIAGE_BRIEF_REJECTED")}, 1},
		{"a coded failure after build stays uncharged", []phasetiming.Entry{passed("scout"), passed("triage"), passed("build"), refused("audit", "AUDIT_BRIEF_REJECTED")}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLane(t, []string{"pinned"}, "bystander")
			l.recordPhases(t, tc.phases...)
			l.closeOut(t)
			if got := l.failureCount(t, "pinned"); got != tc.want {
				t.Errorf("pinned item failure_count = %d, want %d", got, tc.want)
			}
			if got := l.failureCount(t, "bystander"); got != 0 {
				t.Errorf("an item outside the lineage record was charged: failure_count = %d", got)
			}
		})
	}
}

func TestC2668_002_APreBuildFailureThatCannotParkKeepsTheContinuation(t *testing.T) {
	cases := []struct {
		name        string
		parkBlocked bool
	}{
		{"a park that fails keeps the stamp the item holds", true},
		{"a park that succeeds retires the stamp with the item", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLane(t, []string{"pinned"})
			l.recordPhases(t, passed("scout"), refused("triage", cyclestate.DiagCodeTriageTopNEmpty))
			held := map[string]any{"id": "pinned", "kind": "bug", "title": "fixture item pinned", "failure_count": 5,
				"continuation": map[string]any{"snapshot_sha": heldSnapshot, "branch": "cycle-held", "cycle": 2600}}
			body, err := json.Marshal(held)
			if err != nil {
				t.Fatal(err)
			}
			loc, err := lifecycle.Locate(l.inbox, "pinned")
			if err != nil {
				t.Fatal(err)
			}
			inboxtest.WriteFile(t, loc.Path, string(body))
			if tc.parkBlocked {
				inboxtest.WriteFile(t, filepath.Join(l.inbox, "quarantine"), "a file where the park directory must be")
			}
			l.closeOut(t)

			var final map[string]any
			if tc.parkBlocked {
				final = l.item(t, "pinned")
			} else {
				parked, err := lifecycle.FindFileByTaskID(filepath.Join(l.inbox, "quarantine"), "pinned")
				if err != nil {
					t.Fatalf("the item at its retry ceiling was not parked: %v", err)
				}
				final = inboxtest.ItemFields(t, parked)
			}
			stamp, hasStamp := final["continuation"].(map[string]any)
			switch {
			case tc.parkBlocked && (!hasStamp || stamp["snapshot_sha"] != heldSnapshot):
				t.Errorf("the item was not parked and lost its continuation: %v", final)
			case !tc.parkBlocked && hasStamp:
				t.Errorf("a parked item still carries a continuation: %v", stamp)
			}
		})
	}
}

func TestC2668_003_AHostFaultChargesNoItemWhileARealFaultStillDoes(t *testing.T) {
	infraSentinel := `<!-- evolve-verdict: {"phase":"scout","verdict":"FAIL","failure":{"class":"infrastructure"}} -->` + "\n"
	cases := []struct {
		name      string
		phases    []phasetiming.Entry
		sentinel  string
		wantCount int
	}{
		{"a refusal coded as a host fault", []phasetiming.Entry{passed("scout"), refused("triage", cyclestate.DiagCodeTriageLanePinMissing)}, "", 0},
		{"the same shape coded as a real fault", []phasetiming.Entry{passed("scout"), refused("triage", "TRIAGE_BRIEF_REJECTED")}, "", 1},
		{"a scout that the host infrastructure failed", []phasetiming.Entry{refused("scout", "SCOUT_REPORT_REJECTED")}, infraSentinel, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLane(t, []string{"pinned"})
			l.recordPhases(t, tc.phases...)
			if tc.sentinel != "" {
				inboxtest.WriteFile(t, filepath.Join(l.workspace, "scout-report.md"), tc.sentinel)
			}
			l.closeOut(t)
			if got := l.failureCount(t, "pinned"); got != tc.wantCount {
				t.Errorf("pinned item failure_count = %d, want %d", got, tc.wantCount)
			}
		})
	}
}
