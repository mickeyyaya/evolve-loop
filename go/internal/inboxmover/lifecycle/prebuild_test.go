package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prebuildPolicy(id string) *Policy {
	return &Policy{Ceiling: 2, SystemLevel: true, Committed: map[string]bool{id: true}, Prebuild: true, PrebuildCeiling: 2}
}

func TestMover_Release_APreBuildFailureAtTheCeilingParksTheItemAndNamesItInAWarning(t *testing.T) {
	t.Parallel()
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 9, "p.json"), `{"id":"poison","prebuild_failure_cycles":[4]}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	res, err := m.Release(9, "", prebuildPolicy("poison"))
	parked := filepath.Join(inbox, "quarantine", "p.json")
	if err != nil || len(res.Paths) != 1 || res.Paths[0] != parked {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := string(readJSON(t, parked)["prebuild_failure_cycles"]); got != "[4,9]" {
		t.Errorf("prebuild_failure_cycles = %s, want [4,9]", got)
	}
	var named bool
	for _, e := range rc.events {
		named = named || (e.Code == CodeItemPrebuildBreaker && e.Fields["task_id"] == "poison" && e.Fields["cycles"] == "4,9" && e.Fields["ceiling"] == "2" && e.Fields["path"] == parked)
	}
	if !named {
		t.Errorf("events = %+v, want INBOX_ITEM_PREBUILD_BREAKER naming poison", rc.events)
	}
	if _, err := m.Claim("poison", "10"); err == nil {
		t.Error("a parked item must refuse the next claim")
	}
}

func TestMover_Release_APreBuildFailureBelowTheCeilingReleasesAndRecordsTheCycle(t *testing.T) {
	t.Parallel()
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 4, "p.json"), `{"id":"poison"}`)
	writeItem(t, procPath(inbox, 4, "q.json"), `{"id":"quiet"}`)
	q := prebuildPolicy("poison")
	q.PrebuildCeiling = 0
	if _, err := New(inbox, nil).Release(4, "", prebuildPolicy("poison")); err != nil {
		t.Fatal(err)
	}
	if got := string(readJSON(t, filepath.Join(inbox, "p.json"))["prebuild_failure_cycles"]); got != "[4]" {
		t.Errorf("prebuild_failure_cycles = %s, want [4]", got)
	}
	if _, had := readJSON(t, filepath.Join(inbox, "q.json"))["prebuild_failure_cycles"]; had {
		t.Error("an uncommitted id is not charged")
	}
	writeItem(t, procPath(inbox, 5, "r.json"), `{"id":"poison2","prebuild_failure_cycles":[1,2,3]}`)
	q.Committed = map[string]bool{"poison2": true}
	if _, err := New(inbox, nil).Release(5, "", q); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "r.json")); err != nil {
		t.Errorf("ceiling 0 turns the breaker off: %v", err)
	}
}

func TestMover_Release_AFailureThatReachedBuildResetsTheConsecutiveCount(t *testing.T) {
	t.Parallel()
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 6, "p.json"), `{"id":"poison","prebuild_failure_cycles":[5]}`)
	writeItem(t, procPath(inbox, 6, "c.json"), `{"id":"clean"}`)
	q := prebuildPolicy("poison")
	q.Committed["clean"], q.Prebuild = true, false
	if _, err := New(inbox, nil).Release(6, "", q); err != nil {
		t.Fatal(err)
	}
	if _, had := readJSON(t, filepath.Join(inbox, "p.json"))["prebuild_failure_cycles"]; had {
		t.Error("a failure past build must clear the consecutive pre-build record")
	}
	if body, _ := os.ReadFile(filepath.Join(inbox, "c.json")); string(body) != `{"id":"clean"}` {
		t.Errorf("an item with no record is not rewritten: %s", body)
	}
}

func TestRecordPrebuild_TheSameCycleCountsOnce(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "p.json")
	writeItem(t, path, `{"id":"poison","prebuild_failure_cycles":[7]}`)
	cycles, err := recordPrebuild(path, 7, true)
	if err != nil || len(cycles) != 1 {
		t.Fatalf("cycles = %v, err = %v", cycles, err)
	}
}

func TestMover_Release_AnUnreadablePreBuildRecordSkipsTheBreakerAndSaysSo(t *testing.T) {
	t.Parallel()
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 3, "p.json"), `not json`)
	rc := newRecordingCenter()
	q := prebuildPolicy("unknown")
	q.Committed = nil
	if _, err := New(inbox, nil, WithSignals(rc.accessor())).Release(3, "", q); err != nil {
		t.Fatal(err)
	}
	if len(rc.events) == 0 || rc.events[0].Code != CodeItemRewriteFailed || rc.events[0].Fields["step"] != "prebuild_record" {
		t.Errorf("events = %+v", rc.events)
	}
}

func TestMover_Release_APreBuildParkThatCannotMoveFallsOpenAndNamesTheBreakerStep(t *testing.T) {
	t.Parallel()
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "quarantine"), `{}`)
	writeItem(t, procPath(inbox, 9, "p.json"), `{"id":"poison","prebuild_failure_cycles":[4]}`)
	var stderr strings.Builder
	if _, err := New(inbox, nil, WithStderr(&stderr)).Release(9, "", prebuildPolicy("poison")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "quarantine failed for 'poison' (failed before build in cycles 4,9 >= ceiling 2)") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "prebuild breaker:") {
		t.Error("no breaker warning when the park did not happen")
	}
}
