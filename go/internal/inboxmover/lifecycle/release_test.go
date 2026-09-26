package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readJSON(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func TestMover_Release_DoubleMove_EmitsReleaseDoubleMove(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "dup.json"), `{"id":"dup-original"}`)
	writeItem(t, procPath(inbox, 5, "dup.json"), `{"id":"dup-claimed"}`)
	writeItem(t, procPath(inbox, 5, "ok.json"), `{"id":"ok"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	res, err := m.Release(5, "", nil)
	if err != nil || res.Recovered != 1 || len(res.Paths) != 1 || res.Paths[0] != filepath.Join(inbox, "ok.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if body, _ := os.ReadFile(filepath.Join(inbox, "dup.json")); !strings.Contains(string(body), "dup-original") {
		t.Errorf("the root copy was clobbered: %s", body)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeReleaseDoubleMove || rc.events[0].Cycle != 5 || rc.events[0].Fields["base"] != "dup.json" || rc.events[0].Fields["task_id"] != "dup-claimed" {
		t.Errorf("events = %+v", rc.events)
	}
}

func TestMover_Release_QuarantineAtCeiling_ReplaysG2(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 5, "task-a.json"), `{"id":"task-a","title":"A","failure_count":1,"continuation":{"snapshot_sha":"abc123","cycle":4},"zeta":true}`)
	writeItem(t, procPath(inbox, 5, "task-b.json"), `{"id":"task-b","title":"B"}`)
	var stderr strings.Builder
	rec := &recordingAppender{}
	m := New(inbox, rec, WithStderr(&stderr), WithNow(func() time.Time { return fixedClock }))
	res, err := m.Release(5, "", &Policy{Ceiling: 2, Committed: map[string]bool{"task-a": true}})
	if err != nil || res.Recovered != 2 || len(res.Paths) != 2 || res.Paths[0] != filepath.Join(inbox, "quarantine", "task-a.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	item, _ := os.ReadFile(filepath.Join(inbox, "quarantine", "task-a.json"))
	if string(item) != `{"failure_count":2,"id":"task-a","last_failure_reason":"cycle-release","title":"A","zeta":true}` {
		t.Errorf("quarantined bytes: %s", item)
	}
	if b := readJSON(t, filepath.Join(inbox, "task-b.json")); string(b["failure_count"]) != "" {
		t.Errorf("an uncommitted id releases un-bumped: %s", b["failure_count"])
	}
	want := "[inbox-mover] promoted: task-a.json → quarantine/\n[inbox-mover] quarantined: task-a.json (task-level failure #2 >= ceiling 2) ← processing/cycle-5/\n[inbox-mover] released: task-b.json ← processing/cycle-5/\n[inbox-mover] release-cycle: 2 file(s) released from cycle-5\n"
	if stderr.String() != want {
		t.Errorf("stderr:\n got %q\nwant %q", stderr.String(), want)
	}
	if len(rec.records) != 2 || rec.records[0].Action != "promote" || !strings.HasSuffix(rec.records[0].Message, ": ship-promote-quarantine") || rec.records[0].Cycle != 5 ||
		rec.records[1].Action != "recover" || rec.records[1].Message != ".evolve/inbox/processing/cycle-5/task-b.json → .evolve/inbox/task-b.json: cycle-release" {
		t.Errorf("ledger = %+v", rec.records)
	}
	writeItem(t, procPath(inbox, 6, "task-c.json"), `{"id":"task-c","failure_count":5}`)
	if res, err := m.Release(6, "sys", &Policy{Ceiling: 1, SystemLevel: true}); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if c := readJSON(t, filepath.Join(inbox, "task-c.json")); string(c["failure_count"]) != "5" {
		t.Errorf("a system-level failure never bumps: %s", c["failure_count"])
	}
	writeItem(t, procPath(inbox, 7, "task-d.json"), `{"id":"task-d"}`)
	if res, err := m.Release(7, "cycle-failure-release", &Policy{Ceiling: 3}); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if d := readJSON(t, filepath.Join(inbox, "task-d.json")); string(d["failure_count"]) != "1" || string(d["last_failure_reason"]) != `"cycle-failure-release"` {
		t.Errorf("below the ceiling: %v", d)
	}
}

func TestMover_Release_QuarantineFailed_OutcomeErrorAndNoop(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 11, "t7.json"), `{"id":"t7"}`)
	writeItem(t, filepath.Join(inbox, "quarantine"), "x")
	rc := newRecordingCenter()
	var stderr strings.Builder
	m := New(inbox, nil, WithSignals(rc.accessor()))
	legacy := New(inbox, nil, WithStderr(&stderr))
	res, err := m.Release(11, "", &Policy{Ceiling: 1})
	if err != nil || res.Recovered != 1 || res.Paths[0] != filepath.Join(inbox, "t7.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := rc.codes(); len(got) != 2 || got[0] != CodePromoteMoveFailed || got[1] != CodeQuarantineFailed || rc.events[1].Fields["outcome"] != "error" || rc.events[1].Origin != "Mover.Release" || rc.events[0].Origin != "Mover.Promote" {
		t.Errorf("events = %+v", rc.events)
	}
	if rc.events[1].Fields["failure_count"] != "1" || rc.events[1].Fields["ceiling"] != "1" || rc.events[1].Fields["step"] != "quarantine" {
		t.Errorf("fields = %v", rc.events[1].Fields)
	}
	if err := os.Remove(filepath.Join(inbox, "quarantine")); err != nil {
		t.Fatal(err)
	}
	writeItem(t, procPath(inbox, 12, "t8.json"), `{"id":"t8"}`)
	mkdirAll(t, filepath.Join(inbox, "quarantine", "t8.json"))
	rc.events = nil
	if res, err := m.Release(12, "", &Policy{Ceiling: 1}); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := rc.codes(); len(got) != 2 || got[1] != CodeQuarantineFailed || rc.events[1].Fields["outcome"] != "noop" {
		t.Errorf("events = %+v", rc.events)
	}
	writeItem(t, procPath(inbox, 13, "t9.json"), `{"id":"t9"}`)
	mkdirAll(t, filepath.Join(inbox, "quarantine", "t9.json"))
	_, _ = legacy.Release(13, "", &Policy{Ceiling: 1})
	lines := faultLines(stderr.String())
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "[inbox-mover] WARN: promote: mv failed for 't9' → quarantine") ||
		lines[1] != "[inbox-mover] WARN: quarantine no-op for 't9' (task-level failure #1 >= ceiling 1) — not parked, releasing to inbox root instead" {
		t.Errorf("legacy lines: %q", lines)
	}
}

func TestMover_Release_ItemRewriteFailed_Steps(t *testing.T) {
	inbox := newInbox(t)
	item := procPath(inbox, 4, "poison.json")
	writeItem(t, item, `{"id":"poison"}`)
	mkdirAll(t, tmpPathOf(item))
	rc := newRecordingCenter()
	var stderr strings.Builder
	m := New(inbox, nil, WithSignals(rc.accessor()))
	if res, err := m.Release(4, "", &Policy{Ceiling: 1}); err != nil || res.Recovered != 1 || res.Paths[0] != filepath.Join(inbox, "poison.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeItemRewriteFailed || rc.events[0].Fields["step"] != "failure_bump" || rc.events[0].Fields["path"] != item {
		t.Errorf("events = %+v", rc.events)
	}
	if err := os.Remove(tmpPathOf(item)); err != nil {
		t.Fatal(err)
	}
	writeItem(t, item, `{"id":"poison"}`)
	legacy := New(inbox, nil, WithStderr(&stderr))
	_ = os.Remove(filepath.Join(inbox, "poison.json"))
	mkdirAll(t, tmpPathOf(item))
	_, _ = legacy.Release(4, "", &Policy{Ceiling: 1})
	if lines := faultLines(stderr.String()); len(lines) != 1 || lines[0] != "[inbox-mover] WARN: release-cycle: failure_count bump failed for poison.json (open "+tmpPathOf(item)+": is a directory) — quarantine skipped, releasing to inbox root" {
		t.Errorf("legacy line: %q", lines)
	}
	ws := filepath.Join(inbox, "..", "runs", "cycle-9")
	writeItem(t, filepath.Join(ws, "continuation-manifest.json"), `{"snapshot_sha":"abc123","cycle":9}`)
	stamped := procPath(inbox, 9, "t5.json")
	writeItem(t, stamped, `{"id":"t5"}`)
	mkdirAll(t, tmpPathOf(stamped))
	rc.events = nil
	stamper := New(inbox, nil, WithSignals(rc.accessor()), WithRunWorkspace(func(int) string { return ws }))
	if res, err := stamper.Release(9, "", nil); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeItemRewriteFailed || rc.events[0].Fields["step"] != "continuation_stamp" {
		t.Errorf("events = %+v", rc.events)
	}
	if doc := readJSON(t, filepath.Join(inbox, "t5.json")); string(doc["continuation"]) != "" {
		t.Errorf("released unstamped: %s", doc["continuation"])
	}
}

func TestMover_ReleaseFromQuarantine_CounterResetFails_EmitsItemRewriteFailed(t *testing.T) {
	inbox := newInbox(t)
	src := filepath.Join(inbox, "quarantine", "q.json")
	writeItem(t, src, `{"id":"q","failure_count":3,"last_failure_reason":"boom"}`)
	mkdirAll(t, tmpPathOf(src))
	rc := newRecordingCenter()
	rec := &recordingAppender{}
	m := New(inbox, rec, WithSignals(rc.accessor()))
	res, err := m.ReleaseFromQuarantine("q")
	if err != nil || res.DestPath != filepath.Join(inbox, "q.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if doc := readJSON(t, res.DestPath); string(doc["failure_count"]) != "3" {
		t.Errorf("released with the stale count: %s", doc["failure_count"])
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeItemRewriteFailed || rc.events[0].Origin != "Mover.ReleaseFromQuarantine" || rc.events[0].Fields["step"] != "counter_reset" || rc.events[0].Fields["path"] != src {
		t.Errorf("events = %+v", rc.events)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "quarantine-release" || rec.records[0].Message != ".evolve/inbox/quarantine/q.json → .evolve/inbox/q.json: operator-quarantine-release" {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_ReleaseFromQuarantine_Guards(t *testing.T) {
	inbox := newInbox(t)
	m := New(inbox, nil)
	if _, err := m.ReleaseFromQuarantine(""); !errors.Is(err, ErrBadArgs) {
		t.Errorf("empty id: %v", err)
	}
	if _, err := m.ReleaseFromQuarantine("ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no quarantine dir: %v", err)
	}
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q"}`)
	writeItem(t, filepath.Join(inbox, "q.json"), `{"id":"q-root"}`)
	if _, err := m.ReleaseFromQuarantine("q"); !errors.Is(err, ErrMvFailed) {
		t.Errorf("root twin: %v", err)
	}
	if body, _ := os.ReadFile(filepath.Join(inbox, "q.json")); !strings.Contains(string(body), "q-root") {
		t.Error("the root twin is never clobbered")
	}
	// A read-only root fails the rename after the counter reset, a preserved quirk.
	if err := os.Remove(filepath.Join(inbox, "q.json")); err != nil {
		t.Fatal(err)
	}
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","failure_count":3}`)
	if err := os.Chmod(inbox, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	_, err := m.ReleaseFromQuarantine("q")
	if cerr := os.Chmod(inbox, 0o755); cerr != nil {
		t.Fatal(cerr)
	}
	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("the rename into a read-only root must fail (the fault did not happen): %v", err)
	}
	if doc := readJSON(t, filepath.Join(inbox, "quarantine", "q.json")); string(doc["failure_count"]) != "0" {
		t.Errorf("the reset precedes the rename: %s", doc["failure_count"])
	}
}

