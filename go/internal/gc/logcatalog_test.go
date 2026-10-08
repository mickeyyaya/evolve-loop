package gc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func writeAged(t *testing.T, path string, size int, mod time.Time) {
	t.Helper()
	writeFile(t, path, strings.Repeat("x", size))
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func ageDir(t *testing.T, dir string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(dir, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func planLogs(t *testing.T, evolveDir string, pol Policy) map[string]Item {
	t.Helper()
	m, err := Plan(Options{EvolveDir: evolveDir, Policy: pol, Now: nowT0})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return planItems(t, m)
}

func TestPlan_LogCatalogDeletesExpiredLogsOfEachCategoryByItsTTLRule(t *testing.T) {
	dir := t.TempDir()
	expired := map[string]string{
		filepath.Join(dir, "dispatch-logs", "batch-1.log"):    "logs.dispatch.ttl_days",
		filepath.Join(dir, "loop-20260914-1140-verify.log"):   "logs.console.ttl_days",
		filepath.Join(dir, "wave4-20260827.log"):              "logs.console.ttl_days",
		filepath.Join(dir, "logs", "batch-20260814.log"):      "logs.console.ttl_days",
		filepath.Join(dir, "logs", "20260501T000000Z"):        "logs.loop.ttl_days",
		filepath.Join(dir, "logs", "20260502T000000Z-legacy"): "logs.loop.ttl_days",
	}
	for p := range expired {
		if strings.HasSuffix(filepath.Dir(p), "logs") && !strings.HasSuffix(p, ".log") {
			writeAged(t, filepath.Join(p, gcpolicy.LoopLogName), 10, daysAgo(20))
			ageDir(t, p, daysAgo(20))
			continue
		}
		writeAged(t, p, 10, daysAgo(20))
	}
	fresh := filepath.Join(dir, "loop-20260611-wave.log")
	writeAged(t, fresh, 10, daysAgo(2))

	items := planLogs(t, dir, Policy{LogsTTLDays: 14})

	for p, rule := range expired {
		if it := items[p]; it.Action != ActionDelete || it.Rule != rule {
			t.Errorf("%s: planned %+v, want delete by %s", p, it, rule)
		}
	}
	if it, ok := items[fresh]; ok {
		t.Errorf("a log younger than the TTL was planned: %+v", it)
	}
}

func TestPlan_LogCatalogNeverPlansForeignFilesOrSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeAged(t, filepath.Join(dir, "guards.log"), 10, daysAgo(90))
	writeAged(t, filepath.Join(dir, "dispatch-logs", "keep.txt"), 10, daysAgo(90))
	run := filepath.Join(dir, "logs", "20260101T000000Z")
	writeAged(t, filepath.Join(run, gcpolicy.LoopLogName), 10, daysAgo(90))
	ageDir(t, run, daysAgo(90))
	symlink(t, filepath.Join("logs", gcpolicy.LogsCurrentLink, gcpolicy.LoopLogName), filepath.Join(dir, "boundary-loop.log"))
	symlink(t, filepath.Join(run, gcpolicy.LoopLogName), filepath.Join(dir, "loop-link.log"))
	symlink(t, run, filepath.Join(dir, "logs", "20260102T000000Z"))

	m, err := Plan(Options{EvolveDir: dir, Policy: Policy{LogsTTLDays: 1}, Now: func() time.Time { return time.Now().Add(400 * 24 * time.Hour) }})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)

	for _, p := range []string{"guards.log", "dispatch-logs/keep.txt", "boundary-loop.log", "loop-link.log", "logs/20260102T000000Z"} {
		if it, ok := items[filepath.Join(dir, p)]; ok {
			t.Errorf("%s is not a catalog log, but it was planned: %+v", p, it)
		}
	}
	if it := items[run]; it.Rule != "logs.loop.ttl_days" {
		t.Errorf("the real run dir behind the symlinks: planned %+v, want delete by logs.loop.ttl_days", it)
	}
}

func TestPlan_LogCatalogAgesALoopRunDirByItsNewestFile(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "logs", "20260601T000000Z")
	writeAged(t, filepath.Join(active, gcpolicy.LoopLogName), 10, daysAgo(1))
	writeAged(t, filepath.Join(active, "diag.log"), 10, daysAgo(40))
	ageDir(t, active, daysAgo(40))

	items := planLogs(t, dir, Policy{LogsTTLDays: 14})

	if it, ok := items[active]; ok {
		t.Errorf("a run dir whose loop.log grew yesterday was planned by its old dir mtime: %+v", it)
	}
}

