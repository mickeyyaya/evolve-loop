package orchestrator

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func writeItem(t *testing.T, inbox, id string, fields map[string]any) {
	t.Helper()
	doc := map[string]any{"id": id, "title": "Item " + id, "weight": 0.5}
	for k, v := range fields {
		doc[k] = v
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, id+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		raw, err := os.ReadFile(path)
		out[rel] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func guardsProtected(path string) bool { return strings.HasPrefix(path, "go/internal/guards/") }

func fileIn(pkg string) []string { return []string{"go/internal/" + pkg + "/x.go"} }

type proposerCall struct {
	ids        []string
	admissible int
	window     int
}

func seedInbox(t *testing.T, inbox string) {
	t.Helper()
	writeItem(t, inbox, "a-lane", map[string]any{"files": fileIn("alpha")})
	writeItem(t, inbox, "b-lane", map[string]any{"files": fileIn("alpha")})
	writeItem(t, inbox, "c-lane", map[string]any{"files": fileIn("beta")})
	writeItem(t, inbox, "d-deps", map[string]any{"files": fileIn("gamma"), "deps": []string{"a-lane"}})
	writeItem(t, inbox, "e-route", map[string]any{"files": fileIn("delta"), "route": "console-only"})
	writeItem(t, inbox, "f-kind", map[string]any{"kind": "pipeline-repair"})
	writeItem(t, inbox, "g-prot", map[string]any{"files": []string{"go/internal/guards/g.go"}})
	writeItem(t, inbox, "h-mixed", map[string]any{"files": []string{"go/internal/guards/h.go", "go/internal/epsilon/h.go"}})
}

func newCensus(t *testing.T, root string, calls *[]proposerCall, events *eventLog) Census {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	c := Census{
		Inbox:     inboxmover.Options{InboxDir: inbox, Stderr: io.Discard},
		Rank:      inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: epoch},
		Protected: guardsProtected,
		Graph:     cellGraph(t, "alpha", "beta", "gamma", "delta", "epsilon"),
		Window:    4,
		Path:      filepath.Join(root, ".evolve", "waves", SupplyFile),
		Publish:   events.publish,
		Now:       func() time.Time { return epoch },
	}
	if calls != nil {
		var propose Proposer = func(candidates []inboxbatch.Item, admissible, window int) []Proposal {
			call := proposerCall{admissible: admissible, window: window}
			for _, it := range candidates {
				call.ids = append(call.ids, it.ID)
			}
			*calls = append(*calls, call)
			return []Proposal{{Item: "h-mixed", Console: "h-mixed-console", Lane: "h-mixed-lane"}}
		}
		c.Propose = propose
	}
	return c
}

func TestSupply_RecordsTheCauseOfEachRefusal(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	seedInbox(t, inbox)
	before := snapshotTree(t, inbox)
	events, calls := &eventLog{}, []proposerCall{}
	census := newCensus(t, root, &calls, events)

	got, err := census.Take()

	if err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string]int{ClassAdmissible: 2, ClassWaitingCell: 1, ClassWaitingDeps: 1, ClassConsoleRoute: 1, ClassConsoleKind: 1, ClassConsoleProtected: 2}
	if !reflect.DeepEqual(got.Counts, wantCounts) {
		t.Errorf("counts = %v, want %v", got.Counts, wantCounts)
	}
	causes := map[string]Refusal{}
	for _, r := range got.Refusals {
		causes[r.ID] = r
	}
	for id, want := range map[string]struct{ class, reason string }{
		"b-lane":  {ClassWaitingCell, "held by a-lane"},
		"d-deps":  {ClassWaitingDeps, "deps unmet: needs a-lane"},
		"e-route": {ClassConsoleRoute, "route:console-only"},
		"f-kind":  {ClassConsoleKind, "kind:pipeline-repair"},
		"g-prot":  {ClassConsoleProtected, "protected fix surface: go/internal/guards/g.go"},
		"h-mixed": {ClassConsoleProtected, "protected fix surface: go/internal/guards/h.go"},
	} {
		if r := causes[id]; r.Class != want.class || !strings.Contains(r.Reason, want.reason) {
			t.Errorf("refusal of %s = %+v, want class %s and a cause that holds %q", id, r, want.class, want.reason)
		}
	}
	if len(got.Refusals) != 6 {
		t.Errorf("refusals = %d, want 6: one for each item that is not admissible", len(got.Refusals))
	}
	if after := snapshotTree(t, inbox); !reflect.DeepEqual(before, after) {
		t.Errorf("the census changed the inbox: before %v, after %v; it must never route, claim or edit an item", before, after)
	}
	saved, err := ReadSupply(census.Path)
	if err != nil || len(saved) != 1 || !reflect.DeepEqual(saved[0].Counts, wantCounts) || len(saved[0].Refusals) != 6 {
		t.Errorf("record = %+v, %v, want the one census of this plan with its counts and its six refusals", saved, err)
	}
	if supply := events.withCode(CodeSupply); len(supply) != 1 || supply[0].Fields[ClassAdmissible] != "2" || supply[0].Fields["window"] != "4" ||
		!strings.Contains(supply[0].Fields["console"], "e-route: route:console-only") {
		t.Errorf("supply events = %+v, want one with the counts, the window and the console ids with their reasons", supply)
	}
	if short := events.withCode(CodeSupplyShort); len(short) != 1 || short[0].Fields[ClassAdmissible] != "2" || short[0].Fields["window"] != "4" {
		t.Errorf("supply short events = %+v, want one: 2 admissible is below the window of 4", short)
	}
	wantCall := proposerCall{ids: []string{"g-prot", "h-mixed"}, admissible: 2, window: 4}
	if !reflect.DeepEqual(calls, []proposerCall{wantCall}) || len(got.Proposals) != 1 || got.Proposals[0].Console != "h-mixed-console" {
		t.Errorf("proposer calls = %+v, proposals = %+v, want one call over the protected items and its proposal recorded", calls, got.Proposals)
	}
}