func TestMover_RecoverOrphans_MoveFailed_EmitsReleaseMoveFailed_StepRecoverOrphans(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 3, "a.json"), `{"id":"a"}`)
	mkdirAll(t, filepath.Join(inbox, "a.json"))
	writeItem(t, procPath(inbox, 3, "b.json"), `{"id":"b"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()), WithActiveCycle(func() (string, error) { return "99", nil }))
	res, err := m.RecoverOrphans()
	if err != nil || res.Recovered != 1 || res.Paths[0] != filepath.Join(inbox, "b.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeReleaseMoveFailed || rc.events[0].Origin != "Mover.RecoverOrphans" || rc.events[0].Fields["step"] != "recover_orphans" || rc.events[0].Cycle != 3 {
		t.Errorf("events = %+v", rc.events)
	}
}

func TestMover_RecoverOrphans_SkipsActiveAndLedgersRecoveries(t *testing.T) {
	inbox := newInbox(t)
	var stderr strings.Builder
	rec := &recordingAppender{}
	m := New(inbox, rec, WithStderr(&stderr), WithActiveCycle(func() (string, error) { return "2", nil }))
	if res, err := m.RecoverOrphans(); err != nil || res.Recovered != 0 {
		t.Fatalf("absent: %+v %v", res, err)
	}
	writeItem(t, filepath.Join(inbox, "processing"), "x")
	if res, err := m.RecoverOrphans(); err != nil || res.Recovered != 0 {
		t.Fatalf("a FILE at processing/: %+v %v", res, err)
	}
	if err := os.Remove(filepath.Join(inbox, "processing")); err != nil {
		t.Fatal(err)
	}
	writeItem(t, filepath.Join(inbox, "dup.json"), `{"id":"dup-original"}`)
	writeItem(t, procPath(inbox, 1, "dup.json"), `{"id":"dup-claimed"}`)
	writeItem(t, procPath(inbox, 2, "live.json"), `{"id":"live"}`)
	res, err := m.RecoverOrphans()
	if err != nil || res.Recovered != 1 || len(res.Paths) != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if body, _ := os.ReadFile(filepath.Join(inbox, "dup.json")); !strings.Contains(string(body), "dup-claimed") {
		t.Error("the processing copy clobbers the root twin (the preserved quirk)")
	}
	want := "[inbox-mover] recover-orphans: no processing/ dir — nothing to do\n[inbox-mover] recover-orphans: no processing/ dir — nothing to do\n[inbox-mover] recovered: dup.json ← processing/cycle-1/\n[inbox-mover] recover-orphans: cycle-2/ is active — skipping\n[inbox-mover] recover-orphans: 1 file(s) recovered\n"
	if stderr.String() != want {
		t.Errorf("stderr:\n got %q\nwant %q", stderr.String(), want)
	}
	if len(rec.records) != 1 || rec.records[0].Cycle != 1 || rec.records[0].Message != ".evolve/inbox/processing/cycle-1/dup.json → .evolve/inbox/dup.json: orphan-recovery-cycle-not-active" {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_Release_ManifestUnreadable_EmitsContinuationManifestUnreadable(t *testing.T) {
	inbox := newInbox(t)
	ws := filepath.Join(inbox, "..", "runs", "cycle-8")
	mkdirAll(t, filepath.Join(ws, "continuation-manifest.json"))
	writeItem(t, procPath(inbox, 8, "t.json"), `{"id":"t"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()), WithRunWorkspace(func(int) string { return ws }))
	if res, err := m.Release(8, "", nil); err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeContinuationManifestUnreadable || rc.events[0].Fields["workspace"] != ws || rc.events[0].Fields["err"] == "" {
		t.Errorf("events = %+v", rc.events)
	}
	if body, _ := os.ReadFile(filepath.Join(inbox, "t.json")); string(body) != `{"id":"t"}` {
		t.Errorf("released unstamped, bytes untouched: %s", body)
	}
}

