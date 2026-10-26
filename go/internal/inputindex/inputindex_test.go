package inputindex

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

type fixture struct {
	t     *testing.T
	root  string
	items map[int][]string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return fixture{t: t, root: root, items: map[int][]string{}}
}

func (f fixture) ws(cycle int) string {
	return filepath.Join(f.root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}

func (f fixture) write(rel, body string) string {
	f.t.Helper()
	path := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f fixture) cycle(n int, item string, files map[string]string) {
	f.t.Helper()
	if item != "" {
		f.items[n] = []string{item}
		if err := WriteRecord(f.root, Record{Cycle: n, ItemIDs: f.items[n]}); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := os.MkdirAll(f.ws(n), 0o755); err != nil {
		f.t.Fatal(err)
	}
	for name, body := range files {
		f.write(filepath.Join(".evolve", "runs", "cycle-"+strconv.Itoa(n), name), body)
	}
}

func (f fixture) build(cycle int, phase string) Index {
	f.t.Helper()
	idx, err := Build(Request{ProjectRoot: f.root, Workspace: f.ws(cycle), Cycle: cycle, Phase: phase, ItemIDs: f.items[cycle]})
	if err != nil {
		f.t.Fatal(err)
	}
	return idx
}

func entryOf(idx Index, kind string, cycle int) (Entry, bool) {
	i := slices.IndexFunc(idx.Entries, func(e Entry) bool { return e.Kind == kind && e.Cycle == cycle })
	if i < 0 {
		return Entry{}, false
	}
	return idx.Entries[i], true
}

func TestBuild_ListsTheDeliverablesOfTheEarlierPhasesAndNotTheOwnOutput(t *testing.T) {
	f := newFixture(t)
	scout := "<!-- challenge-token: abc -->\n# Scout: replay harness for pass^k\nbody\n"
	f.cycle(7, "item-a", map[string]string{
		"scout-report.md":        scout,
		"test-report.md":         "# TDD: 3 RED tests\n",
		"build-report.md":        "# Build of an earlier attempt\n",
		"handoff-tdd-build.json": `{"summary":"two asks for the builder","asks":[]}`,
		"scout-stdout.log":       "raw\n",
		"build-x-final-pane.txt": "raw\n",
	})
	idx := f.build(7, "build")
	got, ok := entryOf(idx, "scout-report", 7)
	if !ok {
		t.Fatalf("the build index must list the scout report, got %+v", idx.Entries)
	}
	sum := sha256.Sum256([]byte(scout))
	want := Entry{ID: got.ID, Class: ClassDeliverable, Kind: "scout-report", Path: ".evolve/runs/cycle-7/scout-report.md", Cycle: 7, Phase: "scout", Attempt: 1,
		Summary: "Scout: replay harness for pass^k", Bytes: int64(len(scout)), SHA256: hex.EncodeToString(sum[:])}
	if got != want {
		t.Fatalf("scout entry = %+v, want %+v", got, want)
	}
	if e, ok := entryOf(idx, "handoff-asks", 7); !ok || e.Summary != "two asks for the builder" || e.Class != ClassDeliverable {
		t.Fatalf("the handoff asks must travel in the index as a deliverable, got %+v", idx.Entries)
	}
	if _, ok := entryOf(idx, "test-report", 7); !ok {
		t.Fatal("the tdd report must be listed")
	}
	if _, ok := entryOf(idx, "build-report", 7); ok {
		t.Fatal("the own output of the phase is not an input")
	}
	for _, e := range idx.Entries {
		if IsRawOutput(e.Path) {
			t.Fatalf("the index lists raw output %s", e.Path)
		}
	}
	if idx.Schema != Schema || idx.Phase != "build" || idx.Cycle != 7 || !slices.Equal(idx.ItemIDs, []string{"item-a"}) {
		t.Fatalf("index header = %+v", idx)
	}
}

func TestBuild_ARetryListsTheLineageOfItsItemAndNoUnrelatedCycle(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", map[string]string{
		"retrospective-report.md":      "# Audit FAIL: add a read-only seam\n",
		"failure-digest.json":          `{"summary":"the harness wrote to the replay set"}`,
		"carryover-todos.json":         `{"todos":[]}`,
		"audit-report.md":              "# Audit: FAIL\n",
		"retrospectives/recovery-6.md": "# Recovery of cycle 6\n",
		"failure-lesson-6.yaml":        "summary: keep the replay set read-only\n",
		"build-stdout.log":             "raw\n",
	})
	f.cycle(9, "item-b", map[string]string{"audit-report.md": "# Audit of an unrelated item\n"})
	f.cycle(7, "item-a", nil)
	idx := f.build(7, "scout")
	if !slices.Equal(idx.Lineage, []int{6}) || idx.Attempt != 2 {
		t.Fatalf("lineage = %v attempt = %d, want [6] and 2", idx.Lineage, idx.Attempt)
	}
	for kind, summary := range map[string]string{
		"retrospective": "Audit FAIL: add a read-only seam", "failure-digest": "the harness wrote to the replay set",
		"carryover": "", "audit-report": "Audit: FAIL", "recovery-retrospective": "Recovery of cycle 6", "failure-lesson": "keep the replay set read-only",
	} {
		e, ok := entryOf(idx, kind, 6)
		if !ok || e.Class != ClassLineage || e.Summary != summary || e.Attempt != 1 {
			t.Errorf("lineage entry %s = %+v (found %v), want summary %q at attempt 1", kind, e, ok, summary)
		}
	}
	for _, e := range idx.Entries {
		if e.Cycle == 9 || IsRawOutput(e.Path) {
			t.Fatalf("the index lists %s", e.Path)
		}
	}
}

