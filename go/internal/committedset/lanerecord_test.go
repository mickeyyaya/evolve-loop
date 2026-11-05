package committedset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func laneFixture(t *testing.T, cycle string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", cycle)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLanePin_TheHostRecordWinsOverAForgedWorkspaceFile(t *testing.T) {
	t.Parallel()
	root, ws := laneFixture(t, "cycle-7")
	if err := WriteRecord(root, Record{Cycle: 7, ItemIDs: []string{"a"}, GoalHash: "h1"}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["b","c"],"goal_hash":"forged"}`)
	if got := LanePin(ws); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("LanePin = %v, want [a]", got)
	}
	doc, ok := LaneDoc(ws)
	if !ok || doc.GoalHash != "h1" || !slices.Equal(doc.TodoIDs, []string{"a"}) {
		t.Fatalf("LaneDoc = %+v %v, want the record", doc, ok)
	}
}

func TestLanePin_ARecordWithNoIDsMeansNoPin(t *testing.T) {
	t.Parallel()
	root, ws := laneFixture(t, "cycle-8")
	if err := WriteRecord(root, Record{Cycle: 8}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["b"]}`)
	if got := LanePin(ws); got != nil {
		t.Fatalf("LanePin = %v, want nil", got)
	}
	if doc, ok := LaneDoc(ws); !ok || len(doc.TodoIDs) != 0 {
		t.Fatalf("LaneDoc = %+v %v, want an empty doc", doc, ok)
	}
}

func TestLanePin_WithoutARecordTheFileIsRead(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-9")
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["x"," y "],"goal_hash":"g"}`)
	if got := LanePin(ws); !slices.Equal(got, []string{"x", "y"}) {
		t.Fatalf("LanePin = %v, want [x y]", got)
	}
	if doc, ok := LaneDoc(ws); !ok || doc.GoalHash != "g" {
		t.Fatalf("LaneDoc = %+v %v", doc, ok)
	}
}

func TestLanePin_AnotherWorkspaceShapeReadsTheFile(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"plain", filepath.Join("runs", "cycle-3"), filepath.Join(".evolve", "runs", "cycle-x"), filepath.Join(".evolve", "other", "cycle-3")} {
		ws := filepath.Join(t.TempDir(), rel)
		writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["z"]}`)
		if got := LanePin(ws); !slices.Equal(got, []string{"z"}) {
			t.Fatalf("%s: LanePin = %v, want [z]", rel, got)
		}
	}
}

func TestLaneDoc_NoSourceAndMalformedFile(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-4")
	if _, ok := LaneDoc(ws); ok {
		t.Fatal("no record and no file must report no doc")
	}
	if _, ok := LaneDoc(""); ok {
		t.Fatal("an empty workspace must report no doc")
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `not json`)
	if _, ok := LaneDoc(ws); ok {
		t.Fatal("a malformed file must report no doc")
	}
}

func TestLaneDrift_NamesBothListsOnlyWhenTheFileDiffers(t *testing.T) {
	t.Parallel()
	root, ws := laneFixture(t, "cycle-5")
	if _, drifted := LaneDrift(ws); drifted {
		t.Fatal("no record is no drift")
	}
	if err := WriteRecord(root, Record{Cycle: 5, ItemIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if _, drifted := LaneDrift(ws); drifted {
		t.Fatal("a missing file is no drift")
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["a"]}`)
	if _, drifted := LaneDrift(ws); drifted {
		t.Fatal("a file equal to the record is no drift")
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["c","b"]}`)
	d, drifted := LaneDrift(ws)
	if !drifted || !slices.Equal(d.Record, []string{"a"}) || !slices.Equal(d.File, []string{"c", "b"}) || d.Path != filepath.Join(ws, LanePinFile) {
		t.Fatalf("LaneDrift = %+v %v", d, drifted)
	}
}

func TestRecord_RoundTripAndReadFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, ok := ReadRecord(root, 3); ok {
		t.Fatal("no record yet")
	}
	want := Record{Cycle: 3, RunID: "r", ItemIDs: []string{"a"}, GoalHash: "h"}
	if err := WriteRecord(root, want); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadRecord(root, 3)
	if !ok || got.RunID != "r" || got.GoalHash != "h" || !slices.Equal(got.ItemIDs, want.ItemIDs) {
		t.Fatalf("ReadRecord = %+v %v", got, ok)
	}
	if _, err := LoadRecord(filepath.Join(root, "missing.json")); err == nil {
		t.Fatal("LoadRecord must fail on a missing file")
	}
	if err := WriteRecord(filepath.Join(root, ".evolve", "lineage", "cycle-3.json", "x"), Record{Cycle: 1}); err == nil {
		t.Fatal("WriteRecord must fail when its directory cannot be made")
	}
	if RecordsDir(root) != filepath.Join(root, ".evolve", "lineage") {
		t.Fatal("RecordsDir")
	}
	if RecordPath(root, 3) != filepath.Join(root, ".evolve", "lineage", "cycle-3.json") {
		t.Fatal("RecordPath")
	}
}