func TestSupply_TheSplitProposalStepRunsBelowTwiceTheWindowOnlyAndNilMeansNone(t *testing.T) {
	for name, tc := range map[string]struct {
		window    int
		proposer  bool
		wantCalls int
		wantShort int
	}{
		"below twice the window":       {window: 2, proposer: true, wantCalls: 1, wantShort: 0},
		"at twice the window":          {window: 1, proposer: true, wantCalls: 0, wantShort: 0},
		"below the window":             {window: 3, proposer: true, wantCalls: 1, wantShort: 1},
		"a nil proposer proposes none": {window: 4, proposer: false, wantCalls: 0, wantShort: 1},
	} {
		root := t.TempDir()
		seedInbox(t, filepath.Join(root, ".evolve", "inbox"))
		events, calls := &eventLog{}, []proposerCall{}
		var tracked *[]proposerCall
		if tc.proposer {
			tracked = &calls
		}
		census := newCensus(t, root, tracked, events)
		census.Window = tc.window

		got, err := census.Take()

		if err != nil {
			t.Fatal(err)
		}
		if len(calls) != tc.wantCalls || len(got.Proposals) != tc.wantCalls {
			t.Errorf("%s: proposer calls = %d, proposals = %d, want %d: the step runs when admissible (2) is below twice the window (%d)", name, len(calls), len(got.Proposals), tc.wantCalls, 2*tc.window)
		}
		if n := len(events.withCode(CodeSupplyShort)); n != tc.wantShort {
			t.Errorf("%s: supply short events = %d, want %d: the signal fires when admissible (2) is below the window (%d)", name, n, tc.wantShort, tc.window)
		}
	}
}

func TestSupply_CountIsReadOnlyAndPublishesNothing(t *testing.T) {
	root := t.TempDir()
	seedInbox(t, filepath.Join(root, ".evolve", "inbox"))
	events := &eventLog{}
	census := newCensus(t, root, nil, events)

	got, err := census.Count()

	if err != nil || got.Counts[ClassAdmissible] != 2 {
		t.Fatalf("Count = %+v, %v, want the 2 admissible items", got, err)
	}
	if _, statErr := os.Stat(census.Path); statErr == nil {
		t.Error("Count wrote the supply record, want a read-only run")
	}
	if len(events.events) != 0 {
		t.Errorf("Count published %d event(s), want none", len(events.events))
	}
}

