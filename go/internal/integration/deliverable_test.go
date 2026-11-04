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

func fullRecord() DeliverableRecord {
	return DeliverableRecord{
		Wave:    "w7",
		Cycle:   42,
		Trailer: CycleTrailer(42),
		Commits: []CommitIdentity{
			{SHA: "a1", BaseSHA: "b0", PatchID: "p1"},
			{SHA: "a2", BaseSHA: "a1", PatchID: "p2"},
		},
		Audit:       AuditBinding{TreeSHA: "tree1", ArtifactSHA: "art1"},
		Scope:       []string{"go/internal/x"},
		Deps:        []string{"cycle-41"},
		InboxRank:   3,
		Risk:        "high",
		DeliveredAt: time.Date(2026, 11, 3, 7, 0, 0, 0, time.UTC),
	}
}

func TestCycleTrailer(t *testing.T) {
	if got := CycleTrailer(42); got != "Evolve-Cycle: 42" {
		t.Fatalf("CycleTrailer(42) = %q", got)
	}
}

func TestRecordRoundTripKeepsEveryD2Field(t *testing.T) {
	dir := t.TempDir()
	want := fullRecord()
	path, err := WriteRecord(dir, want)
	if err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}
	if path != RecordPath(dir, 42) || filepath.Base(path) != "deliverable-42.json" {
		t.Fatalf("path = %q", path)
	}
	got, err := ReadRecord(path)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	want.Schema = RecordSchema
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip\n got %+v\nwant %+v", got, want)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir holds %d entries, want only the record", len(entries))
	}
}

func TestWriteRecordRejectsAnInvalidRecordAndWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	r := fullRecord()
	r.Commits = nil
	if _, err := WriteRecord(dir, r); err == nil {
		t.Fatal("want an error for a record with no commits")
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("an invalid record created the directory")
	}
}

func TestWriteRecordFailsWhenTheDirectoryCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRecord(filepath.Join(blocker, "sub"), fullRecord()); err == nil {
		t.Fatal("want an error when the directory is under a file")
	}
}

func TestWriteRecordFailsWhenTheTargetIsADirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(RecordPath(dir, 42), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRecord(dir, fullRecord()); err == nil {
		t.Fatal("want an error when the record path is a directory")
	}
}

func TestReadRecordErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadRecord(filepath.Join(dir, "none.json")); err == nil {
		t.Fatal("want an error for a missing file")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecord(bad); err == nil {
		t.Fatal("want an error for bad json")
	}
	other := filepath.Join(dir, "other.json")
	if err := os.WriteFile(other, []byte(`{"schema":"x/9"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecord(other); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("want a schema error, got %v", err)
	}
}

func TestValidateNamesTheMissingField(t *testing.T) {
	cases := map[string]func(*DeliverableRecord){
		"wave":     func(r *DeliverableRecord) { r.Wave = "" },
		"cycle":    func(r *DeliverableRecord) { r.Cycle = 0 },
		"trailer":  func(r *DeliverableRecord) { r.Trailer = "" },
		"commits":  func(r *DeliverableRecord) { r.Commits = nil },
		"patch_id": func(r *DeliverableRecord) { r.Commits[0].PatchID = "" },
		"base_sha": func(r *DeliverableRecord) { r.Commits[1].BaseSHA = "" },
		"sha":      func(r *DeliverableRecord) { r.Commits[0].SHA = "" },
	}
	for field, mutate := range cases {
		r := fullRecord()
		r.Commits = append([]CommitIdentity(nil), r.Commits...)
		mutate(&r)
		err := r.Validate()
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s: err = %v", field, err)
		}
	}
	if err := fullRecord().Validate(); err != nil {
		t.Errorf("a full record is invalid: %v", err)
	}
}

func TestDeliverableMapsTheOrderingFacts(t *testing.T) {
	r := fullRecord()
	d := r.Deliverable()
	if d.ID != "cycle-42" || d.InboxRank != 3 || !d.Risky || !d.DeliveredAt.Equal(r.DeliveredAt) || !reflect.DeepEqual(d.Deps, []string{"cycle-41"}) {
		t.Fatalf("deliverable = %+v", d)
	}
	r.Risk = "low"
	if r.Deliverable().Risky {
		t.Fatal("a low risk record is risky")
	}
	r.Deps[0] = "mutated"
	if d.Deps[0] != "cycle-41" {
		t.Fatal("Deliverable shares the Deps slice with the record")
	}
	other := DeliverableRecord{Cycle: 99, InboxRank: 11, Risk: "low"}.Deliverable()
	if other.ID != "cycle-99" || other.InboxRank != 11 || other.Risky || other.Deps != nil {
		t.Fatalf("second record deliverable = %+v", other)
	}
}

func TestWaveDirIsWhereFindRecordLooks(t *testing.T) {
	root := t.TempDir()
	if _, err := WriteRecord(WaveDir(root, "w9"), fullRecord()); err != nil {
		t.Fatal(err)
	}
	if got := WaveDir(root, "w9"); got != filepath.Join(root, ".evolve", "waves", "integration-w9") {
		t.Fatalf("WaveDir = %s", got)
	}
	if r, ok := FindRecord(root, 42); !ok || r.Wave != "w7" {
		t.Fatalf("FindRecord did not see a record written under WaveDir: %+v %v", r, ok)
	}
}

func TestReadRecordRefusesARecordThatDoesNotValidate(t *testing.T) {
	dir := t.TempDir()
	path := RecordPath(dir, 42)
	raw := `{"schema":"` + RecordSchema + `","wave":"w7","cycle":42,"trailer":"Evolve-Cycle: 42","commits":[{"sha":"a1","base_sha":"--output=x","patch_id":""}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecord(path); err == nil || !strings.Contains(err.Error(), "patch_id") {
		t.Fatalf("err = %v, want a refusal that names patch_id", err)
	}
}

func TestHasCycleTrailer(t *testing.T) {
	cases := []struct {
		name, msg string
		cycle     int
		want      bool
	}{
		{"trailer line", "subject\n\nbody\n\nEvolve-Cycle: 42\nOther: x\n", 42, true},
		{"subject form", "evolve-cycle 42: goal=abc\n\nbody", 42, true},
		{"solution form", "solution(t1): evolve-cycle 42 (also t2)", 42, true},
		{"other cycle trailer", "x\n\nEvolve-Cycle: 421\n", 42, false},
		{"other cycle subject", "evolve-cycle 421: goal=abc", 42, false},
		{"body mention only", "subject\n\nthe evolve-cycle 42 change was reverted", 42, false},
		{"empty", "", 42, false},
		{"zero cycle", "evolve-cycle 0: x", 0, false},
	}
	for _, c := range cases {
		if got := HasCycleTrailer(c.msg, c.cycle); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestFindLandedSurvivesARewriteThatChangesTheSHA(t *testing.T) {
	r := fullRecord()
	landed := []LandedCommit{
		{SHA: "z1", Message: "evolve-cycle 40: goal=x", PatchID: "other"},
		{SHA: "z2", Message: "evolve-cycle 42: goal=g\n\nEvolve-Cycle: 42", PatchID: "p2"},
		{SHA: "z3", Message: "evolve-cycle 42: goal=g", PatchID: "p1"},
	}
	if !FindLanded(landed, r) {
		t.Fatal("both patches landed under the trailer with new SHAs, but the cycle is not found")
	}
}

func TestFindLandedNeedsEveryCommitOfTheCycle(t *testing.T) {
	r := fullRecord()
	partial := []LandedCommit{{SHA: "z3", Message: "evolve-cycle 42: goal=g", PatchID: "p1"}}
	if FindLanded(partial, r) {
		t.Fatal("a cycle with one of its two commits on the ref counts as landed")
	}
	untrailed := append(partial, LandedCommit{SHA: "z4", Message: "fix: other work", PatchID: "p2"})
	if FindLanded(untrailed, r) {
		t.Fatal("the second patch matched without the cycle trailer")
	}
}

func TestFindLandedNeedsBothTheTrailerAndThePatchID(t *testing.T) {
	r := fullRecord()
	r.Commits = r.Commits[:1]
	if FindLanded([]LandedCommit{{SHA: "z", Message: "evolve-cycle 42: g", PatchID: "nope"}}, r) {
		t.Fatal("matched on the trailer alone")
	}
	if FindLanded([]LandedCommit{{SHA: "z", Message: "evolve-cycle 9: g", PatchID: "p1"}}, r) {
		t.Fatal("matched on the patch-id alone")
	}
	if FindLanded(nil, r) {
		t.Fatal("matched in an empty history")
	}
	if !FindLanded([]LandedCommit{{SHA: "z", Message: "evolve-cycle 42: g", PatchID: "p1"}}, r) {
		t.Fatal("the trailer and the patch-id together did not match")
	}
	r.Commits = nil
	if FindLanded([]LandedCommit{{SHA: "z", Message: "evolve-cycle 42: g", PatchID: "p1"}}, r) {
		t.Fatal("a record with no commits matched")
	}
}

func TestWriteRecordReportsAnEncodeFailure(t *testing.T) {
	prev := marshalRecord
	marshalRecord = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalRecord = prev })
	if _, err := WriteRecord(t.TempDir(), fullRecord()); err == nil || !strings.Contains(err.Error(), "encode") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteRecordReportsAStagingFailure(t *testing.T) {
	prev := createRecordTemp
	t.Cleanup(func() { createRecordTemp = prev })
	createRecordTemp = func(string, string) (*os.File, error) { return nil, errors.New("no temp") }
	if _, err := WriteRecord(t.TempDir(), fullRecord()); err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("err = %v", err)
	}
	readOnly := filepath.Join(t.TempDir(), "read-only")
	if err := os.WriteFile(readOnly, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	createRecordTemp = func(string, string) (*os.File, error) { return os.Open(readOnly) }
	if _, err := WriteRecord(t.TempDir(), fullRecord()); err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("err = %v, want the failed write of the staged record", err)
	}
	if _, err := os.Stat(readOnly); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failed staged file was not removed: %v", err)
	}
}

func TestWriteRecordNeverWritesThroughALinkAtAStagingName(t *testing.T) {
	dir, victim := t.TempDir(), filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("important\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, RecordPath(dir, 42)+".tmp"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRecord(dir, fullRecord()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "important\n" {
		t.Errorf("the record write followed a link to %s: %q", victim, got)
	}
}

func TestHoldsCommit_VouchesOnlyForACommitOfTheRecord(t *testing.T) {
	t.Parallel()
	r := DeliverableRecord{Commits: []CommitIdentity{{SHA: "aaa"}, {SHA: "bbb"}}}
	if !r.HoldsCommit("bbb") || r.HoldsCommit("ccc") || (DeliverableRecord{}).HoldsCommit("") {
		t.Error("HoldsCommit vouches for a commit that the record does not hold")
	}
}

func TestReadRecordRefusesARecordOfTheContextWidthSchema(t *testing.T) {
	dir := t.TempDir()
	path := RecordPath(dir, 42)
	raw := `{"schema":"evolve.deliverable/1","wave":"w7","cycle":42,"trailer":"Evolve-Cycle: 42","commits":[{"sha":"a1","base_sha":"b0","patch_id":"p1"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecord(path); err == nil || !strings.Contains(err.Error(), "evolve.deliverable/1") || !strings.Contains(err.Error(), RecordSchema) {
		t.Fatalf("err = %v, want a refusal that names both schemas", err)
	}
}

func TestFindRecord_AnOldSchemaRecordReadsAsNoRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(WaveDir(root, "w7"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"schema":"evolve.deliverable/1","wave":"w7","cycle":42,"trailer":"Evolve-Cycle: 42","commits":[{"sha":"a1","base_sha":"b0","patch_id":"p1"}]}`
	if err := os.WriteFile(RecordPath(WaveDir(root, "w7"), 42), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindRecord(root, 42); ok {
		t.Fatal("FindRecord returned a record of the old schema")
	}
}
