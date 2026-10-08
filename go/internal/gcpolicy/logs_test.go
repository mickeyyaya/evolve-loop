package gcpolicy_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

const megabyte = int64(1) << 20

func catalogByName(t *testing.T, cats []gcpolicy.LogCategory) map[string]gcpolicy.LogCategory {
	t.Helper()
	out := make(map[string]gcpolicy.LogCategory, len(cats))
	for _, c := range cats {
		if _, dup := out[c.Name]; dup {
			t.Fatalf("category %q appears twice in the catalog", c.Name)
		}
		out[c.Name] = c
	}
	return out
}

func TestLogCatalog_DefaultsTakeLogsTTLDaysAndTheCompiledSizeCaps(t *testing.T) {
	got := catalogByName(t, gcpolicy.Policy{LogsTTLDays: 17}.LogCatalog())

	want := map[string]gcpolicy.LogCategory{
		gcpolicy.LogCategoryDispatch: {Name: gcpolicy.LogCategoryDispatch, TTLDays: 17, MaxTotalBytes: 512 * megabyte,
			Homes: []gcpolicy.LogHome{{Dir: "dispatch-logs", Glob: "*.log"}}},
		gcpolicy.LogCategoryLoop: {Name: gcpolicy.LogCategoryLoop, TTLDays: 17, MaxTotalBytes: 2048 * megabyte,
			Homes: []gcpolicy.LogHome{{Dir: gcpolicy.LogsDir, Glob: gcpolicy.LogRunDirGlob, IsDir: true}}},
		gcpolicy.LogCategoryConsole: {Name: gcpolicy.LogCategoryConsole, TTLDays: 17, MaxTotalBytes: 1024 * megabyte,
			Homes: []gcpolicy.LogHome{{Dir: ".", Glob: "loop-*.log"}, {Dir: ".", Glob: "wave*.log"}, {Dir: gcpolicy.LogsDir, Glob: "batch-*.log"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LogCatalog() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLogCatalog_AnUnsetLogsTTLDaysTakesTheThirtyDayDefault(t *testing.T) {
	for _, c := range (gcpolicy.Policy{}).LogCatalog() {
		if c.TTLDays != 30 {
			t.Errorf("category %q TTLDays = %d, want the logs_ttl_days default 30", c.Name, c.TTLDays)
		}
	}
}

func TestLogCatalog_APolicyEntryChangesOnlyItsOwnCategory(t *testing.T) {
	p := gcpolicy.Policy{LogsTTLDays: 9, Logs: gcpolicy.LogsPolicy{Loop: gcpolicy.LogRetention{TTLDays: 3, MaxTotalMB: 7}}}

	got := catalogByName(t, p.LogCatalog())

	if l := got[gcpolicy.LogCategoryLoop]; l.TTLDays != 3 || l.MaxTotalBytes != 7*megabyte {
		t.Errorf("loop = ttl %d cap %d, want ttl 3 cap %d from gc.logs.loop", l.TTLDays, l.MaxTotalBytes, 7*megabyte)
	}
	if c := got[gcpolicy.LogCategoryConsole]; c.TTLDays != 9 || c.MaxTotalBytes != 1024*megabyte {
		t.Errorf("console = ttl %d cap %d, want ttl 9 (logs_ttl_days) and the compiled cap", c.TTLDays, c.MaxTotalBytes)
	}
}

func TestLogCatalog_ANonPositiveCapTakesTheCompiledCap(t *testing.T) {
	p := gcpolicy.Policy{Logs: gcpolicy.LogsPolicy{Dispatch: gcpolicy.LogRetention{MaxTotalMB: -4}}}

	if d := catalogByName(t, p.LogCatalog())[gcpolicy.LogCategoryDispatch]; d.MaxTotalBytes != 512*megabyte {
		t.Errorf("dispatch cap = %d, want the compiled %d for a negative max_total_mb", d.MaxTotalBytes, 512*megabyte)
	}
}

func TestLogsPolicy_DecodesFromTheGCLogsKey(t *testing.T) {
	var p gcpolicy.Policy
	raw := `{"logs_ttl_days":12,"logs":{"dispatch":{"ttl_days":2},"loop":{"ttl_days":4,"max_total_mb":5},"console":{"max_total_mb":6}}}`
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := gcpolicy.LogsPolicy{
		Dispatch: gcpolicy.LogRetention{TTLDays: 2},
		Loop:     gcpolicy.LogRetention{TTLDays: 4, MaxTotalMB: 5},
		Console:  gcpolicy.LogRetention{MaxTotalMB: 6},
	}
	if p.Logs != want || p.LogsTTLDays != 12 {
		t.Errorf("decoded logs=%+v logs_ttl_days=%d, want %+v and 12", p.Logs, p.LogsTTLDays, want)
	}
}

func TestToolOutputFiles_NamesTheRawToolLogsAndReturnsACopy(t *testing.T) {
	got := gcpolicy.ToolOutputFiles()
	want := []string{"ship-repocontract-scan.log", "integration-tier.log"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToolOutputFiles() = %v, want %v", got, want)
	}

	got[0] = "changed"

	if again := gcpolicy.ToolOutputFiles(); again[0] != want[0] {
		t.Errorf("a caller changed the catalog through the returned slice: %v", again)
	}
}

func TestLogRunID_UsesTheUTCLayoutThatTheLoopGlobMatches(t *testing.T) {
	at := time.Date(2026, 10, 8, 14, 25, 1, 0, time.FixedZone("x", 8*3600))

	id := gcpolicy.LogRunID(at)

	if id != "20261008T062501Z" {
		t.Errorf("LogRunID = %q, want the UTC stamp 20261008T062501Z", id)
	}
	for _, name := range []string{id, id + "-legacy"} {
		if !gcpolicy.IsLogRunDir(name) {
			t.Errorf("IsLogRunDir(%q) = false, want true", name)
		}
	}
	for _, name := range []string{gcpolicy.LogsCurrentLink, "batch-20260814.log", "2026-10-08", "x20261008T062501Z"} {
		if gcpolicy.IsLogRunDir(name) {
			t.Errorf("IsLogRunDir(%q) = true, want false", name)
		}
	}
}

func TestIsPollutedArchive_MatchesOnlyTheNameThatTheWorkspaceGuardMints(t *testing.T) {
	at := time.Date(2026, 9, 1, 21, 14, 50, 11361000, time.UTC)
	minted := "cycle-1604" + gcpolicy.PollutedArchiveName(at)

	cases := []struct {
		name string
		want bool
	}{
		{minted, true},
		{"cycle-1604.polluted-20260901T211450.011361000", true},
		{"cycle-1604", false},
		{"cycle-1749.reset-20260928T204122.641389000", false},
		{"cycle-1604.polluted-", false},
		{"notes.polluted-20260901T211450.011361000", false},
		{"cycle-1604.polluted-20260901T211450.011361000-copy", false},
	}
	for _, c := range cases {
		if got := gcpolicy.IsPollutedArchive(c.name); got != c.want {
			t.Errorf("IsPollutedArchive(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoopLogLayout_PinsTheNamesThatTheDocsAndTheConsoleMonitorsRead(t *testing.T) {
	got := []string{gcpolicy.LogsDir, gcpolicy.LogsCurrentLink, gcpolicy.LoopLogName, gcpolicy.LogWriterPIDSuffix}
	want := []string{"logs", "current", "loop.log", ".writer-pid"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loop log layout = %v, want %v: .evolve/boundary-loop.log links to logs/current/loop.log", got, want)
	}
}

func TestIsLogRunID_MatchesOnlyTheExactRunIDLayout(t *testing.T) {
	id := gcpolicy.LogRunID(time.Date(2026, 10, 8, 14, 25, 1, 0, time.UTC))
	cases := map[string]bool{
		id:                       true,
		id + "-legacy":           false,
		id + "x":                 false,
		"2026100814250Z":         false,
		"20261008T142501":        false,
		"../20261008T142501Z":    false,
		"20261008T142501Z/..":    false,
		gcpolicy.LogsCurrentLink: false,
	}
	for name, want := range cases {
		if got := gcpolicy.IsLogRunID(name); got != want {
			t.Errorf("IsLogRunID(%q) = %v, want %v", name, got, want)
		}
	}
}
