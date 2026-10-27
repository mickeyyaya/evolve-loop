package fixability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	acFiveConsole = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	mergeConsole  = "retro-minting-files-through-inbox-add-console"
)

func testFence(p string) bool {
	return strings.HasPrefix(p, "go/internal/guards/") || strings.HasPrefix(p, "go/internal/inboxmover/lifecycle/")
}

type round struct {
	Failure      phasecontract.FailureBlock `json:"failure"`
	GateFailures []string                   `json:"gate_failures"`
}

func cycle2035(t *testing.T, name string) Input {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cycle-2035", name))
	if err != nil {
		t.Fatal(err)
	}
	var r round
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return Input{
		Defects: r.Failure.Defects, GateFailures: r.GateFailures,
		OpenConsoleItems: []string{acFiveConsole, mergeConsole}, Protected: testFence,
		Salvage: "/dev/cl-c2035-lane@fix/c2035",
	}
}

func ids(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}

func TestClassify_Cycle2035Round1StopsOnTheProtectedBlockerM1(t *testing.T) {
	t.Parallel()
	d := Classify(cycle2035(t, "round1.json"))

	if !d.Stop || d.Reason != ReasonHandoffBlocker {
		t.Fatalf("round 1 = %+v, want a stop with %s", d, ReasonHandoffBlocker)
	}
	if got := ids(d.Blockers); !slices.Equal(got, []string{"M1"}) {
		t.Errorf("blockers = %v, want M1 (the audit proved its tests red)", got)
	}
	if !slices.Contains(d.ConsoleItems, acFiveConsole) || d.Salvage != "/dev/cl-c2035-lane@fix/c2035" {
		t.Errorf("decision = %+v, want the console item %s and the salvage pointer", d, acFiveConsole)
	}
}

func TestClassify_ANonBlockingHandoffFindingIsListedAndDoesNotStop(t *testing.T) {
	t.Parallel()
	in := cycle2035(t, "round1.json")
	in.Defects = in.Defects[1:]

	d := Classify(in)

	if d.Stop || !slices.Equal(ids(d.Handoff), []string{"M2"}) || len(d.Blockers) != 0 {
		t.Errorf("M2 alone = %+v, want M2 as a hand-off finding and no stop: MEDIUM, no gate failure, no red test", d)
	}
}

func TestClassify_TheLaneFindingH1AloneDoesNotStop(t *testing.T) {
	t.Parallel()
	in := cycle2035(t, "round3.json")
	in.Defects, in.GateFailures = in.Defects[:1], nil

	d := Classify(in)

	if d.Stop || !slices.Equal(ids(d.Lane), []string{"H1"}) || len(d.Handoff) != 0 {
		t.Errorf("H1 alone = %+v, want one lane finding and no stop", d)
	}
}

func TestClassify_Cycle2035Round3SeparatesTheLaneFixFromTheBlocker(t *testing.T) {
	t.Parallel()
	d := Classify(cycle2035(t, "round3.json"))

	if !d.Stop || !slices.Equal(ids(d.Lane), []string{"H1", "M3"}) || !slices.Equal(ids(d.Handoff), []string{"M1", "M2"}) {
		t.Errorf("round 3 = %+v, want lane H1 and M3, hand-off M1 and M2, and a stop", d)
	}
	m1 := d.Blockers[0]
	if m1.ID != "M1" || !strings.Contains(m1.Blocks, "gate") {
		t.Errorf("M1 = %+v, want it blocked by the ship gate that names its package", m1)
	}
}

func TestClassify_Cycle2035Round4StopsOnTheGateThatNamesTheTests(t *testing.T) {
	t.Parallel()
	d := Classify(cycle2035(t, "round4.json"))

	if !d.Stop || !slices.Equal(ids(d.Blockers), []string{"M1"}) {
		t.Errorf("round 4 = %+v, want a stop on M1", d)
	}
}

func TestClassify_AHighFindingOnAFencePathBlocks(t *testing.T) {
	t.Parallel()
	d := Classify(Input{Defects: []string{"H2: go/internal/guards/integrity_surface.go:54 misses a fragment"}, Protected: testFence})

	if !d.Stop || len(d.Blockers) != 1 || d.Blockers[0].Severity != "HIGH" || !slices.Equal(d.Blockers[0].Paths, []string{"go/internal/guards/integrity_surface.go"}) {
		t.Errorf("decision = %+v, want H2 on the fence to block as HIGH", d)
	}
}

func TestClassify_NoFindingNoStop(t *testing.T) {
	t.Parallel()
	if d := Classify(Input{}); d.Stop || d.Reason != "" || len(d.Lane)+len(d.Handoff) != 0 {
		t.Errorf("empty = %+v, want no finding and no stop", d)
	}
}

func TestClassify_ARedTestNeedsTheFailVerbNextToTheTests(t *testing.T) {
	t.Parallel()
	in := Input{OpenConsoleItems: []string{mergeConsole}}
	for text, blocks := range map[string]bool{
		"M1: floor tests fail (3) with this diff; fix is " + mergeConsole:      true,
		"M1: floor tests (3) fail with this diff; fix is " + mergeConsole:      true,
		"M1: the predicate failed; see " + mergeConsole:                        true,
		"M2: add a regression test that fails before the fix; " + mergeConsole: false,
		"M2: the tests no longer fail after " + mergeConsole:                   false,
	} {
		in.Defects = []string{text}
		if d := Classify(in); d.Stop != blocks {
			t.Errorf("%q: stop = %t, want %t", text, d.Stop, blocks)
		}
	}
}

func TestParseFinding_ReadsTheIDAndTheSeverityLetter(t *testing.T) {
	t.Parallel()
	for text, want := range map[string][2]string{
		"C1: a crash": {"C1", "CRITICAL"}, "H3: x": {"H3", "HIGH"}, "M2: y": {"M2", "MEDIUM"}, "L1: z": {"L1", "LOW"},
		"no id here": {"", ""},
	} {
		f := parseFinding(text)
		if f.ID != want[0] || f.Severity != want[1] {
			t.Errorf("parseFinding(%q) = %q %q, want %q %q", text, f.ID, f.Severity, want[0], want[1])
		}
	}
}

func TestNamesItem_NeedsTheWholeID(t *testing.T) {
	t.Parallel()
	if namesItem("see retro-minting-files-through-inbox-add-console-two", mergeConsole) {
		t.Error("a longer id is another item")
	}
	if namesItem("a hotfix-x here", "fix-x") || !namesItem("a hotfix-x and fix-x", "fix-x") {
		t.Error("the id must also start at a word edge, and a later whole match still counts")
	}
	if !namesItem("queued as "+mergeConsole+".", mergeConsole) || !namesItem(mergeConsole, mergeConsole) {
		t.Error("the id at the end of a sentence or alone names the item")
	}
}