func TestPlan_LogCatalogNeverPlansTheCurrentRunDir(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "logs", "20260101T000000Z")
	writeAged(t, filepath.Join(current, gcpolicy.LoopLogName), 4<<20, daysAgo(90))
	ageDir(t, current, daysAgo(90))
	symlink(t, filepath.Base(current), filepath.Join(dir, "logs", gcpolicy.LogsCurrentLink))

	items := planLogs(t, dir, Policy{LogsTTLDays: 1, Logs: gcpolicy.LogsPolicy{Loop: gcpolicy.LogRetention{MaxTotalMB: 1}}})

	if it, ok := items[current]; ok {
		t.Errorf("the target of logs/current was planned: %+v", it)
	}
}

func TestPlan_LogCatalogDeletesTheOldestLogsOverTheSizeCap(t *testing.T) {
	dir := t.TempDir()
	newest := filepath.Join(dir, "loop-c.log")
	middle := filepath.Join(dir, "loop-b.log")
	oldest := filepath.Join(dir, "logs", "batch-a.log")
	writeAged(t, newest, 600<<10, daysAgo(1))
	writeAged(t, middle, 300<<10, daysAgo(2))
	writeAged(t, oldest, 300<<10, daysAgo(3))

	items := planLogs(t, dir, Policy{LogsTTLDays: 30, Logs: gcpolicy.LogsPolicy{Console: gcpolicy.LogRetention{MaxTotalMB: 1}}})

	if it := items[oldest]; it.Action != ActionDelete || it.Rule != "logs.console.max_total_mb" {
		t.Errorf("the oldest log over the 1 MB cap: planned %+v, want delete by logs.console.max_total_mb", it)
	}
	for _, p := range []string{newest, middle} {
		if it, ok := items[p]; ok {
			t.Errorf("%s fits under the cap (900 KiB in total), but it was planned: %+v", filepath.Base(p), it)
		}
	}
}

func TestPlan_LogCatalogCapCountsOnlyTheLogsThatTheTTLKeeps(t *testing.T) {
	dir := t.TempDir()
	expired := filepath.Join(dir, "loop-old.log")
	kept := filepath.Join(dir, "loop-new.log")
	writeAged(t, expired, 900<<10, daysAgo(40))
	writeAged(t, kept, 900<<10, daysAgo(1))

	items := planLogs(t, dir, Policy{LogsTTLDays: 30, Logs: gcpolicy.LogsPolicy{Console: gcpolicy.LogRetention{MaxTotalMB: 1}}})

	if it := items[expired]; it.Rule != "logs.console.ttl_days" {
		t.Errorf("expired log: planned %+v, want delete by logs.console.ttl_days", it)
	}
	if it, ok := items[kept]; ok {
		t.Errorf("the kept log fits the cap once the expired log goes, but it was planned: %+v", it)
	}
}

func writeWriterPID(t *testing.T, logPath string, pid int) {
	t.Helper()
	writeFile(t, logPath+gcpolicy.LogWriterPIDSuffix, strconv.Itoa(pid))
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("start a short process: %v", err)
	}
	return cmd.Process.Pid
}

func TestPlan_LogCatalogCountsTheCurrentRunDirTowardTheCap(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "logs", "20260610T000000Z")
	other := filepath.Join(dir, "logs", "20260609T000000Z")
	writeAged(t, filepath.Join(current, gcpolicy.LoopLogName), 900<<10, daysAgo(1))
	writeAged(t, filepath.Join(other, gcpolicy.LoopLogName), 300<<10, daysAgo(2))
	ageDir(t, current, daysAgo(1))
	ageDir(t, other, daysAgo(2))
	symlink(t, filepath.Base(current), filepath.Join(dir, "logs", gcpolicy.LogsCurrentLink))

	items := planLogs(t, dir, Policy{LogsTTLDays: 30, Logs: gcpolicy.LogsPolicy{Loop: gcpolicy.LogRetention{MaxTotalMB: 1}}})

	if it := items[other]; it.Rule != "logs.loop.max_total_mb" {
		t.Errorf("the current dir holds 900 KiB of the 1 MB cap, so the older 300 KiB dir must go: planned %+v", it)
	}
	if it, ok := items[current]; ok {
		t.Errorf("the current dir was planned: %+v", it)
	}
}

