package loopchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// --- 38. the pure decisions ---

func TestStartDecision_Table(t *testing.T) {
	cases := []struct {
		n, max, pending int
		brake           bool
		reason          string
		stop            bool
	}{
		{0, 3, 2, false, "", false},
		{1, 3, 0, false, StopInboxEmpty, true},
		{3, 3, 5, false, StopMaxBatches, true},
		{0, 3, 5, true, StopOperatorBrake, true},
		{0, 3, 0, true, StopOperatorBrake, true},
		{2, 3, 1, false, "", false},
		{0, 20, 0, false, "", false},
		{0, 0, 0, false, StopMaxBatches, true},
		{7, 20, 0, false, StopInboxEmpty, true},
	}
	for _, c := range cases {
		if reason, stop := StartDecision(c.n, c.max, c.pending, c.brake); reason != c.reason || stop != c.stop {
			t.Errorf("StartDecision(%d,%d,%d,%v) = (%q,%v), want (%q,%v)", c.n, c.max, c.pending, c.brake, reason, stop, c.reason, c.stop)
		}
	}
}

func TestContinueDecision_Table(t *testing.T) {
	cases := []struct {
		rc     int
		reason string
		exit   int
		stop   bool
	}{
		{0, "", 0, false}, {3, "", 3, false}, {5, StopQuotaDefer, 5, true}, {2, StopBatchError, 2, true}, {4, StopBatchError, 4, true}, {130, StopBatchError, 130, true},
	}
	for _, c := range cases {
		if reason, exit, stop := ContinueDecision(c.rc); reason != c.reason || exit != c.exit || stop != c.stop {
			t.Errorf("ContinueDecision(%d) = (%q,%d,%v)", c.rc, reason, exit, stop)
		}
	}
}

func TestStopReasons_AreTheSevenWireStrings(t *testing.T) {
	want := map[string]string{
		StopOperatorBrake: "chain_operator_brake", StopInboxEmpty: "chain_inbox_empty", StopMaxBatches: "chain_max_batches",
		StopQuotaDefer: "chain_quota_defer", StopBatchError: "chain_batch_error", StopInboxUnreadable: "chain_inbox_unreadable",
		StopBoundaryRefreshReexec: "chain_boundary_refresh_reexec",
	}
	for got, spelling := range want {
		if got != spelling {
			t.Errorf("%q != %q", got, spelling)
		}
	}
	if len(want) != 7 {
		t.Error("seven stop reasons")
	}
}

// --- 39. the inbox count, the brake, the config ---

func TestInboxPendingCount_SkipsDirsNonJSONAndNamesInvalidFiles(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	writeJSON(t, filepath.Join(inbox, "good.json"), map[string]any{"id": "real-item", "weight": 0.88})
	writeJSON(t, filepath.Join(inbox, "processed", "done.json"), map[string]any{"id": "done"})
	for name, body := range map[string]string{"empty.json": "", "array.json": `[{"id":"a"}]`, "noid.json": `{"weight":0.5}`, "blankid.json": `{"id":""}`, "README.md": "not a todo"} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	n, skipped, err := InboxPendingCount(evolveDir)
	if err != nil || n != 1 || strings.Join(skipped, ",") != "array.json,blankid.json,empty.json,noid.json" {
		t.Errorf("(%d, %v, %v): one real item, four named skips, no dirs, no non-json", n, skipped, err)
	}
	if n, skipped, err := InboxPendingCount(filepath.Join(t.TempDir(), "nope")); err != nil || n != 0 || skipped != nil {
		t.Errorf("a missing inbox is zero with no skips: %d %v %v", n, skipped, err)
	}
	if err := os.RemoveAll(inbox); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := InboxPendingCount(evolveDir); err == nil || !strings.HasPrefix(err.Error(), "read inbox: ") {
		t.Errorf("any other read error is returned wrapped: %v", err)
	}
	if isInboxItemFile(filepath.Join(t.TempDir(), "absent.json")) {
		t.Error("an unreadable file is not an item")
	}
}

