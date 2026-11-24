package auditledger

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var realLedgerCopy = flag.String("real-ledger-copy", "", "path to a COPY of a real ledger for the equivalence test")

func legacyAuditorRows(ledgerPath string) ([]Entry, error) {
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", ledgerPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	var rows []Entry
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if IsAuditorRow(e) {
			rows = append(rows, e)
		}
	}
	return rows, nil
}

func legacyLatestAuditorEntry(ledgerPath, runID string) (Entry, error) {
	rows, err := legacyAuditorRows(ledgerPath)
	if err != nil {
		return Entry{}, err
	}
	return BindRun(rows, runID)
}

func blobSize(i int) int {
	if i%1500 == 7 {
		return 70000
	}
	return 40
}

func sampleRuns(runs []string) []string {
	var out []string
	for i := 0; i < len(runs); i += len(runs)/12 + 1 {
		out = append(out, runs[i])
	}
	return append(out, runs[len(runs)-1])
}

type fixtureShape struct {
	rows            int
	trailingNewline bool
	truncatedLast   bool
	malformedMiddle bool
}

func buildFixture(shape fixtureShape) (string, []string) {
	var sb strings.Builder
	var runs []string
	for i := 0; i < shape.rows; i++ {
		run := fmt.Sprintf("run-%04d", i/7)
		switch i % 9 {
		case 0:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"role":"auditor","kind":"agent_subprocess","run_id":%q,"exit_code":0,"artifact_path":"/a/%d.md","artifact_sha256":"sha%d","git_head":"h%d","worktree_tree_sha":"w%d","worktree_base_sha":"b%d"}`+"\n", i, i, run, i, i, i, i, i)
			runs = append(runs, run)
		case 1:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"role":"builder","kind":"agent_subprocess","run_id":%q}`+"\n", i, i, run)
		case 2:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"role":"orchestrator","kind":"routing_decision","note":%q}`+"\n", i, i, strings.Repeat("n", 200+i%300))
		case 3:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"role":"auditor","kind":"phase","run_id":%q}`+"\n", i, i, run)
		case 4:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":"%d","kind":"agent_subprocess","role":"auditor","run_id":%q,"exit_code":1}`+"\n", i, i, run)
			runs = append(runs, run)
		case 5:
			fmt.Fprintf(&sb, `{"ts":"t%d","kind":"phase_skipped","role":"rollback-plan"}`+"\n", i)
		case 6:
			fmt.Fprintf(&sb, "\n  {\"ts\":\"t%d\",\"role\":\"auditor\",\"kind\":\"agent_subprocess\",\"run_id\":%q}  \r\n", i, run)
			runs = append(runs, run)
		case 7:
			fmt.Fprintf(&sb, `{"ts":"t%d","role":"auditor","kind":"agent_subprocess","run_id":%q,"blob":%q}`+"\n", i, run, strings.Repeat("B", blobSize(i)))
			runs = append(runs, run)
		default:
			fmt.Fprintf(&sb, `{"ts":"t%d","role":"auditor","kind":"agent_subprocess","run_id":"","exit_code":0}`+"\n", i)
		}
		if shape.malformedMiddle && i == shape.rows/2 {
			sb.WriteString(`{"role":"auditor","kind":"agent_subprocess","run_id":"torn` + "\n")
		}
	}
	out := sb.String()
	if shape.truncatedLast {
		out += `{"role":"auditor","kind":"agent_subprocess","run_id":"trunc`
	}
	if !shape.trailingNewline && !shape.truncatedLast {
		out = strings.TrimSuffix(out, "\n")
	}
	return out, runs
}

