package gc

// L3.1 rule-eval tests on synthetic trees. The engine's contract:
//   - newest KeepFull runs are kept in full, however old;
//   - beyond KeepFull, dead runs age into archive then delete (0 disables);
//   - LIVE runs are never targeted, no matter their age;
//   - quarantine + ledger paths are never targeted even if discovery
//     hands them in (manual-only / append-only hard rules);
//   - salvage and dispatch-log TTLs delete stale entries only;
//   - .ephemeral subtrees of KEPT runs age out on the tracker TTL.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

func nowT0() time.Time { return t0 }

func daysAgo(n int) time.Time { return t0.Add(-time.Duration(n) * 24 * time.Hour) }

// mkRun creates a synthetic run dir with the given mtime.
func mkRun(t *testing.T, evolveDir, name string, mod time.Time) RunDir {
	t.Helper()
	p := filepath.Join(evolveDir, "runs", name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
	return RunDir{Path: p, ModTime: mod}
}

func planItems(t *testing.T, m Manifest) map[string]Item {
	t.Helper()
	out := make(map[string]Item, len(m.Items))
	for _, it := range m.Items {
		out[it.Path] = it
	}
	return out
}

func TestPlan_KeepFullProtectsNewestRegardlessOfAge(t *testing.T) {
	dir := t.TempDir()
	// Three ancient runs, keep_full=2, delete_after=30: only the OLDEST may go.
	r1 := mkRun(t, dir, "cycle-1", daysAgo(400))
	r2 := mkRun(t, dir, "cycle-2", daysAgo(300))
	r3 := mkRun(t, dir, "cycle-3", daysAgo(200))
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 2, DeleteAfterDays: 30}},
		Runs:      []RunDir{r1, r2, r3},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if _, ok := items[r1.Path]; !ok {
		t.Errorf("oldest run beyond keep_full must be deleted; manifest: %+v", m.Items)
	}
	if it, ok := items[r1.Path]; ok && it.Action != ActionDelete {
		t.Errorf("expected delete for %s, got %s", r1.Path, it.Action)
	}
	for _, kept := range []RunDir{r2, r3} {
		if _, ok := items[kept.Path]; ok {
			t.Errorf("run %s is within keep_full=2 and must be kept", kept.Path)
		}
	}
}

func TestPlan_ArchiveThenDeleteLadder(t *testing.T) {
	dir := t.TempDir()
	old := mkRun(t, dir, "cycle-10", daysAgo(90))    // beyond delete_after=60
	middle := mkRun(t, dir, "cycle-11", daysAgo(30)) // beyond archive_after=14
	fresh := mkRun(t, dir, "cycle-12", daysAgo(2))   // beyond keep_full but young
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 0, ArchiveAfterDays: 14, DeleteAfterDays: 60}},
		Runs:      []RunDir{old, middle, fresh},
		Now:       nowT0,
	})
	// KeepFull: 0 means "use the default 10" (zero value = defaults) — that
	// would keep everything. Use 1 explicitly to exercise the ladder.
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(m.Items) != 0 {
		t.Fatalf("zero-value KeepFull must default to 10 and keep all three: %+v", m.Items)
	}

	m, err = Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1, ArchiveAfterDays: 14, DeleteAfterDays: 60}},
		Runs:      []RunDir{old, middle, fresh},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if it := items[old.Path]; it.Action != ActionDelete {
		t.Errorf("90d-old run: want delete, got %+v", it)
	}
	if it := items[middle.Path]; it.Action != ActionArchive {
		t.Errorf("30d-old run: want archive, got %+v", it)
	}
	if _, ok := items[fresh.Path]; ok {
		t.Errorf("2d-old run is younger than both thresholds and must be kept")
	}
}

func TestPlan_ZeroThresholdsDisableActions(t *testing.T) {
	dir := t.TempDir()
	ancient := mkRun(t, dir, "cycle-20", daysAgo(1000))
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1}}, // no archive_after, no delete_after
		Runs:      []RunDir{mkRun(t, dir, "cycle-21", daysAgo(1)), ancient},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(m.Items) != 0 {
		t.Errorf("with both thresholds 0 (disabled) nothing may be planned: %+v", m.Items)
	}
}