func TestBrakeEngaged(t *testing.T) {
	evolveDir := t.TempDir()
	if BrakeEngaged(evolveDir) {
		t.Error("no brake file")
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "loop-stop"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !BrakeEngaged(evolveDir) {
		t.Error(".evolve/loop-stop engages the brake")
	}
}

func TestLoadChainConfig_DefaultsOnError(t *testing.T) {
	dir := t.TempDir()
	if cc := LoadChainConfig(dir); cc.Enabled || cc.MaxBatches != policy.DefaultChainMaxBatches {
		t.Errorf("absent policy: %+v", cc)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cc := LoadChainConfig(dir); cc.Enabled {
		t.Errorf("malformed policy defaults: %+v", cc)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(`{"chain":{"enabled":true,"max_batches":4}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cc := LoadChainConfig(dir); !cc.Enabled || cc.MaxBatches != 4 {
		t.Errorf("a chain block resolves: %+v", cc)
	}
}

// --- 40. the last refresh entry ---

func TestLastRefreshLogEntry_MostRecentMissingEmptyUnparseable(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, LogFile)
	if got, err := LastRefreshLogEntry(logPath); got != nil || err != nil {
		t.Errorf("missing: (%v, %v)", got, err)
	}
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := LastRefreshLogEntry(logPath); got != nil || err != nil {
		t.Errorf("empty: (%v, %v)", got, err)
	}
	var buf bytes.Buffer
	for _, e := range []RefreshLogEntry{{Batch: 1, AuthorizedClass: AuthorizedClassBoundaryRefresh, Timestamp: "t1", OldSHA: "a", NewSHA: "b"}, {Batch: 9, AuthorizedClass: AuthorizedClassBoundaryRefresh, Timestamp: "t9", OldSHA: "b", NewSHA: "c"}} {
		line, _ := json.Marshal(e)
		buf.Write(line)
		buf.WriteString("\n")
	}
	buf.WriteString("\n") // a trailing blank line is skipped
	if err := os.WriteFile(logPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := LastRefreshLogEntry(logPath); err != nil || got == nil || got.Batch != 9 || got.NewSHA != "c" || got.Timestamp != "t9" {
		t.Errorf("the LAST entry: %+v %v", got, err)
	}
	if err := os.WriteFile(logPath, append(buf.Bytes(), []byte("{corrupt\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := LastRefreshLogEntry(logPath); got != nil || err != nil {
		t.Errorf("an unparseable last line is (nil, nil) — Q-C4: (%v, %v)", got, err)
	}
}

// --- 48. the fleet-lane check ---

func liveRun(t *testing.T, evolveDir, name string, pid int) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: name, OwnerPID: pid}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestFleetLaneActive_ExcludesOwnPIDLease(t *testing.T) {
	evolveDir := t.TempDir()
	if active, err := FleetLaneActive(evolveDir); err != nil || active {
		t.Errorf("no runs dir is inactive: %v %v", active, err)
	}
	liveRun(t, evolveDir, "cycle-own", os.Getpid())
	if active, err := FleetLaneActive(evolveDir); err != nil || active {
		t.Errorf("our own fresh lease is not a sibling: %v %v", active, err)
	}
	// A sibling is a DIFFERENT LIVE process: os.Getpid()+1 only happened to
	// be alive; since liveness is the owner's (2026-09-15) the fixture holds a
	// real child for the duration.
	sibling := exec.Command("sleep", "30")
	if err := sibling.Start(); err != nil {
		t.Fatalf("spawn a live sibling: %v", err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })
	liveRun(t, evolveDir, "cycle-sibling", sibling.Process.Pid)
	if active, err := FleetLaneActive(evolveDir); err != nil || !active {
		t.Errorf("a foreign LIVE pid is a sibling: %v %v", active, err)
	}
	// Live without a lease at all (the current-workspace liveness source) is
	// deliberately NOT excluded — but only a fresh lease or the current
	// workspace makes a dir Live; a bare run dir is dead.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, "runs", "cycle-dead"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bare, "runs", "cycle-dead", "run.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if active, err := FleetLaneActive(bare); err != nil || active {
		t.Errorf("a dead run dir is inactive: %v %v", active, err)
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "runs"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FleetLaneActive(broken); err == nil || !strings.Contains(err.Error(), "fleet-lane discovery") {
		t.Errorf("a discovery error is returned: %v", err)
	}
}

// --- 50-54. the Driver ---

// chainFixture scripts a Driver over a temp project with `items` pending.
type chainFixture struct {
	root, evolveDir string
	stderr          bytes.Buffer
	sig             *signals
	deps            DriverDeps
	batches         int
	refreshAt       int
	widths          []int
}

func newChainFixture(t *testing.T, items int) *chainFixture {
	t.Helper()
	f := &chainFixture{sig: newSignals(), widths: []int{1}}
	f.root, f.evolveDir, _ = project(t)
	for i := 0; i < items; i++ {
		writeJSON(t, filepath.Join(f.evolveDir, "inbox", fmt.Sprintf("item-%d.json", i)), map[string]any{"id": fmt.Sprintf("item-%d", i)})
	}
	f.deps = DriverDeps{
		Batch:   func() int { f.batches++; return 0 },
		Refresh: func(batch int) bool { return batch == f.refreshAt },
		LastRefresh: func() (*RefreshLogEntry, error) {
			return &RefreshLogEntry{Batch: 2, AuthorizedClass: AuthorizedClassBoundaryRefresh, Timestamp: "2026-09-14T10:00:00Z", OldSHA: "STALE_PIN", NewSHA: "7448e675c81e3a4ca21703a47a15ab7b8a4fdc957a3267d43dbe6b7edf34d753"}, nil
		},
		FleetWidth: func() int {
			w := f.widths[0]
			if len(f.widths) > 1 {
				f.widths = f.widths[1:]
			}
			return w
		},
		QuotaPause: func() (QuotaPause, bool) { return QuotaPause{}, false },
	}
	return f
}

func (f *chainFixture) run(cap int) Result {
	return NewDriver(Roots{ProjectRoot: f.root, EvolveDir: f.evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: cap}, f.deps, &f.stderr, WithSignals(f.sig.accessor())).Run()
}

// resultJSON renders the Result exactly as the host prints it (MarshalIndent
// + newline): the Result IS the chain's stdout wire schema, so the fold-0
// goldens (captured through runLoopChain) replay here with no view struct.
func resultJSON(t *testing.T, r Result) string {
	t.Helper()
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(buf) + "\n"
}

// --- review fold R1: the chain summary schema has ONE home ---

// TestResult_IsTheChainSummaryWireSchema pins the stdout contract on the leaf
// type itself: the five wire keys in order, `boundary_refresh` omitted when
// nil, the exit code never on the wire, and ChainMode stamped by Run — a field
// added or a tag renamed here changes the host's stdout, and this test.
func TestResult_IsTheChainSummaryWireSchema(t *testing.T) {
	full := Result{ChainMode: true, MaxBatches: 5, Batches: []BatchRecord{{Batch: 1, RC: 0, FleetCount: 1, InboxPending: 3}}, StopReason: StopBoundaryRefreshReexec, Exit: 7,
		BoundaryRefresh: &RefreshLogEntry{Batch: 2, AuthorizedClass: AuthorizedClassBoundaryRefresh, Timestamp: "t", OldSHA: "a", NewSHA: "b"}}
	// keys lists the top-level keys in wire order (each spelled once in the
	// document, so byte offset is order).
	keys := func(r Result) []string {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(raw, &top); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(top))
		for k := range top {
			out = append(out, k)
		}
		sort.Slice(out, func(i, j int) bool {
			return bytes.Index(raw, []byte(`"`+out[i]+`":`)) < bytes.Index(raw, []byte(`"`+out[j]+`":`))
		})
		return out
	}
	if got := strings.Join(keys(full), ","); got != "chain_mode,max_batches,batches,chain_stop_reason,boundary_refresh" {
		t.Errorf("the wire keys, in order: %s", got)
	}
	if got := strings.Join(keys(Result{}), ","); got != "chain_mode,max_batches,batches,chain_stop_reason" {
		t.Errorf("boundary_refresh is omitted when nil and Exit never appears: %s", got)
	}
	f := newChainFixture(t, 0)
	if r := f.run(5); !r.ChainMode {
		t.Error("Run stamps ChainMode: a Result IS a chain run")
	}
}

func driverSection(t *testing.T, name string) string {
	t.Helper()
	for _, s := range strings.Split(golden(t, "stderr_driver.golden.txt"), "== ") {
		n, body, _ := strings.Cut(s, "\n")
		if n == name {
			return body
		}
	}
	t.Fatalf("no golden section %q", name)
	return ""
}

func TestDriver_StopPrecedenceIsBrakeRefreshDrainedCap(t *testing.T) {
	t.Run("brake", func(t *testing.T) {
		f := newChainFixture(t, 2)
		if err := os.WriteFile(filepath.Join(f.evolveDir, "loop-stop"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		f.refreshAt = 1
		r := f.run(5)
		if r.StopReason != StopOperatorBrake || r.Exit != 0 || f.batches != 0 || resultJSON(t, r) != golden(t, "chain_result_brake.golden.json") || f.stderr.String() != driverSection(t, "brake") {
			t.Errorf("the brake outranks the refresh and runs no batch: %+v %q", r, f.stderr.String())
		}
	})
	t.Run("boundary refresh", func(t *testing.T) {
		f := newChainFixture(t, 3)
		f.refreshAt = 2
		r := f.run(5)
		if r.StopReason != StopBoundaryRefreshReexec || r.BoundaryRefresh == nil || r.BoundaryRefresh.Batch != 2 || f.batches != 1 ||
			template(resultJSON(t, r), f.root, f.evolveDir) != golden(t, "chain_result_boundary_refresh_reexec.golden.json") {
			t.Errorf("a refresh stops the chain before that boundary's batch with the last log entry: %+v", r)
		}
		want := "[chain] batch 1/5 starting — inbox pending=3, fleet lanes=1\n[chain] stopping after 1 batch(es): chain_boundary_refresh_reexec — re-exec is terminal, the new process resumes the chain\n"
		if f.stderr.String() != want || len(*f.sig.events) != 0 {
			t.Errorf("the kept lines, no event: %q %v", f.stderr.String(), f.sig.codes())
		}
		f.deps.LastRefresh = func() (*RefreshLogEntry, error) { return nil, errors.New("unreadable") }
		if r := f.run(5); r.BoundaryRefresh != nil {
			t.Errorf("a failed last-entry read leaves the field nil: %+v", r.BoundaryRefresh)
		}
	})
	t.Run("drained", func(t *testing.T) {
		f := newChainFixture(t, 0)
		r := f.run(5)
		if r.StopReason != StopInboxEmpty || f.batches != 1 || resultJSON(t, r) != golden(t, "chain_result_inbox_empty.golden.json") || f.stderr.String() != driverSection(t, "inbox_empty") {
			t.Errorf("a drained inbox runs exactly one batch: %+v %q", r, f.stderr.String())
		}
	})
	t.Run("cap", func(t *testing.T) {
		f := newChainFixture(t, 3)
		r := f.run(2)
		if r.StopReason != StopMaxBatches || f.batches != 2 || resultJSON(t, r) != golden(t, "chain_result_max_batches.golden.json") || f.stderr.String() != driverSection(t, "max_batches") {
			t.Errorf("the cap is exact: %+v %q", r, f.stderr.String())
		}
	})
}

func TestDriver_BatchErrorAndInboxUnreadableAreLoopHaltIncidents(t *testing.T) {
	f := newChainFixture(t, 3)
	f.deps.Batch = func() int { f.batches++; return 2 }
	r := f.run(5)
	ev := f.sig.only(t, CodeChainBatchError)
	if r.StopReason != StopBatchError || r.Exit != 2 || f.batches != 1 || resultJSON(t, r) != golden(t, "chain_result_batch_error.golden.json") ||
		ev.Kind != signalcenter.KindLoopHalt || ev.Severity != signalcenter.SeverityIncident || ev.Origin != "Driver.Run" || ev.Cycle != 0 ||
		ev.Reason != "batch 1 exited rc=2 — stopping the chain (chain_batch_error)" || ev.Fields["batch"] != "1" || ev.Fields["rc"] != "2" || ev.Fields["stop_reason"] != StopBatchError {
		t.Errorf("rc=2 is a loop.halt INCIDENT: %+v %+v", r, ev)
	}
	if f.stderr.String() != "[chain] batch 1/5 starting — inbox pending=3, fleet lanes=1\n" {
		t.Errorf("only the kept line on stderr: %q", f.stderr.String())
	}
	f = newChainFixture(t, 2)
	f.deps.Batch = func() int { f.batches++; return 3 }
	if r := f.run(3); r.StopReason != StopMaxBatches || f.batches != 3 || len(*f.sig.events) != 0 {
		t.Errorf("rc=3 keeps chaining silently: %+v %v", r, f.sig.codes())
	}
	f = newChainFixture(t, 0)
	if err := os.RemoveAll(filepath.Join(f.evolveDir, "inbox")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.evolveDir, "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = f.run(5)
	ev = f.sig.only(t, CodeChainInboxUnreadable)
	if r.StopReason != StopInboxUnreadable || r.Exit != 2 || f.batches != 0 || resultJSON(t, r) != golden(t, "chain_result_inbox_unreadable.golden.json") ||
		ev.Kind != signalcenter.KindLoopHalt || ev.Severity != signalcenter.SeverityIncident || ev.Fields["path"] != filepath.Join(f.evolveDir, "inbox") || ev.Fields["stop_reason"] != StopInboxUnreadable || ev.Fields["exit"] != "2" || ev.Fields["batch"] != "1" ||
		template(ev.Reason, f.root, f.evolveDir) != "cannot read the inbox (read inbox: open {EVOLVE_DIR}/inbox: not a directory) — stopping the chain rather than looping blind" {
		t.Errorf("an unreadable inbox is a loop.halt INCIDENT: %+v %+v", r, ev)
	}
}

func TestDriver_QuotaDeferIsAWarnWithTheCheckpointFields(t *testing.T) {
	f := newChainFixture(t, 3)
	f.deps.Batch = func() int { f.batches++; return 5 }
	f.deps.QuotaPause = func() (QuotaPause, bool) {
		return QuotaPause{Cycle: 7, WakeAt: "2026-09-14T18:00:00Z", Source: "probe"}, true
	}
	r := f.run(5)
	ev := f.sig.only(t, CodeChainQuotaDefer)
	if r.StopReason != StopQuotaDefer || r.Exit != 5 || f.batches != 1 || resultJSON(t, r) != golden(t, "chain_result_quota_defer.golden.json") ||
		ev.Kind != signalcenter.KindLoopWarning || ev.Severity != signalcenter.SeverityWarn || ev.Reason != "batch 1 hit the quota wall (cycle=7 wake-at=2026-09-14T18:00:00Z source=probe) — DEFERRING, not relaunching" ||
		ev.Fields["batch"] != "1" || ev.Fields["cycle"] != "7" || ev.Fields["wake_at"] != "2026-09-14T18:00:00Z" || ev.Fields["source"] != "probe" {
		t.Errorf("rc=5 is a loop.warning WARN with the checkpoint fields: %+v %+v", r, ev)
	}
	want := "[chain] batch 1/5 starting — inbox pending=3, fleet lanes=1\n[chain]   the checkpoint is intact; resume when quota resets: evolve loop --resume\n"
	if f.stderr.String() != want {
		t.Errorf("the hint stays a line: %q", f.stderr.String())
	}
	f = newChainFixture(t, 3)
	f.deps.Batch = func() int { f.batches++; return 5 }
	f.run(5)
	if ev := f.sig.only(t, CodeChainQuotaDefer); ev.Reason != "batch 1 hit the quota wall (no checkpoint block on disk) — DEFERRING, not relaunching" || ev.Fields["cycle"] != "" || ev.Fields["batch"] != "1" {
		t.Errorf("no block: the fields are empty: %+v", ev)
	}
}

func TestDriver_InvalidInboxItemIsNamedBeforeTheStopAndNotCounted(t *testing.T) {
	f := newChainFixture(t, 1)
	if err := os.WriteFile(filepath.Join(f.evolveDir, "inbox", "typo-item.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.evolveDir, "loop-stop"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.run(5)
	ev := f.sig.only(t, CodeChainInboxItemInvalid)
	if r.StopReason != StopOperatorBrake || ev.Fields["name"] != "typo-item.json" || ev.Fields["path"] != filepath.Join(f.evolveDir, "inbox", "typo-item.json") || ev.Fields["batch"] != "1" ||
		ev.Reason != "skipping .evolve/inbox/typo-item.json — not a valid inbox item (no parseable object with an `id`); it is NOT counted as pending work" || ev.Kind != signalcenter.KindLoopWarning {
		t.Errorf("named before the stop: %+v %+v", r, ev)
	}
	if !strings.Contains(f.stderr.String(), "inbox pending=1") {
		t.Errorf("the invalid file is not counted: %q", f.stderr.String())
	}
}

func TestDriver_FleetWidthIsRecordedFreshEveryBatchNeverNarrowed(t *testing.T) {
	f := newChainFixture(t, 5)
	f.widths = []int{3, 5}
	r := f.run(2)
	if len(r.Batches) != 2 || r.Batches[0].FleetCount != 3 || r.Batches[1].FleetCount != 5 || r.Batches[1].InboxPending != 5 || r.Batches[1].Batch != 2 {
		t.Errorf("width read fresh per batch: %+v", r.Batches)
	}
	if !strings.Contains(f.stderr.String(), "fleet lanes=3") || !strings.Contains(f.stderr.String(), "fleet lanes=5") {
		t.Errorf("each batch's line names its width: %q", f.stderr.String())
	}
}

func TestDriver_NullObjectAndSignalsWired(t *testing.T) {
	f := newChainFixture(t, 0)
	f.deps.Batch = func() int { return 2 }
	var d *Driver = NewDriver(Roots{ProjectRoot: f.root, EvolveDir: f.evolveDir}, policy.ChainConfig{MaxBatches: 5}, f.deps, &f.stderr)
	if d.SignalsWired() {
		t.Error("no accessor is not wired")
	}
	if r := d.Run(); r.StopReason != StopBatchError {
		t.Errorf("the decision holds without a Center: %+v", r)
	}
	r := f.refresherNull()
	if r.SignalsWired() {
		t.Error("a nil Center is not wired")
	}
	if NewDriver(Roots{}, policy.ChainConfig{}, f.deps, &f.stderr, WithSignals(f.sig.accessor())).SignalsWired() != true {
		t.Error("an accessor returning a Center is wired")
	}
}

func (f *chainFixture) refresherNull() *Refresher {
	return NewRefresher(Roots{ProjectRoot: f.root, EvolveDir: f.evolveDir}, RefreshDeps{}, &f.stderr, WithSignals(func() *signalcenter.Center { return nil }))
}

// --- review fold R4: one producer per console sentence ---

// TestDriver_StopLineHasOneProducer pins the unit's own rule (§4
// Fold-the-twin): the "[chain] stopping after …" sentence the brake and the
// start decision share is spelled ONCE in the Driver.
func TestDriver_StopLineHasOneProducer(t *testing.T) {
	src, err := os.ReadFile("driver.go")
	if err != nil {
		t.Fatal(err)
	}
	const sentence = `"[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n"`
	if n := strings.Count(string(src), sentence); n != 1 {
		t.Errorf("the stop sentence has %d producers in driver.go, want exactly 1 (Driver.stop)", n)
	}
}

// A sealed lane's lease outlives its process: the writer stops heartbeating
// at exit but the file stays "fresh" for a full TTL (cycle 1679 sealed at
// 17:03Z; the boundary refresh at 17:03 read its 17:02 heartbeat as an active
// sibling and refused to rebuild — LOOP_BOUNDARY_REFRESH_SKIPPED step=lane_active
// with no lane running, twice on 2026-09-15). Liveness is the OWNER, not the
// timestamp: a fresh lease whose owner pid is gone is not a sibling; a lease
// with no owner pid stays a sibling (nothing to probe — refuse conservatively).
func TestFleetLaneActive_DeadOwnerIsNotASibling(t *testing.T) {
	evolveDir := t.TempDir()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn a process to retire: %v", err)
	}
	dead := cmd.Process.Pid
	liveRun(t, evolveDir, "cycle-sealed", dead)
	if active, err := FleetLaneActive(evolveDir); err != nil || active {
		t.Errorf("a fresh lease whose owner pid %d has exited is a sealed lane, not a sibling: active=%v err=%v", dead, active, err)
	}
	liveRun(t, evolveDir, "cycle-unowned", 0)
	if active, err := FleetLaneActive(evolveDir); err != nil || !active {
		t.Errorf("a fresh lease with no owner pid cannot be proven dead — still a sibling: active=%v err=%v", active, err)
	}
}