func fixturePath(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertEquivalent(t *testing.T, path string, runs []string) {
	t.Helper()
	oldRows, oldErr := legacyAuditorRows(path)
	newRows, newErr := AuditorRows(path)
	if fmt.Sprint(oldErr) != fmt.Sprint(newErr) || !reflect.DeepEqual(oldRows, newRows) {
		t.Fatalf("AuditorRows differ: old %d rows err=%v, new %d rows err=%v", len(oldRows), oldErr, len(newRows), newErr)
	}
	probe := append([]string{"", "run-absent", "run-0000"}, runs...)
	if len(probe) > 40 {
		probe = append([]string{"", "run-absent", "run-0000"}, sampleRuns(runs)...)
	}
	for _, run := range probe {
		oldE, oldErr := legacyLatestAuditorEntry(path, run)
		newE, newErr := LatestAuditorEntry(path, run)
		if fmt.Sprint(oldErr) != fmt.Sprint(newErr) || oldE != newE {
			t.Fatalf("LatestAuditorEntry(%q) differs: old %+v err=%v, new %+v err=%v", run, oldE, oldErr, newE, newErr)
		}
	}
}

func TestReadersMatchTheLegacyReaderOnARealShapedLedger(t *testing.T) {
	for name, shape := range map[string]fixtureShape{
		"clean, trailing newline":     {rows: 12000, trailingNewline: true},
		"no trailing newline":         {rows: 12000},
		"truncated last line":         {rows: 12000, truncatedLast: true},
		"malformed middle line":       {rows: 12000, trailingNewline: true, malformedMiddle: true},
		"malformed and truncated end": {rows: 12000, truncatedLast: true, malformedMiddle: true},
	} {
		t.Run(name, func(t *testing.T) {
			body, runs := buildFixture(shape)
			if len(body) < 1<<20 {
				t.Fatalf("fixture is %d bytes; it must span many 64 KiB chunks", len(body))
			}
			assertEquivalent(t, fixturePath(t, body), runs)
		})
	}
}

func TestReadersMatchTheLegacyReaderOnEdgeLedgers(t *testing.T) {
	for name, body := range map[string]string{
		"empty":            "",
		"blank lines only": "\n\n  \n",
		"no auditor row":   `{"role":"builder","kind":"agent_subprocess","run_id":"r"}` + "\n",
		"one row no eol":   `{"role":"auditor","kind":"agent_subprocess","run_id":"r","exit_code":2}`,
		"nbsp padded":      " " + `{"role":"auditor","kind":"agent_subprocess","run_id":"r"}` + " \n",
		"upper-case keys":  `{"ROLE":"auditor","Kind":"agent_subprocess","RUN_ID":"r"}` + "\n",
		"duplicate keys":   `{"role":"x","role":"auditor","kind":"agent_subprocess","run_id":"r"}` + "\n",
		"escaped values":   `{"role":"auditor","kind":"agent_subprocess","run_id":"r-1"}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			assertEquivalent(t, fixturePath(t, body), []string{"r", "r-1"})
		})
	}
}

func TestReadersMatchTheLegacyReaderOnAMissingLedger(t *testing.T) {
	assertEquivalent(t, filepath.Join(t.TempDir(), "none.jsonl"), nil)
}

func TestReadersMatchTheLegacyReaderOnARealLedgerCopy(t *testing.T) {
	if *realLedgerCopy == "" {
		t.Skip("pass -real-ledger-copy=<path to a copy> to run")
	}
	rows, err := legacyAuditorRows(*realLedgerCopy)
	if err != nil {
		t.Fatal(err)
	}
	var runs []string
	for _, r := range rows {
		runs = append(runs, r.RunID)
	}
	assertEquivalent(t, *realLedgerCopy, runs)
}

func TestAuditorFilterPassesEveryRowTheAuditorPredicateAccepts(t *testing.T) {
	lines := []string{
		`{"role":"auditor","kind":"agent_subprocess"}`,
		`{"kind":"agent_subprocess","role":"auditor","run_id":"r"}`,
		`{"ROLE":"auditor","KIND":"agent_subprocess"}`,
		`{"role":"auditor","kind":"agent_subprocess"}`,
		`{"role":"auditor","kind":"agent_subprocess"}`,
		`{"role":"x","role":"auditor","kind":"agent_subprocess"}`,
	}
	f := auditorFilter()
	for _, line := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil || !IsAuditorRow(e) {
			t.Fatalf("premise: %s is not an auditor row (%v)", line, err)
		}
		if !f.Pass([]byte(line)) {
			t.Errorf("the prefilter drops an auditor row: %s", line)
		}
	}
	if f.Pass([]byte(`{"role":"builder","kind":"agent_subprocess"}`)) {
		t.Error("the prefilter must drop a builder row")
	}
}

func TestAuditorFilterNeedlesComeFromThePredicateConstants(t *testing.T) {
	got := auditorFilter().Needles
	if len(got) != 2 || string(got[0]) != AuditorKind || string(got[1]) != AuditorRole {
		t.Fatalf("needles = %q, want [%s %s]", got, AuditorKind, AuditorRole)
	}
	if !IsAuditorRow(Entry{Kind: AuditorKind, Role: AuditorRole}) {
		t.Fatal("IsAuditorRow must accept exactly the constant pair")
	}
}