func TestMover_Release_StampsFromTheManifest(t *testing.T) {
	inbox := newInbox(t)
	ws := filepath.Join(inbox, "..", "runs", "cycle-8")
	writeItem(t, filepath.Join(ws, "continuation-manifest.json"), `{"snapshot_sha":"abc123","cycle":8}`)
	writeItem(t, procPath(inbox, 8, "t.json"), `{"id":"t"}`)
	writeItem(t, procPath(inbox, 9, "u.json"), `{"id":"u"}`)
	m := New(inbox, nil, WithRunWorkspace(func(c int) string { return filepath.Join(inbox, "..", "runs", "cycle-"+strconv.Itoa(c)) }))
	if _, err := m.Release(8, "", nil); err != nil {
		t.Fatal(err)
	}
	if doc := readJSON(t, filepath.Join(inbox, "t.json")); !strings.Contains(string(doc["continuation"]), `"snapshot_sha":"abc123"`) {
		t.Errorf("stamped: %s", doc["continuation"])
	}
	if _, err := m.Release(9, "", nil); err != nil {
		t.Fatal(err)
	}
	if doc := readJSON(t, filepath.Join(inbox, "u.json")); string(doc["continuation"]) != "" {
		t.Errorf("no manifest ⇒ no stamp: %s", doc["continuation"])
	}
}

