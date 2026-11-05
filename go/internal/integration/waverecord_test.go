package integration

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validWaveRecord() WaveRecord {
	return WaveRecord{
		Pool:       41,
		Epoch:      3,
		ShadowBase: strings.Repeat("a", 40),
		Batches: []IntegrationBatch{
			{Index: 1, Base: strings.Repeat("a", 40), Branch: "integration/p41-b1", Tip: strings.Repeat("b", 40), Members: []int{42, 43}},
			{Index: 2, Base: strings.Repeat("b", 40), Branch: "integration/p41-b2", Tip: strings.Repeat("c", 40), PR: 7, Members: []int{44}},
		},
		Members: map[int]MemberState{
			42: {Status: MemberComposed, Landed: true, PatchIDs: []string{"p42"}},
			43: {Status: MemberConflicted, PatchIDs: []string{"p43"}},
			44: {Status: MemberPending, PatchIDs: []string{"p44"}},
		},
		Decisions: []FlushRecord{{Trigger: TriggerWaveEnd, At: time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC), Flush: []int{42, 43}, Hold: []int{44}}},
		Failures:  []DriverFailure{{Step: "compose", Error: "boom", At: time.Date(2026, 11, 5, 9, 1, 0, 0, time.UTC)}},
		Ended:     true,
		LastCycle: 44,
	}
}

func TestPoolBranch_NamesThePoolAndTheBatchApartFromTheConsoleTrainBranch(t *testing.T) {
	t.Parallel()
	if got := PoolBranch(41, 2); got != "integration/p41-b2" {
		t.Errorf("PoolBranch(41, 2) = %q", got)
	}
	if PoolBranch(41, 2) == batchBranch("41", 2) {
		t.Error("the pool branch and the console train branch share a name")
	}
}

func TestWaveRecordPath_SitsBesideTheDeliverableDirectoryOfThePool(t *testing.T) {
	t.Parallel()
	want := filepath.Join("/r", ".evolve", "waves", "integration-41.json")
	if got := WaveRecordPath("/r", 41); got != want {
		t.Errorf("WaveRecordPath = %q, want %q", got, want)
	}
}

func TestWaveRecord_ARecordRoundTripsThroughTheAtomicWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := validWaveRecord()
	path, err := WriteWaveRecord(root, want)
	if err != nil {
		t.Fatal(err)
	}
	if path != WaveRecordPath(root, 41) {
		t.Fatalf("path = %q", path)
	}
	got, err := ReadWaveRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	want.Schema = WaveRecordSchema
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if len(left) != 0 {
		t.Errorf("staging files left behind: %v", left)
	}
}

func TestWaveRecord_ValidateNamesEachBrokenField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*WaveRecord)
		want   string
	}{
		{"no pool", func(r *WaveRecord) { r.Pool = 0 }, "pool 0"},
		{"no epoch", func(r *WaveRecord) { r.Epoch = 0 }, "no epoch"},
		{"short shadow base", func(r *WaveRecord) { r.ShadowBase = "abc" }, "shadow base"},
		{"negative last cycle", func(r *WaveRecord) { r.LastCycle = -1 }, "last cycle -1"},
		{"batch index zero", func(r *WaveRecord) { r.Batches[0].Index = 0 }, "batch index 0"},
		{"batch index repeats", func(r *WaveRecord) { r.Batches[1].Index = 1 }, "batch index 1"},
		{"branch of another pool", func(r *WaveRecord) { r.Batches[0].Branch = "integration/p42-b1" }, "integration/p42-b1"},
		{"console train branch", func(r *WaveRecord) { r.Batches[0].Branch = "integration/w41-b1" }, "integration/w41-b1"},
		{"short batch base", func(r *WaveRecord) { r.Batches[0].Base = "abc" }, "batch 1 base"},
		{"short batch tip", func(r *WaveRecord) { r.Batches[0].Tip = "abc" }, "batch 1 tip"},
		{"negative pr", func(r *WaveRecord) { r.Batches[1].PR = -1 }, "pr -1"},
		{"member below one", func(r *WaveRecord) { r.Batches[0].Members = []int{0} }, "member 0"},
		{"member repeated", func(r *WaveRecord) { r.Batches[0].Members = []int{42, 42} }, "repeats member 42"},
		{"batch member unknown", func(r *WaveRecord) { r.Batches[0].Members = []int{99} }, "member 99"},
		{"member key below one", func(r *WaveRecord) { r.Members[0] = MemberState{Status: MemberPending} }, "member 0"},
		{"unknown status", func(r *WaveRecord) { r.Members[42] = MemberState{Status: "lost"} }, `status "lost"`},
		{"empty status", func(r *WaveRecord) { r.Members[42] = MemberState{} }, `status ""`},
		{"unknown trigger", func(r *WaveRecord) { r.Decisions[0].Trigger = "x" }, `trigger "x"`},
		{"decision flush member unknown", func(r *WaveRecord) { r.Decisions[0].Flush = []int{99} }, "member 99"},
		{"decision member unknown", func(r *WaveRecord) { r.Decisions[0].Hold = []int{99} }, "member 99"},
		{"failure without a step", func(r *WaveRecord) { r.Failures[0].Step = "" }, "no step"},
	} {
		r := validWaveRecord()
		r.Batches = append([]IntegrationBatch(nil), r.Batches...)
		r.Decisions = append([]FlushRecord(nil), r.Decisions...)
		r.Failures = append([]DriverFailure(nil), r.Failures...)
		r.Members = map[int]MemberState{42: r.Members[42], 43: r.Members[43], 44: r.Members[44]}
		tc.mutate(&r)
		err := r.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
	if err := validWaveRecord().Validate(); err != nil {
		t.Errorf("a valid record failed: %v", err)
	}
}