func TestLineageComesOnlyFromPipelineRecords(t *testing.T) {
	f := newFixture(t)
	f.cycle(5, "item-z", map[string]string{"retrospective-report.md": "# this text names item-a but the records do not\n", "lane-scope.json": `{"todo_ids":["item-a"]}`})
	f.cycle(9, "", map[string]string{"audit-report.md": "# a cell planted a lane scope here\n", "lane-scope.json": `{"todo_ids":["item-a"]}`})
	f.cycle(7, "item-a", map[string]string{"scout-report.md": "# lineage: 5\n", "inputs/lineage.json": `[5]`, "lane-scope.json": `{"todo_ids":["item-z"]}`})
	if err := continuation.WriteRegistryEntry(f.root, "item-a", continuation.Continuation{SnapshotSHA: "abc", Cycle: 4}); err != nil {
		t.Fatal(err)
	}
	f.cycle(4, "", map[string]string{"retrospective-report.md": "# ancestor of the continuation chain\n"})
	idx := f.build(7, "build")
	if !slices.Equal(idx.Lineage, []int{4}) {
		t.Fatalf("lineage = %v, want [4]: only the pipeline lineage records and the continuation registry name a lineage cycle; a lane-scope.json that a cell can write never does", idx.Lineage)
	}
	if _, ok := entryOf(idx, "retrospective", 4); !ok {
		t.Fatal("the continuation ancestor's retrospective must be listed")
	}
}

func TestLineage_KeepsTheLastLineageMaxCycles(t *testing.T) {
	f := newFixture(t)
	for n := 1; n <= 7; n++ {
		f.cycle(n, "item-a", nil)
	}
	f.cycle(8, "item-a", nil)
	idx, err := Build(Request{ProjectRoot: f.root, Workspace: f.ws(8), Cycle: 8, Phase: "build", LineageMax: 3, ItemIDs: f.items[8]})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(idx.Lineage, []int{5, 6, 7}) || idx.Attempt != 8 {
		t.Fatalf("lineage = %v attempt = %d, want [5 6 7] and 8", idx.Lineage, idx.Attempt)
	}
	if got := f.build(8, "build").Lineage; len(got) != DefaultLineageMax {
		t.Fatalf("the default cap must keep %d cycles, got %v", DefaultLineageMax, got)
	}
}

func TestBuild_ListsTheLessonsThatNameTheItemOrALineageCycle(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", nil)
	f.cycle(7, "item-a", nil)
	f.write(".evolve/instincts/lessons/by-item.yaml", "summary: names item-a\n")
	f.write(".evolve/instincts/lessons/by-cycle.yaml", "title: from cycle-6\n")
	f.write(".evolve/instincts/lessons/unrelated.yaml", "summary: cycle-60 item-ab\n")
	idx := f.build(7, "build")
	var paths []string
	for _, e := range idx.Entries {
		if e.Kind == "lesson" {
			paths = append(paths, e.Path)
		}
	}
	want := []string{".evolve/instincts/lessons/by-cycle.yaml", ".evolve/instincts/lessons/by-item.yaml"}
	if !slices.Equal(paths, want) {
		t.Fatalf("lessons = %v, want %v", paths, want)
	}
}

func TestBuild_NotesACycleWithNoItemID(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", nil)
	f.cycle(7, "", map[string]string{"scout-report.md": "# s\n"})
	idx := f.build(7, "build")
	if len(idx.Lineage) != 0 || idx.Attempt != 1 || len(idx.Notes) != 1 || !strings.Contains(idx.Notes[0], "no item id") {
		t.Fatalf("index = %+v, want no lineage and a note that the cycle has no item id", idx)
	}
}

