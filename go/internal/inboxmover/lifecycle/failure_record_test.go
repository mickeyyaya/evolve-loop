package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestMover_Release_ChargesTheItemWithTheFailureRecord(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 8, "task-r.json"), `{"id":"task-r","failure_count":1}`)
	m := New(inbox, nil)
	if res, err := m.Release(8, "cycle-failure-release", &Policy{Ceiling: 3, FailureClass: "capability", Fingerprint: "codes:floor"}); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	doc := readJSON(t, filepath.Join(inbox, "task-r.json"))
	if string(doc["failure_count"]) != "2" || string(doc["last_failure_class"]) != `"capability"` || string(doc["last_failure_fingerprint"]) != `"codes:floor"` {
		t.Errorf("the charge keeps the class and the fingerprint of the failure: %v", doc)
	}
}

func TestBumpWith_AFailureWithoutAClassClearsTheOlderRecord(t *testing.T) {
	inbox := newInbox(t)
	item := filepath.Join(inbox, "a.json")
	writeItem(t, item, `{"id":"a","failure_count":1,"last_failure_class":"capability","last_failure_fingerprint":"codes:old"}`)
	if n, err := bumpWith(item, failureRecord{reason: "r"}, nil); err != nil || n != 2 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	doc := readJSON(t, item)
	for _, k := range []string{"last_failure_class", "last_failure_fingerprint"} {
		if _, kept := doc[k]; kept {
			t.Errorf("the record describes the latest failure only, but %s stayed: %v", k, doc)
		}
	}
}

func TestMover_ReleaseFromQuarantine_ClearsTheFailureRecord(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","failure_count":2,"last_failure_reason":"r","last_failure_class":"spec","last_failure_fingerprint":"text:abc"}`)
	m := New(inbox, nil)
	res, err := m.ReleaseFromQuarantine("q")
	if err != nil {
		t.Fatalf("ReleaseFromQuarantine: %v", err)
	}
	doc := readJSON(t, res.DestPath)
	for _, k := range []string{"last_failure_reason", "last_failure_class", "last_failure_fingerprint"} {
		if _, kept := doc[k]; kept {
			t.Errorf("a released item starts with a clean failure record, but %s stayed: %v", k, doc)
		}
	}
}