func TestWaveRecord_ANoneTriggerIsAValidDecision(t *testing.T) {
	t.Parallel()
	r := validWaveRecord()
	r.Decisions = []FlushRecord{{Trigger: TriggerNone}}
	if err := r.Validate(); err != nil {
		t.Errorf("a decision with no trigger is the hold-all decision: %v", err)
	}
}

func TestWriteWaveRecord_RefusesAnInvalidRecordAndWritesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	r := validWaveRecord()
	r.Epoch = 0
	if _, err := WriteWaveRecord(root, r); err == nil {
		t.Fatal("an invalid record was written")
	}
	if _, err := os.Stat(WaveRecordPath(root, 41)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused write left a file: %v", err)
	}
}

func TestReadWaveRecord_RefusesAWrongSchemaABadFileAndAFileOfAnotherPool(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "waves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := ReadWaveRecord(filepath.Join(dir, "absent.json")); err == nil {
		t.Error("an absent file read")
	}
	if _, err := ReadWaveRecord(write("integration-1.json", "{not json")); err == nil {
		t.Error("a malformed file read")
	}
	old := write("integration-current.json", `{"schema":"evolve.integration-record/1","wave":"current"}`)
	if _, err := ReadWaveRecord(old); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("an old schema record read: %v", err)
	}
	invalid := write("integration-2.json", `{"schema":"`+WaveRecordSchema+`","pool":2}`)
	if _, err := ReadWaveRecord(invalid); err == nil || !strings.Contains(err.Error(), "no epoch") {
		t.Errorf("an invalid record read: %v", err)
	}
	good, err := WriteWaveRecord(root, validWaveRecord())
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(dir, "integration-50.json")
	if err := os.Rename(good, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWaveRecord(moved); err == nil || !strings.Contains(err.Error(), "pool 41") {
		t.Errorf("a record in the file of another pool read: %v", err)
	}
}

func TestWriteWaveRecord_AFailedPublishIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "waves"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(WaveRecordPath(root, 41), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteWaveRecord(root, validWaveRecord()); err == nil {
		t.Fatal("a publish over a directory succeeded")
	}
}

func TestMemberStatus_EachStatusKeepsItsWordThroughTheWrite(t *testing.T) {
	t.Parallel()
	statuses := map[MemberStatus]string{
		MemberPending:             "pending",
		MemberComposed:            "composed",
		MemberConflicted:          "conflicted",
		MemberConflictedBaseAhead: "conflicted_base_ahead",
		MemberMerged:              "merged",
		MemberEjected:             "ejected",
		MemberFailed:              "failed",
	}
	for status, word := range statuses {
		if string(status) != word {
			t.Errorf("status %q, want the word %q", status, word)
		}
		root := t.TempDir()
		rec := validWaveRecord()
		rec.Members[42] = MemberState{Status: status, PatchIDs: []string{"p42"}}
		path, err := WriteWaveRecord(root, rec)
		if err != nil {
			t.Fatalf("write %q: %v", status, err)
		}
		got, err := ReadWaveRecord(path)
		if err != nil {
			t.Fatalf("read %q: %v", status, err)
		}
		if got.Members[42].Status != status {
			t.Errorf("status after the write = %q, want %q", got.Members[42].Status, status)
		}
	}
}
