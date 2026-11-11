package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMover_Release_ChargesTheItemWithTheFailureRecord(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 8, "task-r.json"), `{"id":"task-r","failure_count":1}`)
	m := New(inbox, nil)
	if res, err := m.Release(8, "cycle-failure-release", &Policy{Ceiling: 3, FailureClass: "capability", Fingerprint: "codes:floor", Reasons: []string{strings.Repeat("a", 5000)}}); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	doc := readJSON(t, filepath.Join(inbox, "task-r.json"))
	if string(doc["failure_count"]) != "2" || string(doc["last_failure_class"]) != `"capability"` || string(doc["last_failure_fingerprint"]) != `"codes:floor"` {
		t.Errorf("the charge keeps the class and the fingerprint of the failure: %v", doc)
	}
	if string(doc["retry_plan"]) != `"fresh-deep-plan"` {
		t.Errorf("retry_plan = %s, want fresh-deep-plan", doc["retry_plan"])
	}
}

func TestBumpWith_AFailureWithoutAClassClearsTheOlderRecord(t *testing.T) {
	inbox := newInbox(t)
	item := filepath.Join(inbox, "a.json")
	writeItem(t, item, `{"id":"a","failure_count":1,"last_failure_class":"capability","last_failure_fingerprint":"codes:old","last_failure_rung":"R2"}`)
	if n, err := bumpWith(item, failureRecord{reason: "r"}, nil); err != nil || n != 2 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	doc := readJSON(t, item)
	for _, k := range []string{"last_failure_class", "last_failure_fingerprint", "last_failure_rung"} {
		if _, kept := doc[k]; kept {
			t.Errorf("the record describes the latest failure only, but %s stayed: %v", k, doc)
		}
	}
}

func TestMover_ReleaseFromQuarantine_ClearsTheFailureRecord(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","failure_count":2,"last_failure_reason":"r","last_failure_class":"spec","last_failure_fingerprint":"text:abc","last_failure_rung":"R1"}`)
	m := New(inbox, nil)
	res, err := m.ReleaseFromQuarantine("q")
	if err != nil {
		t.Fatalf("ReleaseFromQuarantine: %v", err)
	}
	doc := readJSON(t, res.DestPath)
	for _, k := range []string{"last_failure_reason", "last_failure_class", "last_failure_fingerprint", "last_failure_rung"} {
		if _, kept := doc[k]; kept {
			t.Errorf("a released item starts with a clean failure record, but %s stayed: %v", k, doc)
		}
	}
}

func TestBumpWith_StampsTheRungOfTheFailedTry(t *testing.T) {
	inbox := newInbox(t)
	item := filepath.Join(inbox, "a.json")
	writeItem(t, item, `{"id":"a"}`)
	if _, err := bumpWith(item, failureRecord{reason: "r", rung: "R1"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(readJSON(t, item)["last_failure_rung"]); got != `"R1"` {
		t.Errorf("last_failure_rung = %s", got)
	}
}

func TestMover_ReadLastFailureRung_ReadsPendingAndClaimedOnly(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"a","last_failure_rung":"R2"}`)
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","last_failure_rung":"R1"}`)
	m := New(inbox, nil)
	for id, want := range map[string]string{"a": "R2", "q": "", "ghost": ""} {
		if got := m.ReadLastFailureRung(id); got != want {
			t.Errorf("ReadLastFailureRung(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestMover_Release_OnCeilingConsole(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 9, "task-con.json"), `{"id":"task-con"}`)
	m := New(inbox, nil)
	res, err := m.Release(9, "cycle-failure-release", &Policy{Ceiling: 1, OnCeiling: "console"})
	if err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	doc := readJSON(t, filepath.Join(inbox, "task-con.json"))
	if string(doc["route"]) != `"console-manual"` {
		t.Errorf("route = %s, want console-manual", doc["route"])
	}
}