func TestBuild_RefusesARequestWithoutAWorkspaceOrRoot(t *testing.T) {
	for _, req := range []Request{{ProjectRoot: "/r"}, {Workspace: "/w"}, {ProjectRoot: "r", Workspace: "/w"}} {
		if _, err := Build(req); err == nil {
			t.Errorf("Build(%+v) must fail", req)
		}
	}
}

func TestBuild_FailsLoudlyWhenADeliverableCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.cycle(7, "item-a", nil)
	if err := os.MkdirAll(filepath.Join(f.ws(7), "scout-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(Request{ProjectRoot: f.root, Workspace: f.ws(7), Cycle: 7, Phase: "build"}); err == nil {
		t.Fatal("an unreadable deliverable must fail the build of the index")
	}
}

func TestWriteReadAndReadablePaths(t *testing.T) {
	f := newFixture(t)
	ws := f.ws(7)
	idx := Index{Schema: Schema, Cycle: 7, Entries: []Entry{
		{Path: ".evolve/runs/cycle-6/retrospective-report.md"}, {Path: "../outside"}, {Path: "/etc/passwd"}, {Path: ""},
	}}
	if err := Write(ws, idx); err != nil {
		t.Fatal(err)
	}
	got, err := ReadablePaths(f.root, ws)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{Path(ws), filepath.Join(f.root, ".evolve", "runs", "cycle-6", "retrospective-report.md")}
	if !slices.Equal(got, want) {
		t.Fatalf("ReadablePaths = %v, want %v: a path outside the root is never granted", got, want)
	}
}

func TestRead_RefusesAMissingAMalformedOrAForeignIndex(t *testing.T) {
	f := newFixture(t)
	ws := f.ws(7)
	if _, err := Read(ws); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing index: err = %v", err)
	}
	if _, err := ReadablePaths(f.root, ws); err == nil {
		t.Fatal("ReadablePaths of a missing index must fail")
	}
	f.write(".evolve/runs/cycle-7/inputs/index.json", "{")
	if _, err := Read(ws); err == nil {
		t.Fatal("a malformed index must fail")
	}
	f.write(".evolve/runs/cycle-7/inputs/index.json", `{"schema":"other/1"}`)
	if _, err := Read(ws); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("a foreign schema must fail, got %v", err)
	}
}

func TestWrite_FailsWhenTheInputsDirectoryCannotBeMade(t *testing.T) {
	f := newFixture(t)
	f.write("ws", "a file where the workspace should be")
	if err := Write(filepath.Join(f.root, "ws"), Index{}); err == nil {
		t.Fatal("Write must fail when the inputs directory cannot be made")
	}
}

