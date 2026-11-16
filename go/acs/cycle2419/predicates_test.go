//go:build acs

package cycle2419

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildDir  string
	buildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveCLI(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
		if err != nil {
			buildErr = err
			return
		}
		if buildDir, err = os.MkdirTemp("", "evolve-acs-2419-"); err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(buildDir, "evolve")
		out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", builtBin, "./cmd/evolve")
		if runErr != nil || code != 0 {
			buildErr = &buildFailure{tail: acsassert.OutputTail(out+errOut, 20)}
		}
	})
	if buildErr != nil {
		t.Fatalf("build evolve: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct{ tail string }

func (b *buildFailure) Error() string { return b.tail }

func exitOf(code int) *int { return &code }

func tryRecord(rung, class, fingerprint string, exit *int) llmcalls.Record {
	return llmcalls.Record{Rung: rung, FailureClass: class, Fingerprint: fingerprint, ExitCode: exit}
}

func rowFor(t *testing.T, rows []retryladder.ReportRow, rung retryladder.Rung, class string) retryladder.ReportRow {
	t.Helper()
	for _, row := range rows {
		if row.Rung == rung && row.Class == class {
			return row
		}
	}
	t.Fatalf("report has no row for %s/%q: %+v", rung, class, rows)
	return retryladder.ReportRow{}
}

func TestC2419_001_ReportCountsTheDefectsEachRungFixed(t *testing.T) {
	runs := [][]llmcalls.Record{{
		tryRecord("R1", "capability", "codes:floor", exitOf(0)),
		tryRecord("R1", "capability", "codes:floor", exitOf(0)),
		tryRecord("R1", "capability", "codes:cover", exitOf(0)),
	}}
	row := rowFor(t, retryladder.BuildReport(runs), retryladder.R1, "capability")
	want := map[string]int{"codes:floor": 2, "codes:cover": 1}
	if row.Fixes != 3 || !reflect.DeepEqual(row.Defects, want) {
		t.Errorf("RED: R1 row fixes=%d defects=%v, want fixes=3 defects=%v", row.Fixes, row.Defects, want)
	}
}

func TestC2419_002_OnlyAPassedRaisedRungOwnsAFixedDefect(t *testing.T) {
	runs := [][]llmcalls.Record{{
		tryRecord("R1", "capability", "codes:floor", exitOf(1)),
		tryRecord("R1", "capability", "codes:floor", nil),
		tryRecord("R0", "", "codes:floor", exitOf(0)),
		tryRecord("R2", "capability", "", exitOf(0)),
	}}
	rows := retryladder.BuildReport(runs)
	for _, row := range rows {
		if row.Rung == retryladder.R1 && (row.Fixes != 0 || len(row.Defects) != 0) {
			t.Errorf("RED: a failed or unfinished R1 try fixed %v (fixes=%d), want no defect", row.Defects, row.Fixes)
		}
		if row.Rung == retryladder.R0 && len(row.Defects) != 0 {
			t.Errorf("RED: R0 is the base try and fixes no defect, got %v", row.Defects)
		}
	}
	r2 := rowFor(t, rows, retryladder.R2, "capability")
	if r2.Fixes != 1 || len(r2.Defects) != 0 {
		t.Errorf("RED: an R2 fix with no fingerprint = fixes %d defects %v, want 1 fix and no named defect", r2.Fixes, r2.Defects)
	}
}

func TestC2419_003_DefectsCountSeparatelyPerRungAcrossRuns(t *testing.T) {
	runs := [][]llmcalls.Record{
		{tryRecord("R1", "capability", "codes:floor", exitOf(0))},
		{tryRecord("R1", "capability", "codes:floor", exitOf(0)), tryRecord("R2", "capability", "codes:floor", exitOf(0))},
	}
	rows := retryladder.BuildReport(runs)
	r1 := rowFor(t, rows, retryladder.R1, "capability")
	r2 := rowFor(t, rows, retryladder.R2, "capability")
	if r1.Defects["codes:floor"] != 2 || r2.Defects["codes:floor"] != 1 {
		t.Errorf("RED: R1 defects=%v R2 defects=%v, want codes:floor 2 and 1", r1.Defects, r2.Defects)
	}
}

func TestC2419_004_RowJSONCarriesDefectsOnlyWhenARungFixedOne(t *testing.T) {
	named := retryladder.BuildReport([][]llmcalls.Record{{tryRecord("R1", "capability", "codes:floor", exitOf(0))}})
	raw, err := json.Marshal(named[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fields["defects"], map[string]any{"codes:floor": float64(1)}) {
		t.Errorf("RED: row json %s lacks defects {codes:floor:1}", raw)
	}
	bare := retryladder.BuildReport([][]llmcalls.Record{{tryRecord("R1", "capability", "", exitOf(0))}})
	raw, _ = json.Marshal(bare[0])
	if strings.Contains(string(raw), "defects") {
		t.Errorf("RED: a row with no named defect still prints defects: %s", raw)
	}
}

const (
	ledgerWithDefects = `{"phase":"build","rung":"R0","exit_code":1}
{"phase":"build","rung":"R1","failure_class":"capability","fingerprint":"codes:floor","exit_code":0}
{"phase":"tdd","rung":"R1","failure_class":"capability","fingerprint":"codes:floor","exit_code":0}
{"phase":"build","rung":"R1","failure_class":"capability","fingerprint":"codes:cover","exit_code":1}
{"phase":"build","rung":"R2","failure_class":"capability","fingerprint":"codes:cover","exit_code":0}
`
	ledgerWithoutDefects = `{"phase":"build","rung":"R0","exit_code":1}
{"phase":"build","rung":"R1","failure_class":"capability","exit_code":0}
`
)

func ladderProject(t *testing.T, ledger string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "llm-calls.ndjson"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func ladderReport(t *testing.T, root string, extra ...string) string {
	t.Helper()
	args := append([]string{"ladder", "report", "--project-root", root}, extra...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveCLI(t), args...)
	if code != 0 {
		t.Fatalf("evolve %v exit %d: %s", args, code, stderr)
	}
	return stdout
}

func TestC2419_005_LadderReportJSONNamesTheDefectsOfEachRung(t *testing.T) {
	out := ladderReport(t, ladderProject(t, ledgerWithDefects), "--json")
	var got struct {
		Rows []struct {
			Rung    string         `json:"rung"`
			Fixes   int            `json:"fixes"`
			Defects map[string]int `json:"defects"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("RED: ladder report --json is not one JSON object: %v\n%s", err, out)
	}
	byRung := map[string]map[string]int{}
	for _, row := range got.Rows {
		byRung[row.Rung] = row.Defects
	}
	if !reflect.DeepEqual(byRung["R1"], map[string]int{"codes:floor": 2}) || !reflect.DeepEqual(byRung["R2"], map[string]int{"codes:cover": 1}) {
		t.Errorf("RED: defects by rung = %v, want R1 {codes:floor:2} and R2 {codes:cover:1}", byRung)
	}
	if len(byRung["R0"]) != 0 {
		t.Errorf("RED: R0 names defects %v, want none", byRung["R0"])
	}
}

func TestC2419_006_LadderReportTextListsTheDefectsBelowTheUnchangedTable(t *testing.T) {
	out := ladderReport(t, ladderProject(t, ledgerWithDefects))
	var header []string
	var defectRows [][]string
	inDefects := false
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		fields := strings.Fields(line)
		switch {
		case reflect.DeepEqual(fields, []string{"RUNG", "DEFECT", "FIXES"}):
			inDefects = true
		case inDefects && len(fields) > 0:
			defectRows = append(defectRows, fields)
		case !inDefects && len(fields) > 0 && header == nil:
			header = fields
		}
	}
	if !reflect.DeepEqual(header, []string{"RUNG", "CLASS", "TRIES", "FIXES", "TOKENS"}) {
		t.Errorf("RED: the first table header = %v, want the unchanged five columns", header)
	}
	want := [][]string{{"R1", "codes:floor", "2"}, {"R2", "codes:cover", "1"}}
	if !reflect.DeepEqual(defectRows, want) {
		t.Errorf("RED: defect rows = %v, want %v (a RUNG DEFECT FIXES table after the main table)\n%s", defectRows, want, out)
	}
}

func TestC2419_007_LadderReportPrintsNoDefectTableWhenNoRungNamedOne(t *testing.T) {
	root := ladderProject(t, ledgerWithoutDefects)
	text := ladderReport(t, root)
	if strings.Contains(text, "DEFECT") {
		t.Errorf("RED: the text report prints a defect table with no defect:\n%s", text)
	}
	if !strings.Contains(text, "TOKENS") {
		t.Errorf("RED: the text report lost its main table:\n%s", text)
	}
	if json := ladderReport(t, root, "--json"); strings.Contains(json, "defects") {
		t.Errorf("RED: the json report prints defects with no defect: %s", json)
	}
}