func TestPlan_LiveRunNeverTargeted(t *testing.T) {
	dir := t.TempDir()
	live := mkRun(t, dir, "cycle-30", daysAgo(500))
	live.Live = true
	dead := mkRun(t, dir, "cycle-31", daysAgo(400))
	newer := mkRun(t, dir, "cycle-32", daysAgo(1))
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1, DeleteAfterDays: 30}},
		Runs:      []RunDir{live, dead, newer},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if _, ok := items[live.Path]; ok {
		t.Errorf("LIVE run must never be targeted, however old")
	}
	if _, ok := items[dead.Path]; !ok {
		t.Errorf("dead 400d run beyond keep_full must be deleted")
	}
}

func TestPlan_QuarantineAndLedgerNeverTargeted(t *testing.T) {
	dir := t.TempDir()
	q := filepath.Join(dir, "quarantine", "cycle-7")
	if err := os.MkdirAll(q, 0o755); err != nil {
		t.Fatal(err)
	}
	// Even if a (buggy or adversarial) discovery hands quarantine or ledger
	// paths in as run dirs, the engine refuses.
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1, DeleteAfterDays: 1}},
		Runs: []RunDir{
			{Path: q, ModTime: daysAgo(900)},
			{Path: filepath.Join(dir, "ledger.jsonl"), ModTime: daysAgo(900)},
			{Path: filepath.Join(dir, "ledger-segments", "seg-0001.jsonl.gz"), ModTime: daysAgo(900)},
			{Path: filepath.Join(dir, "runs", "cycle-40"), ModTime: daysAgo(1)}, // the keep_full slot
		},
		Now: nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, it := range m.Items {
		if it.Path == q || filepath.Base(it.Path) == "ledger.jsonl" ||
			filepath.Dir(it.Path) == filepath.Join(dir, "ledger-segments") {
			t.Errorf("protected path targeted: %+v", it)
		}
	}
}

func TestPlan_SalvageAndLogTTLs(t *testing.T) {
	dir := t.TempDir()
	oldSalvage := filepath.Join(dir, "operator-salvage", "cycle-282-main-tree")
	freshSalvage := filepath.Join(dir, "operator-salvage", "cycle-300-main-tree")
	for p, mod := range map[string]time.Time{oldSalvage: daysAgo(45), freshSalvage: daysAgo(2)} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	logsDir := filepath.Join(dir, "dispatch-logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldLog := filepath.Join(logsDir, "batch-1.log")
	freshLog := filepath.Join(logsDir, "batch-2.log")
	for p, mod := range map[string]time.Time{oldLog: daysAgo(45), freshLog: daysAgo(2)} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}

	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{SalvageTTLDays: 30, LogsTTLDays: 30},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if it := items[oldSalvage]; it.Action != ActionDelete {
		t.Errorf("45d salvage beyond 30d TTL: want delete, got %+v", it)
	}
	if _, ok := items[freshSalvage]; ok {
		t.Errorf("fresh salvage must be kept")
	}
	if it := items[oldLog]; it.Action != ActionDelete {
		t.Errorf("45d dispatch log beyond 30d TTL: want delete, got %+v", it)
	}
	if _, ok := items[freshLog]; ok {
		t.Errorf("fresh dispatch log must be kept")
	}
}