func TestIsRawOutput(t *testing.T) {
	for path, want := range map[string]bool{
		"a/scout-stdout.log": true, "a/build-stderr.log": true, "a/audit-stdout.clean.txt": true,
		"a/build-s-final-pane.txt": true, "a/tmux-final-scrollback.txt": true, "a/build-pane-watch.json": true,
		"a/.bridge/x.md": true, "a/scout-report.md": false, "a/stdout.md": false,
	} {
		if got := IsRawOutput(path); got != want {
			t.Errorf("IsRawOutput(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSummaryOf(t *testing.T) {
	long := strings.Repeat("x", 300)
	for name, c := range map[string]struct{ file, body, want string }{
		"md heading after a comment": {"a.md", "<!-- t -->\n\n## Heading two\n", "Heading two"},
		"md without heading":         {"a.md", "plain first line\n# later\n", "plain first line"},
		"md empty":                   {"a.md", "\n\n", ""},
		"json summary":               {"a.json", `{"summary":"  one\nline "}`, "one line"},
		"json without summary":       {"a.json", `{"x":1}`, ""},
		"json array":                 {"a.json", `[1]`, ""},
		"yaml title":                 {"a.yaml", "id: 1\ntitle: \"the title\"\n", "the title"},
		"yaml none":                  {"a.yaml", "id: 1\n", ""},
		"other ext":                  {"a.txt", "text", ""},
		"long":                       {"a.md", "# " + long, strings.Repeat("x", summaryMax)},
	} {
		if got := summaryOf(c.file, []byte(c.body)); got != c.want {
			t.Errorf("%s: summaryOf = %q, want %q", name, got, c.want)
		}
	}
}

func TestBuild_NotesAnUnreadableContinuationRegistry(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", nil)
	f.cycle(7, "item-a", nil)
	f.write(".evolve/continuation-registry.json", "{")
	idx := f.build(7, "build")
	if !slices.Equal(idx.Lineage, []int{6}) || len(idx.Notes) != 1 || !strings.Contains(idx.Notes[0], "continuation registry is unreadable") {
		t.Fatalf("index = %+v, want lineage [6] and a note on the registry", idx)
	}
}

func TestRawOutputSuffixes_AreTheSuffixesThatIsRawOutputRefuses(t *testing.T) {
	suffixes := RawOutputSuffixes()
	if len(suffixes) == 0 {
		t.Fatal("no raw output suffixes")
	}
	for _, s := range suffixes {
		if !IsRawOutput("ws/agent" + s) {
			t.Errorf("IsRawOutput must refuse the suffix %s", s)
		}
	}
	suffixes[0] = "changed"
	if RawOutputSuffixes()[0] == "changed" {
		t.Fatal("RawOutputSuffixes must return a copy")
	}
}

func TestBuild_SkipsADeliverableThatIsASymlinkSoNoTargetLeaksIntoTheIndex(t *testing.T) {
	f := newFixture(t)
	secret := f.write("home/.ssh/id_rsa", "PRIVATE KEY LINE\n")
	f.cycle(7, "item-a", nil)
	if err := os.Symlink(secret, filepath.Join(f.ws(7), "scout-report.md")); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.build(7, "build").Entries {
		if e.Kind == "scout-report" || strings.Contains(e.Summary, "PRIVATE") {
			t.Fatalf("a planted symlink must not become an entry: %+v", e)
		}
	}
}

func TestWriteRecord_KeepsTheItemIDsOfACycleWhereNoCellCanWrite(t *testing.T) {
	f := newFixture(t)
	if err := WriteRecord(f.root, Record{Cycle: 3, ItemIDs: []string{"item-a"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(f.root, ".evolve", "lineage", "cycle-3.json"))
	if err != nil || !strings.Contains(string(raw), "item-a") {
		t.Fatalf("record = %q err = %v", raw, err)
	}
	f.write("file", "x")
	if err := WriteRecord(filepath.Join(f.root, "file"), Record{Cycle: 3}); err == nil {
		t.Fatal("WriteRecord must fail when its directory cannot be made")
	}
}

func TestLineage_NotesAMalformedRecordAndSkipsIt(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", nil)
	f.write(".evolve/lineage/cycle-5.json", "{")
	f.write(".evolve/lineage/other.json", `{"cycle":4,"item_ids":["item-a"]}`)
	got, notes := Lineage(f.root, 7, []string{"item-a"})
	if !slices.Equal(got, []int{6}) || len(notes) != 1 || !strings.Contains(notes[0], "cycle-5.json") {
		t.Fatalf("lineage = %v notes = %v, want [6] and a note on the malformed record", got, notes)
	}
}

func TestBuild_SkipsAnEntryUnderASymlinkedDirectoryOrWithASecondHardLink(t *testing.T) {
	f := newFixture(t)
	f.cycle(6, "item-a", nil)
	f.cycle(7, "item-a", nil)
	outside := f.write("home/docs/notes.md", "# HOME-DOC-LINE\n")
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(f.ws(6), "retrospectives")); err != nil {
		t.Fatal(err)
	}
	linked := f.write(".evolve/runs/cycle-6/audit-report.md", "# hard linked\n")
	if err := os.Link(linked, filepath.Join(f.root, "home", "copy.md")); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.build(7, "build").Entries {
		if e.Kind == "recovery-retrospective" || e.Kind == "audit-report" {
			t.Fatalf("an entry through a symlinked directory or with a second hard link must be skipped: %+v", e)
		}
	}
}

func TestReadRecord_ReturnsTheItemIDsThatThePipelineRecorded(t *testing.T) {
	f := newFixture(t)
	if _, ok := ReadRecord(f.root, 3); ok {
		t.Fatal("no record yet")
	}
	if err := WriteRecord(f.root, Record{Cycle: 3, ItemIDs: []string{"item-a"}}); err != nil {
		t.Fatal(err)
	}
	if r, ok := ReadRecord(f.root, 3); !ok || !slices.Equal(r.ItemIDs, []string{"item-a"}) || r.Cycle != 3 {
		t.Fatalf("ReadRecord = %+v %v", r, ok)
	}
	f.write(".evolve/lineage/cycle-4.json", "{")
	if _, ok := ReadRecord(f.root, 4); ok {
		t.Fatal("a malformed record is no record")
	}
}
