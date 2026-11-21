package dispatchevents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	lineDispatchWriter = `{"event_type":"verify-single-flight-unavailable","timestamp":"2026-11-21T08:55:57Z","source_phase":"acs-suite","severity":"WARN","details":"the host verification lock was not taken","remediation_hint":"a wait-bound miss means another lane held the lock","cycle":2582}`
	lineWatchdogWriter = `{"details":"no output for 600s","event_type":"stall-detected","remediation_hint":"Check agent turn count","severity":"ERROR","source_phase":"phase-watchdog","timestamp":"2026-11-21T09:00:00Z"}`
	lineSubagentWriter = `{"event_type":"tdd-test-conflict","timestamp":"2026-11-21T09:10:00Z","source_phase":"subagent-run","severity":"WARN","details":"two tests disagree","remediation_hint":""}`
)

func writeEventsFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, EventsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadEvents_ReadsTheLinesOfEachWriter(t *testing.T) {
	t.Parallel()
	path := writeEventsFile(t, lineDispatchWriter+"\n\n"+lineWatchdogWriter+"\n"+lineSubagentWriter+"\n")
	got, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	want := []struct {
		kind EventType
		sev  Severity
		time string
		cyc  int
	}{
		{EventVerifySingleFlightUnavailable, SeverityWarn, "2026-11-21T08:55:57Z", 2582},
		{EventStallDetected, SeverityError, "2026-11-21T09:00:00Z", 0},
		{"tdd-test-conflict", SeverityWarn, "2026-11-21T09:10:00Z", 0},
	}
	if len(got) != len(want) {
		t.Fatalf("ReadEvents returned %d events, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].EventType != w.kind || got[i].Severity != w.sev || got[i].Timestamp != w.time || got[i].Cycle != w.cyc {
			t.Errorf("event %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestReadEvents_AnAbsentFileIsNoEvents(t *testing.T) {
	t.Parallel()
	got, err := ReadEvents(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("ReadEvents(absent) = %v, %v; want no events and no error", got, err)
	}
}

func TestReadEvents_ACorruptLineNamesItsNumber(t *testing.T) {
	t.Parallel()
	atLine2 := writeEventsFile(t, lineDispatchWriter+"\n{not json\n"+lineWatchdogWriter+"\n")
	atLine3 := writeEventsFile(t, lineDispatchWriter+"\n"+lineWatchdogWriter+"\n{not json\n")
	_, err2 := ReadEvents(atLine2)
	_, err3 := ReadEvents(atLine3)
	if err2 == nil || err3 == nil {
		t.Fatalf("ReadEvents(corrupt) errors = %v, %v; want both to fail", err2, err3)
	}
	msg2, msg3 := strings.ReplaceAll(err2.Error(), atLine2, ""), strings.ReplaceAll(err3.Error(), atLine3, "")
	if msg2 == msg3 {
		t.Fatalf("a corrupt line 2 and a corrupt line 3 must report different positions, both gave %q", msg2)
	}
}

func TestReadEvents_AnUnreadablePathIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, EventsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEvents(dir); err == nil {
		t.Fatal("ReadEvents must fail when the events path is a directory")
	}
}

func TestReadEvents_ALineOverTheScannerLimitIsAnError(t *testing.T) {
	t.Parallel()
	long := `{"event_type":"x","details":"` + strings.Repeat("a", maxEventLineBytes) + `"}`
	if _, err := ReadEvents(writeEventsFile(t, long+"\n")); err == nil {
		t.Fatal("ReadEvents must refuse a line over the limit")
	}
}

func TestReadEvents_APathUnderAFileIsAnOpenError(t *testing.T) {
	t.Parallel()
	dir := writeEventsFile(t, lineDispatchWriter+"\n")
	if _, err := ReadEvents(filepath.Join(dir, EventsFile)); err == nil {
		t.Fatal("ReadEvents must fail when the workspace path is a file")
	}
}

func TestReadEvents_ReadsWhatTheWriterWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := NewWriter(dir).Emit(Event{EventType: EventVerifyFailed, Severity: SeverityError, Cycle: 4, Details: "round trip"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadEvents(dir)
	if err != nil || len(got) != 1 || got[0].EventType != EventVerifyFailed || got[0].Cycle != 4 || got[0].Details != "round trip" {
		t.Fatalf("ReadEvents after Emit = %+v, %v", got, err)
	}
}