func TestPlan_TrackerEphemeralTTLInsideKeptRuns(t *testing.T) {
	dir := t.TempDir()
	kept := mkRun(t, dir, "cycle-50", daysAgo(1))
	eph := filepath.Join(kept.Path, ".ephemeral")
	if err := os.MkdirAll(eph, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(eph, daysAgo(10), daysAgo(10)); err != nil {
		t.Fatal(err)
	}
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{TrackerTTLDays: 7},
		Runs:      []RunDir{kept},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if it := items[eph]; it.Action != ActionDelete {
		t.Errorf("10d .ephemeral beyond 7d TTL inside a kept run: want delete, got %+v", it)
	}
	if _, ok := items[kept.Path]; ok {
		t.Errorf("the kept run itself must not be targeted")
	}
}

func TestApply_ArchiveMovesAndDeleteRemoves(t *testing.T) {
	dir := t.TempDir()
	arch := mkRun(t, dir, "cycle-60", daysAgo(40))
	if err := os.WriteFile(filepath.Join(arch.Path, "scout-report.md"), []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	del := mkRun(t, dir, "cycle-61", daysAgo(90))
	m := Manifest{Items: []Item{
		{Path: arch.Path, Action: ActionArchive, Rule: "runs.archive_after_days"},
		{Path: del.Path, Action: ActionDelete, Rule: "runs.delete_after_days"},
	}}
	if err := Apply(dir, m); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(arch.Path); !os.IsNotExist(err) {
		t.Errorf("archived run must be moved away from runs/")
	}
	moved := filepath.Join(dir, "archive", "runs", "cycle-60", "scout-report.md")
	if _, err := os.Stat(moved); err != nil {
		t.Errorf("archived content must survive at %s: %v", moved, err)
	}
	if _, err := os.Stat(del.Path); !os.IsNotExist(err) {
		t.Errorf("deleted run must be gone")
	}
}

func TestApply_RefusesProtectedPaths(t *testing.T) {
	dir := t.TempDir()
	q := filepath.Join(dir, "quarantine", "cycle-9")
	if err := os.MkdirAll(q, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(ledger, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(dir, "archive", "runs", "cycle-8")
	if err := os.MkdirAll(archived, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Apply(dir, Manifest{Items: []Item{
		{Path: q, Action: ActionDelete, Rule: "x"},
		{Path: ledger, Action: ActionDelete, Rule: "x"},
		{Path: archived, Action: ActionDelete, Rule: "x"},
	}})
	if err == nil {
		t.Fatal("Apply must refuse protected (quarantine/ledger/archive) paths with an error")
	}
	for _, p := range []string{q, ledger, archived} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("protected path %s must survive a refused Apply: %v", p, statErr)
		}
	}
}

func TestApply_RequiresAbsoluteEvolveDir(t *testing.T) {
	if err := Apply("relative/.evolve", Manifest{}); err == nil {
		t.Fatal("Apply must refuse a relative evolveDir (archive dst would resolve against CWD)")
	}
}

// H1 pin: the live-run hard rule covers the run's SUBTREES too — a live
// run's stale .ephemeral must not be planned away (deleting a running
// session's tracker state would corrupt it).
func TestPlan_LiveRunEphemeralNeverTargeted(t *testing.T) {
	dir := t.TempDir()
	live := mkRun(t, dir, "cycle-70", daysAgo(1))
	live.Live = true
	eph := filepath.Join(live.Path, ".ephemeral")
	if err := os.MkdirAll(eph, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(eph, daysAgo(30), daysAgo(30)); err != nil {
		t.Fatal(err)
	}
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{TrackerTTLDays: 7},
		Runs:      []RunDir{live},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(m.Items) != 0 {
		t.Errorf("a LIVE run's .ephemeral must never be targeted: %+v", m.Items)
	}
}

// KeepFull semantic pin: the count is over the newest N run dirs by mtime,
// live or dead — a live run inside the window consumes a slot (live runs
// are protected independently, so the window is purely positional).
func TestPlan_KeepFullCountsLiveRunsPositionally(t *testing.T) {
	dir := t.TempDir()
	liveNewest := mkRun(t, dir, "cycle-80", daysAgo(1))
	liveNewest.Live = true
	deadMiddle := mkRun(t, dir, "cycle-81", daysAgo(50))
	deadOldest := mkRun(t, dir, "cycle-82", daysAgo(60))
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 2, DeleteAfterDays: 30}},
		Runs:      []RunDir{liveNewest, deadMiddle, deadOldest},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if _, ok := items[deadMiddle.Path]; ok {
		t.Errorf("dead run at position 2 of keep_full=2 must be kept (positional window)")
	}
	if it := items[deadOldest.Path]; it.Action != ActionDelete {
		t.Errorf("dead run beyond the positional window must be deleted, got %+v", it)
	}
}