func TestSupply_WithoutAModuleGraphEveryReadyItemCountsAdmissibleAndTheRecordSaysSo(t *testing.T) {
	root := t.TempDir()
	seedInbox(t, filepath.Join(root, ".evolve", "inbox"))
	census := newCensus(t, root, nil, &eventLog{})
	census.Graph = nil

	got, err := census.Count()

	if err != nil || got.Counts[ClassAdmissible] != 3 || got.Counts[ClassWaitingCell] != 0 {
		t.Fatalf("Count = %+v, %v, want the 3 ready items admissible and none waiting for a cell", got, err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "module graph") {
		t.Errorf("warnings = %v, want one that says the module graph is not loaded", got.Warnings)
	}
	empty := newCensus(t, t.TempDir(), nil, &eventLog{})
	empty.Graph = nil
	if none, err := empty.Count(); err != nil || len(none.Warnings) != 0 || none.Counts[ClassAdmissible] != 0 {
		t.Errorf("Count of an empty inbox = %+v, %v, want zero counts and no warning", none, err)
	}
}

func TestSupply_AnInboxThatCannotBeReadIsAnErrorAndTheLoadWarningsAreKept(t *testing.T) {
	root := t.TempDir()
	notADir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Dir(notADir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newCensus(t, root, nil, &eventLog{}).Count(); err == nil {
		t.Error("Count of an inbox that is a file returned no error")
	}
	events := &eventLog{}
	if _, err := newCensus(t, root, nil, events).Take(); err == nil || len(events.events) != 0 {
		t.Errorf("Take of an inbox that is a file = %v with %d event(s), want an error and nothing published", err, len(events.events))
	}

	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, "ok", map[string]any{"files": fileIn("alpha")})
	if err := os.WriteFile(filepath.Join(inbox, "broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := newCensus(t, root, nil, &eventLog{}).Count()
	if err != nil || got.Counts[ClassAdmissible] != 1 || len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "broken.json") {
		t.Errorf("Count = %+v, %v, want the readable item counted and the broken file named in the warnings", got, err)
	}
}

func TestSupply_AFailedRecordIsReportedAfterTheEventsWerePublished(t *testing.T) {
	for name, prepare := range map[string]func(path string){
		"a record that cannot be read":    func(path string) { mustMkdir(t, path) },
		"a record that cannot be written": func(path string) { mustSymlink(t, filepath.Join(t.TempDir(), "gone"), filepath.Dir(path)) },
	} {
		root := t.TempDir()
		seedInbox(t, filepath.Join(root, ".evolve", "inbox"))
		events := &eventLog{}
		census := newCensus(t, root, nil, events)
		prepare(census.Path)

		got, err := census.Take()

		if err == nil || !strings.Contains(err.Error(), "supply record") {
			t.Errorf("%s: Take = %v, want an error that names the supply record", name, err)
		}
		if got.Counts[ClassAdmissible] != 2 || len(events.withCode(CodeSupply)) != 1 {
			t.Errorf("%s: Take = %+v with %d supply event(s), want the census and its event despite the failed record", name, got, len(events.withCode(CodeSupply)))
		}
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestSupply_TheEventNamesAtMostEightConsoleItemsAndCountsTheRest(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	for i := range 11 {
		writeItem(t, inbox, "kind-"+strconv.Itoa(i), map[string]any{"kind": "pipeline-repair"})
	}
	events := &eventLog{}
	census := newCensus(t, root, nil, events)
	census.Window = 1

	got, err := census.Take()

	if err != nil || got.Counts[ClassConsoleKind] != 11 || len(got.Refusals) != 11 {
		t.Fatalf("Take = %+v, %v, want 11 console items in the record", got, err)
	}
	line := events.withCode(CodeSupply)[0].Fields["console"]
	if n := strings.Count(line, "kind:pipeline-repair"); n != 8 || !strings.HasSuffix(line, "(+3 more)") {
		t.Errorf("console field = %q, want 8 items and the count of the 3 more", line)
	}
}

func TestConsoleClass_FollowsTheReasonsThatTheRoutingAuthorityGives(t *testing.T) {
	for name, tc := range map[string]struct {
		item inboxbatch.Item
		want string
	}{
		"an explicit route":                      {inboxbatch.Item{Route: "console"}, ClassConsoleRoute},
		"a pipeline kind":                        {inboxbatch.Item{Kind: "pipeline-repair"}, ClassConsoleKind},
		"a protected file":                       {inboxbatch.Item{Files: []string{"go/internal/guards/a.go"}}, ClassConsoleProtected},
		"route lane ignored for an autofile":     {inboxbatch.Item{Route: "lane", Kind: "pipeline-x", InjectedBy: "agent"}, ClassConsoleKind},
		"route lane cannot relax a protected":    {inboxbatch.Item{Route: "lane", Files: []string{"go/internal/guards/a.go"}}, ClassConsoleProtected},
		"a route beats a pipeline kind":          {inboxbatch.Item{Route: "console", Kind: "pipeline-x"}, ClassConsoleRoute},
		"a pipeline kind beats a protected file": {inboxbatch.Item{Kind: "pipeline-x", Files: []string{"go/internal/guards/a.go"}}, ClassConsoleKind},
	} {
		routed, reason := inboxbatch.ConsoleRouted(tc.item, guardsProtected)
		if !routed {
			t.Fatalf("%s: the routing authority did not route the item to the console", name)
		}
		if got := consoleClass(reason); got != tc.want {
			t.Errorf("%s: consoleClass(%q) = %s, want %s", name, reason, got, tc.want)
		}
	}
}

func TestSupply_MeasureCarriesEachClassUnderTheNameOfTheClass(t *testing.T) {
	counts := map[string]int{}
	for i, class := range Classes() {
		counts[class] = i + 1
	}

	raw, err := json.Marshal(Supply{Counts: counts}.Measure())

	var got map[string]int
	if err != nil || json.Unmarshal(raw, &got) != nil || !reflect.DeepEqual(got, counts) {
		t.Errorf("measure JSON = %s, %v, want the six counts under the six class names %v", raw, err, counts)
	}
}

func TestSupply_TotalIsTheCountOfEveryItemTheCensusSaw(t *testing.T) {
	root := t.TempDir()
	seedInbox(t, filepath.Join(root, ".evolve", "inbox"))

	got, err := newCensus(t, root, nil, &eventLog{}).Count()

	if err != nil || got.Total() != 8 || (Supply{}).Total() != 0 {
		t.Errorf("Total = %d, %v, want the 8 inbox items and 0 for an empty record", got.Total(), err)
	}
}

func TestClasses_ListTheSixClassesOfTheSpecInOrder(t *testing.T) {
	want := []string{"admissible", "waiting_cell", "waiting_deps", "console_route", "console_kind", "console_protected"}
	if got := Classes(); !reflect.DeepEqual(got, want) {
		t.Errorf("Classes = %v, want %v (wave-orchestrator.md §4)", got, want)
	}
}

func TestSupplyRecord_KeepsTheLastRecordsAndSelectsTheLastOneOfARunWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves", SupplyFile)
	for i := range supplyKeep + 3 {
		s := Supply{At: epoch.Add(time.Duration(i) * time.Minute), Counts: map[string]int{ClassAdmissible: i}}
		if err := AppendSupply(path, s); err != nil {
			t.Fatal(err)
		}
	}

	history, err := ReadSupply(path)

	if err != nil || len(history) != supplyKeep {
		t.Fatalf("history = %d records, %v, want the last %d", len(history), err, supplyKeep)
	}
	if oldest := history[0].Counts[ClassAdmissible]; oldest != 3 {
		t.Fatalf("oldest record = #%d, want #3: the first 3 of %d fell off", oldest, supplyKeep+3)
	}
	for name, tc := range map[string]struct {
		from, to time.Time
		want     int
		ok       bool
	}{
		"the run window":       {epoch.Add(5 * time.Minute), epoch.Add(8 * time.Minute), 7, true},
		"an open-ended window": {epoch.Add(5 * time.Minute), time.Time{}, supplyKeep + 2, true},
		"before every record":  {epoch, epoch.Add(2 * time.Minute), 0, false},
		"after every record":   {epoch.Add(time.Hour), time.Time{}, 0, false},
	} {
		got, ok := SupplyIn(history, tc.from, tc.to)
		if ok != tc.ok || (ok && got.Counts[ClassAdmissible] != tc.want) {
			t.Errorf("%s: SupplyIn = %v, %v, want #%d, %v", name, got.Counts, ok, tc.want, tc.ok)
		}
	}
}

func TestSupplyRecord_AnAbsentFileIsEmptyAndABrokenOneIsAnError(t *testing.T) {
	dir := t.TempDir()
	if history, err := ReadSupply(filepath.Join(dir, "none.json")); err != nil || history != nil {
		t.Errorf("ReadSupply of an absent file = %v, %v, want nil and no error", history, err)
	}
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSupply(broken); err == nil {
		t.Error("ReadSupply of a broken file returned no error")
	}
	if err := AppendSupply(broken, Supply{}); err == nil {
		t.Error("AppendSupply over a broken file returned no error, want it to refuse so the operator sees it")
	}
	if _, err := ReadSupply(dir); err == nil {
		t.Error("ReadSupply of a directory returned no error")
	}
}

func TestSupplyEvents_RegisterTheirCodesWithTheSpecNames(t *testing.T) {
	for code, want := range map[signalcenter.Code]string{CodeSupply: "LOOP_ORCH_SUPPLY", CodeSupplyShort: "LOOP_ORCH_SUPPLY_SHORT"} {
		if string(code) != want {
			t.Errorf("code = %q, want %q", code, want)
		}
	}
}

func TestSupply_CountRefusesACensusWithNoClock(t *testing.T) {
	census := newCensus(t, t.TempDir(), nil, &eventLog{})
	census.Now = nil

	if _, err := census.Count(); !errors.Is(err, errCensusClock) {
		t.Errorf("Count with no clock = %v, want the error that names the clock", err)
	}
}

func TestSupplyRecord_AnAppendHoldsTheFileLockAcrossItsReadModifyWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves", SupplyFile)
	held, err := flock.Lock(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	short := supplyStore{lock: flock.LockWithin, write: atomicwrite.JSON, wait: 50 * time.Millisecond}

	err = short.append(path, Supply{Window: 1})

	if !errors.Is(err, flock.ErrLockDeadline) || !strings.Contains(err.Error(), "lock the supply record") {
		t.Fatalf("append while another writer holds the lock = %v, want the lock deadline, named as the supply record lock", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("the record was written without the lock")
	}
	held()
	if err := short.append(path, Supply{Window: 2}); err != nil {
		t.Errorf("append after the lock was released = %v, want success", err)
	}
}

func TestSupplyRecord_ConcurrentAppendsLoseNoRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves", SupplyFile)
	writers := supplyKeep - 2
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendSupply(path, Supply{Window: i}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	history, err := ReadSupply(path)
	windows := map[int]bool{}
	for _, s := range history {
		windows[s.Window] = true
	}
	if err != nil || len(windows) != writers {
		t.Errorf("history = %d distinct records, %v, want %d: a read-modify-write without a lock loses updates", len(windows), err, writers)
	}
}

func TestSupplyRecord_AFailedLockAndAFailedWriteAreErrorsThatNameTheRecord(t *testing.T) {
	boom := errors.New("disk is gone")
	path := filepath.Join(t.TempDir(), "waves", SupplyFile)
	noLock := supplyStore{lock: func(string, time.Duration, func()) (func(), error) { return nil, boom }, write: atomicwrite.JSON}
	noWrite := supplyStore{lock: flock.LockWithin, write: func(string, any) error { return boom }, wait: time.Second}

	for name, tc := range map[string]struct {
		store supplyStore
		want  string
	}{
		"a lock that fails":  {noLock, "lock the supply record"},
		"a write that fails": {noWrite, "write the supply record"},
	} {
		if err := tc.store.append(path, Supply{}); !errors.Is(err, boom) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: append = %v, want %q around the cause", name, err, tc.want)
		}
	}
}
