package auditledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerscan"
)

func benchLedger(b *testing.B) (path, newestRun string) {
	b.Helper()
	body, runs := buildFixture(fixtureShape{rows: 20000, trailingNewline: true})
	path = filepath.Join(b.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		b.Fatal(err)
	}
	return path, runs[len(runs)-1]
}

func BenchmarkLatestAuditorEntryLegacy(b *testing.B) {
	path, run := benchLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := legacyLatestAuditorEntry(path, run); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLatestAuditorEntryScan(b *testing.B) {
	path, run := benchLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := LatestAuditorEntry(path, run); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditorRowsLegacy(b *testing.B) {
	path, _ := benchLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := legacyAuditorRows(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditorRowsScan(b *testing.B) {
	path, _ := benchLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := AuditorRows(path); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPrefilterPassRateOnTheBenchmarkFixture(t *testing.T) {
	body, _ := buildFixture(fixtureShape{rows: 20000, trailingNewline: true})
	if rows := strings.Count(body, "\n"); rows < 10000 {
		t.Fatalf("fixture has %d rows, want 10000 or more", rows)
	}
	p := fixturePath(t, body)
	passed := 0
	if err := ledgerscan.Reverse(p, auditorFilter(), func([]byte) (bool, error) { passed++; return false, nil }); err != nil {
		t.Fatal(err)
	}
	total := 0
	if err := ledgerscan.Reverse(p, ledgerscan.Filter{}, func([]byte) (bool, error) { total++; return false, nil }); err != nil {
		t.Fatal(err)
	}
	t.Logf("prefilter passes %d of %d rows (%.0f%%)", passed, total, 100*float64(passed)/float64(total))
	if passed >= total || passed == 0 {
		t.Fatalf("passed=%d total=%d: the prefilter must drop some rows and keep the auditor rows", passed, total)
	}
}
