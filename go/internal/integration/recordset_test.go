package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func writePatched(t *testing.T, root, wave string, cycle int, sha, patch string) {
	t.Helper()
	rec := DeliverableRecord{Wave: wave, Cycle: cycle, Trailer: CycleTrailer(cycle),
		Commits: []CommitIdentity{{SHA: sha, BaseSHA: "b", PatchID: patch}}}
	if _, err := WriteRecord(WaveDir(root, wave), rec); err != nil {
		t.Fatal(err)
	}
}

func TestListRecords_ListsPoolsInNumericOrderAndReturnsEachReadFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePatched(t, root, "1000", 7, "later", "p")
	writePatched(t, root, "999", 8, "earlier", "p")
	bad := filepath.Join(WaveDir(root, "999"), "deliverable-9.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	set := ListRecords(root)

	if len(set.Listed) != 2 || set.Listed[0].Pool != 999 || set.Listed[1].Pool != 1000 {
		t.Fatalf("listed = %+v, want pool 999 then 1000", set.Listed)
	}
	if len(set.Failures) != 1 || set.Failures[0].Path != bad || set.Failures[0].Err == nil {
		t.Fatalf("failures = %+v, want the bad file %s with its error", set.Failures, bad)
	}
}

func TestListRecords_AMalformedRootIsAFailureAndNotAnEmptySet(t *testing.T) {
	t.Parallel()
	set := ListRecords("[")
	if len(set.Listed) != 0 || len(set.Failures) != 1 {
		t.Fatalf("set = %+v, want no record and one failure", set)
	}
}

func TestRecordSet_TheProjectionsKeepTheLookupsOfTheSingleHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePatched(t, root, "5", 7, "oldcommit", "p")
	writePatched(t, root, "9", 7, "newcommit", "q")
	writePatched(t, root, "3", 8, "othercycle", "p")
	set := ListRecords(root)

	if r, ok := set.ByCommit("newcommit"); !ok || r.Wave != "9" {
		t.Errorf("ByCommit(newcommit) = %+v, %v, want pool 9", r, ok)
	}
	if _, ok := set.ByCommit("nope"); ok {
		t.Error("ByCommit found an unknown commit")
	}
	if r, ok := set.ForCommit(7, "oldcommit"); !ok || r.Wave != "5" {
		t.Errorf("ForCommit(7, oldcommit) = %+v, %v, want pool 5", r, ok)
	}
	if _, ok := set.ForCommit(7, "othercycle"); ok {
		t.Error("ForCommit matched a commit of another cycle")
	}
	if r, ok := FindRecordByCommit(root, "newcommit"); !ok || r.Wave != "9" {
		t.Errorf("FindRecordByCommit = %+v, %v, want pool 9", r, ok)
	}
	if r, ok := RecordForCommit(root, 7, "oldcommit"); !ok || r.Wave != "5" {
		t.Errorf("RecordForCommit = %+v, %v, want pool 5", r, ok)
	}
}

func TestRecordSet_OwnedGivesEachMemberToItsFirstPoolAndSkipsNamedWaves(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePatched(t, root, "5", 7, "first", "same")
	writePatched(t, root, "9", 7, "second", "same")
	writePatched(t, root, "9", 8, "other", "own")
	writePatched(t, root, "12", 7, "different", "changed")
	writePatched(t, root, "console", 7, "named", "same")

	owned := ListRecords(root).Owned()

	var got []string
	for _, o := range owned {
		got = append(got, o.Record.Commits[0].SHA)
	}
	want := []string{"first", "other", "different"}
	if len(got) != len(want) {
		t.Fatalf("owned = %v, want %v: same cycle and patches keep the first owner, other patches are a new member, a named wave owns nothing", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("owned = %v, want %v", got, want)
		}
	}
}

func TestMemberKey_NamesTheCycleAndItsPatchesInAnyOrder(t *testing.T) {
	t.Parallel()
	a := DeliverableRecord{Cycle: 7, Commits: []CommitIdentity{{PatchID: "x"}, {PatchID: "y"}}}
	b := DeliverableRecord{Cycle: 7, Commits: []CommitIdentity{{PatchID: "y"}, {PatchID: "x"}}}
	c := DeliverableRecord{Cycle: 8, Commits: []CommitIdentity{{PatchID: "x"}, {PatchID: "y"}}}
	if MemberKey(a) != MemberKey(b) {
		t.Error("the order of the patches changed the member")
	}
	if MemberKey(a) == MemberKey(c) {
		t.Error("two cycles share a member")
	}
}