func TestPlan_LogCatalogWarnsWhenTheKeptLogsStayOverTheCap(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "logs", "20260610T000000Z")
	writeAged(t, filepath.Join(current, gcpolicy.LoopLogName), 2<<20, daysAgo(1))
	ageDir(t, current, daysAgo(1))
	symlink(t, filepath.Base(current), filepath.Join(dir, "logs", gcpolicy.LogsCurrentLink))

	m, err := Plan(Options{EvolveDir: dir, Policy: Policy{Logs: gcpolicy.LogsPolicy{Loop: gcpolicy.LogRetention{MaxTotalMB: 1}}}, Now: nowT0})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if len(m.Warnings) != 1 || !strings.Contains(m.Warnings[0], "logs.loop.max_total_mb") {
		t.Errorf("Warnings = %q, want one line that names logs.loop.max_total_mb", m.Warnings)
	}
}

func TestPlan_LogCatalogNeverPlansALogThatALiveLoopWrites(t *testing.T) {
	dir := t.TempDir()
	liveDir := filepath.Join(dir, "logs", "20260101T000000Z")
	deadDir := filepath.Join(dir, "logs", "20260102T000000Z")
	liveFile := filepath.Join(dir, "loop-manual.log")
	for _, d := range []string{liveDir, deadDir} {
		writeAged(t, filepath.Join(d, gcpolicy.LoopLogName), 10, daysAgo(90))
	}
	writeAged(t, liveFile, 10, daysAgo(90))
	writeWriterPID(t, filepath.Join(liveDir, gcpolicy.LoopLogName), os.Getpid())
	writeWriterPID(t, filepath.Join(deadDir, gcpolicy.LoopLogName), deadPID(t))
	writeWriterPID(t, liveFile, os.Getpid())

	m, err := Plan(Options{EvolveDir: dir, Policy: Policy{LogsTTLDays: 1}, Now: func() time.Time { return time.Now().Add(400 * 24 * time.Hour) }})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)

	for _, p := range []string{liveDir, liveFile} {
		if it, ok := items[p]; ok {
			t.Errorf("%s has a live writer, but it was planned: %+v", p, it)
		}
	}
	if it := items[deadDir]; it.Rule != "logs.loop.ttl_days" {
		t.Errorf("a run dir whose writer is dead: planned %+v, want delete by logs.loop.ttl_days", it)
	}
}

func TestPlan_LogCatalogDeletesTheWriterPIDFileWithItsLog(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "loop-manual.log")
	writeAged(t, logFile, 10, daysAgo(90))
	writeWriterPID(t, logFile, deadPID(t))

	m, err := Plan(Options{EvolveDir: dir, Policy: Policy{LogsTTLDays: 1}, Now: func() time.Time { return time.Now().Add(400 * 24 * time.Hour) }})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)

	for _, p := range []string{logFile, logFile + gcpolicy.LogWriterPIDSuffix} {
		if it := items[p]; it.Rule != "logs.console.ttl_days" {
			t.Errorf("%s: planned %+v, want delete by logs.console.ttl_days", filepath.Base(p), it)
		}
	}
}

func TestPlan_LogCatalogKeepsALogWhoseWriterPIDFileCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "logs", "20260101T000000Z")
	writeAged(t, filepath.Join(run, gcpolicy.LoopLogName), 10, daysAgo(90))
	if err := os.MkdirAll(filepath.Join(run, gcpolicy.LoopLogName+gcpolicy.LogWriterPIDSuffix), 0o755); err != nil {
		t.Fatal(err)
	}

	m, err := Plan(Options{EvolveDir: dir, Policy: Policy{LogsTTLDays: 1}, Now: func() time.Time { return time.Now().Add(400 * 24 * time.Hour) }})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if it, ok := planItems(t, m)[run]; ok {
		t.Errorf("an unreadable writer pid file must count as a live writer, but the dir was planned: %+v", it)
	}
}