func TestReadLaneDoc_SeparatesAMissingPinFromAMalformedOne(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-2")
	if _, err := ReadLaneDoc(ws); !os.IsNotExist(err) {
		t.Fatalf("missing pin: err = %v, want not-exist", err)
	}
	if _, err := ReadLaneDoc(""); !os.IsNotExist(err) {
		t.Fatalf("empty workspace: err = %v, want not-exist", err)
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{broken`)
	if _, err := ReadLaneDoc(ws); err == nil || os.IsNotExist(err) {
		t.Fatalf("malformed pin: err = %v, want a parse error", err)
	}
}

func TestShippedIDs_ReadsTheSkipShippedBucketAndNothingElse(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-1")
	if got := ShippedIDs(ws); got != nil {
		t.Fatalf("ShippedIDs without a decision = %v", got)
	}
	writeFile(t, filepath.Join(ws, DecisionFile), `{"top_n":[{"id":"t"}],"skip_shipped":[{"task_id":" "},{"task_id":"s"}]}`)
	if got := ShippedIDs(ws); !slices.Equal(got, []string{"s"}) {
		t.Fatalf("ShippedIDs = %v, want [s]", got)
	}
}

func TestLanePin_ACycleNumberThatOverflowsReadsTheFile(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-99999999999999999999999")
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["z"]}`)
	if got := LanePin(ws); !slices.Equal(got, []string{"z"}) {
		t.Fatalf("LanePin = %v, want [z]", got)
	}
}

func TestLaneDrift_AFileOfTheSameLengthWithOtherIDsIsDrift(t *testing.T) {
	t.Parallel()
	root, ws := laneFixture(t, "cycle-5")
	if err := WriteRecord(root, Record{Cycle: 5, ItemIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["b"]}`)
	d, drifted := LaneDrift(ws)
	if !drifted || !slices.Equal(d.Record, []string{"a"}) || !slices.Equal(d.File, []string{"b"}) {
		t.Fatalf("LaneDrift = %+v %v, want drift [a] against [b]", d, drifted)
	}
}

func TestLanePin_ARecordOfAnotherRootShapeIsNotReadFromTheWorkspacePath(t *testing.T) {
	t.Parallel()
	for name, rel := range map[string]string{
		"parent is not .evolve": filepath.Join("other", "runs", "cycle-3"),
		"dir is not runs":       filepath.Join(".evolve", "other", "cycle-3"),
	} {
		root := t.TempDir()
		if err := WriteRecord(root, Record{Cycle: 3, ItemIDs: []string{"rec"}}); err != nil {
			t.Fatal(err)
		}
		ws := filepath.Join(root, rel)
		writeFile(t, filepath.Join(ws, LanePinFile), `{"todo_ids":["file"]}`)
		if got := LanePin(ws); !slices.Equal(got, []string{"file"}) {
			t.Errorf("%s: LanePin = %v, want [file]", name, got)
		}
	}
}

func TestDrift_CarriesThePinPathAndBothListsOfAForgedFile(t *testing.T) {
	t.Parallel()
	root, ws := laneFixture(t, "cycle-6")
	if err := WriteRecord(root, Record{Cycle: 6, ItemIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, LanePinPath(ws), `{"todo_ids":["forged"]}`)
	want := Drift{Path: filepath.Join(ws, LanePinFile), Record: []string{"a"}, File: []string{"forged"}}
	got, drifted := LaneDrift(ws)
	if !drifted || got.Path != want.Path || !slices.Equal(got.Record, want.Record) || !slices.Equal(got.File, want.File) {
		t.Fatalf("LaneDrift = %+v %v, want %+v", got, drifted, want)
	}
}

func TestReadLaneDoc_NamesTheFileInEachFailure(t *testing.T) {
	t.Parallel()
	_, ws := laneFixture(t, "cycle-7")
	if _, err := ReadLaneDoc(ws); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an absent file must stay not-exist, got %v", err)
	}
	writeFile(t, LanePinPath(ws), `{bad`)
	if _, err := ReadLaneDoc(ws); err == nil || !strings.Contains(err.Error(), LanePinFile+" malformed") {
		t.Fatalf("malformed error = %v", err)
	}
	if err := os.Remove(LanePinPath(ws)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(LanePinPath(ws), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLaneDoc(ws); err == nil || !strings.Contains(err.Error(), LanePinFile+" unreadable") {
		t.Fatalf("unreadable error = %v", err)
	}
}