func TestMover_Release_DirArmsAndMoveFailed(t *testing.T) {
	inbox := newInbox(t)
	var stderr strings.Builder
	rc := newRecordingCenter()
	m := New(inbox, nil, WithStderr(&stderr), WithSignals(rc.accessor()))
	if res, err := m.Release(1, "", nil); err != nil || res.Recovered != 0 {
		t.Fatalf("absent: %+v %v", res, err)
	}
	if stderr.String() != "[inbox-mover] release-cycle: processing/cycle-1/ absent — nothing to release\n" {
		t.Errorf("stderr = %q", stderr.String())
	}
	writeItem(t, filepath.Join(inbox, "processing"), "x")
	if _, err := m.Release(1, "", nil); err == nil || !strings.Contains(err.Error(), "release-cycle: stat processing/cycle-1: ") {
		t.Errorf("a FILE at processing/ is a stat error: %v", err)
	}
	if err := os.Remove(filepath.Join(inbox, "processing")); err != nil {
		t.Fatal(err)
	}
	writeItem(t, filepath.Join(inbox, "processing", "cycle-2"), "x")
	stderr.Reset()
	if res, err := m.Release(2, "", nil); err != nil || res.Recovered != 0 || stderr.Len() != 0 {
		t.Errorf("a FILE at processing/cycle-N is a silent no-op: %+v %v %q", res, err, stderr.String())
	}
	item := procPath(inbox, 3, "t6.json")
	writeItem(t, item, `{"id":"t6"}`)
	if err := os.Chmod(filepath.Dir(item), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(item), 0o755) })
	res, err := m.Release(3, "", nil)
	if err != nil || res.Recovered != 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, statErr := os.Stat(item); statErr != nil {
		t.Fatalf("the read-only cycle dir must have kept the item (the fault did not happen): %v", statErr)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeReleaseMoveFailed || rc.events[0].Origin != "Mover.Release" || rc.events[0].Fields["step"] != "release_cycle" || rc.events[0].Fields["base"] != "t6.json" {
		t.Errorf("events = %+v", rc.events)
	}
}

func TestShouldQuarantine_PureDecision(t *testing.T) {
	if ShouldQuarantine(2, 2, false) != true || ShouldQuarantine(1, 2, false) != false || ShouldQuarantine(5, 0, false) != false || ShouldQuarantine(5, 2, true) != false {
		t.Error("ceiling > 0 && !systemLevel && count >= ceiling")
	}
}
