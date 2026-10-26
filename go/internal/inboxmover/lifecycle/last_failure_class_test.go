package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestReadLastFailureClass_ReadsTheFailureRecordOfAPendingOrClaimedItem(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"a","failure_count":1,"last_failure_class":"capability"}`)
	writeItem(t, procPath(inbox, 3, "b.json"), `{"id":"b","failure_count":2,"last_failure_class":"spec"}`)
	writeItem(t, filepath.Join(inbox, "c.json"), `{"id":"c","failure_count":1}`)
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","last_failure_class":"capability"}`)
	m := New(inbox, nil)
	for id, want := range map[string]string{"a": "capability", "b": "spec", "c": "", "q": "", "ghost": ""} {
		if got := m.ReadLastFailureClass(id); got != want {
			t.Errorf("ReadLastFailureClass(%s) = %q, want %q", id, got, want)
		}
	}
}
