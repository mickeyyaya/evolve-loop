package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const ladderCycleOneLedger = `{"phase":"build","rung":"R0","exit_code":1,"tokens":{"input":10,"output":5,"cache_read":0,"cache_write":1}}
{"phase":"scout","rung":"R0","exit_code":0,"tokens":{"input":1,"output":1,"cache_read":1,"cache_write":0}}
{"phase":"build","rung":"R1","failure_class":"capability","exit_code":0,"tokens":{"input":20,"output":10,"cache_read":100,"cache_write":0}}
`

const ladderCycleTwoLedger = `{"phase":"build","rung":"R0","exit_code":1,"tokens":{"input":3,"output":3,"cache_read":0,"cache_write":0}}
{"phase":"audit","rung":"R0","exit_code":0,"tokens":{"input":2,"output":0,"cache_read":0,"cache_write":0}}
{"phase":"build","rung":"R1","failure_class":"capability","exit_code":1,"tokens":{"input":4,"output":4,"cache_read":0,"cache_write":0}}
{"phase":"build","rung":"R2","failure_class":"capability","exit_code":0,"tokens":{"input":5,"output":5,"cache_read":5,"cache_write":5}}
{"phase":"tdd","rung":"R0","exit_code":85,"tokens":{"input":1,"output":0,"cache_read":0,"cache_write":0}}
{"phase":"tdd","rung":"R1","failure_class":"quota","exit_code":0,"tokens":{"input":7,"output":7,"cache_read":7,"cache_write":7}}
not a json line
`

func ladderProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runs := filepath.Join(root, ".evolve", "runs")
	for name, ledger := range map[string]string{"cycle-1": ladderCycleOneLedger, "cycle-2": ladderCycleTwoLedger} {
		dir := filepath.Join(runs, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "llm-calls.ndjson"), []byte(ledger), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scout-report.md"), []byte("untouched "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(runs, "cycle-3"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func runLadder(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	cmd := lookupCommand("ladder")
	if cmd == nil {
		t.Fatal("ladder is not in the CLI registry")
	}
	var stdout, stderr bytes.Buffer
	rc := cmd.Run(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func ladderTreeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		out[rel] = hex.EncodeToString(sum[:]) + "@" + info.ModTime().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLadderReport_VerbPrintsExactCountsPerRungAndClass(t *testing.T) {
	t.Parallel()
	root := ladderProject(t)
	rc, stdout, stderr := runLadder(t, "report", "--project-root", root)
	if rc != 0 {
		t.Fatalf("rc = %d, stderr = %q", rc, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	want := [][]string{
		{"RUNG", "CLASS", "TRIES", "FIXES", "TOKENS"},
		{"R0", "-", "5", "0", "28"},
		{"R1", "capability", "2", "1", "138"},
		{"R1", "quota", "1", "1", "28"},
		{"R2", "capability", "1", "1", "20"},
	}
	if len(lines) != len(want) {
		t.Fatalf("report has %d lines, want %d:\n%s", len(lines), len(want), stdout)
	}
	for i, line := range lines {
		if got := strings.Fields(line); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("line %d = %v, want %v", i, got, want[i])
		}
	}
}

type ladderJSONRow struct {
	Rung   string `json:"rung"`
	Class  string `json:"class"`
	Tries  int    `json:"tries"`
	Fixes  int    `json:"fixes"`
	Tokens struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cache_read"`
		CacheWrite int `json:"cache_write"`
	} `json:"tokens"`
}

type ladderJSONReport struct {
	Runs int             `json:"runs"`
	Rows []ladderJSONRow `json:"rows"`
}

func TestLadderReport_VerbJSONCarriesTheSameCounts(t *testing.T) {
	t.Parallel()
	root := ladderProject(t)
	rc, stdout, stderr := runLadder(t, "report", "--json", "--project-root", root)
	if rc != 0 {
		t.Fatalf("rc = %d, stderr = %q", rc, stderr)
	}
	var got ladderJSONReport
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if got.Runs != 2 {
		t.Errorf("runs = %d, want 2 (cycle-3 has no ledger)", got.Runs)
	}
	type key struct {
		rung, class     string
		tries, fixes    int
		in, out, cr, cw int
	}
	var have []key
	for _, r := range got.Rows {
		have = append(have, key{r.Rung, r.Class, r.Tries, r.Fixes, r.Tokens.Input, r.Tokens.Output, r.Tokens.CacheRead, r.Tokens.CacheWrite})
	}
	want := []key{
		{"R0", "", 5, 0, 17, 9, 1, 1},
		{"R1", "capability", 2, 1, 24, 14, 100, 0},
		{"R1", "quota", 1, 1, 7, 7, 7, 7},
		{"R2", "capability", 1, 1, 5, 5, 5, 5},
	}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("json rows\n got  %+v\n want %+v", have, want)
	}
}

func TestLadderReport_VerbWritesNothing(t *testing.T) {
	t.Parallel()
	root := ladderProject(t)
	before := ladderTreeDigest(t, root)
	for _, args := range [][]string{
		{"report", "--project-root", root},
		{"report", "--json", "--project-root", root},
		{"report", "--bogus", "--project-root", root},
		{"report", "--project-root", filepath.Join(root, "absent")},
	} {
		runLadder(t, args...)
	}
	after := ladderTreeDigest(t, root)
	if !reflect.DeepEqual(before, after) {
		var changed []string
		for k, v := range after {
			if before[k] != v {
				changed = append(changed, k)
			}
		}
		for k := range before {
			if _, ok := after[k]; !ok {
				changed = append(changed, k)
			}
		}
		sort.Strings(changed)
		t.Fatalf("the verb changed the project tree: %v", changed)
	}
}

func TestLadderReport_VerbEmptyRunsDirPrintsOnlyTheHeader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, stdout, stderr := runLadder(t, "report", "--project-root", root)
	if rc != 0 {
		t.Fatalf("rc = %d, stderr = %q", rc, stderr)
	}
	if lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "RUNG") {
		t.Fatalf("stdout = %q, want the header only", stdout)
	}
	rc, stdout, _ = runLadder(t, "report", "--json", "--project-root", root)
	var got map[string]json.RawMessage
	if rc != 0 || json.Unmarshal([]byte(stdout), &got) != nil {
		t.Fatalf("rc = %d, stdout = %q", rc, stdout)
	}
	if string(got["rows"]) != "[]" || string(got["runs"]) != "0" {
		t.Fatalf("empty json = %s, want rows [] and runs 0", stdout)
	}
}

func TestLadderReport_VerbExitCodes(t *testing.T) {
	t.Parallel()
	root := ladderProject(t)
	cases := []struct {
		name   string
		args   []string
		wantRC int
		inErr  string
	}{
		{"absent runs dir is an input error", []string{"report", "--project-root", filepath.Join(t.TempDir(), "nope")}, 2, "runs"},
		{"no subcommand is a usage error", nil, 10, "Usage"},
		{"unknown subcommand is a usage error", []string{"zap"}, 10, "Usage"},
		{"unknown flag is a usage error", []string{"report", "--bogus", "--project-root", root}, 10, "Usage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc, stdout, stderr := runLadder(t, tc.args...)
			if rc != tc.wantRC {
				t.Fatalf("rc = %d, want %d (stdout %q, stderr %q)", rc, tc.wantRC, stdout, stderr)
			}
			if !strings.Contains(stderr, tc.inErr) {
				t.Errorf("stderr %q lacks %q", stderr, tc.inErr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("a failing run must print nothing on stdout, got %q", stdout)
			}
		})
	}
}

func TestLadderReport_VerbHelpExitsZero(t *testing.T) {
	t.Parallel()
	rc, stdout, _ := runLadder(t, "--help")
	if rc != 0 || !strings.Contains(stdout, "ladder report") {
		t.Fatalf("rc = %d, stdout = %q", rc, stdout)
	}
}

func TestLadderReport_RegistrySummaryNamesTheVerbAndItsFlags(t *testing.T) {
	t.Parallel()
	cmd := lookupCommand("ladder")
	if cmd == nil {
		t.Fatal("ladder is not in the CLI registry")
	}
	for _, token := range []string{"report", "--json", "--project-root"} {
		if !strings.Contains(cmd.Summary, token) {
			t.Errorf("registry summary %q lacks %q", cmd.Summary, token)
		}
	}
}

func TestLadderReport_ReportHelpExitsZero(t *testing.T) {
	t.Parallel()
	rc, stdout, _ := runLadder(t, "report", "-h")
	if rc != 0 || !strings.Contains(stdout, "ladder report") {
		t.Fatalf("rc = %d, stdout = %q", rc, stdout)
	}
}

func TestLadderReport_StrayPositionalArgIsAUsageError(t *testing.T) {
	t.Parallel()
	rc, stdout, stderr := runLadder(t, "report", "extra", "--project-root", ladderProject(t))
	if rc != 10 || stdout != "" || !strings.Contains(stderr, "Usage") {
		t.Fatalf("rc = %d, stdout = %q, stderr = %q", rc, stdout, stderr)
	}
}

func TestLadderReport_UnreadableWorkingDirIsAnInputError(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stdout, stderr bytes.Buffer
	getwd := func() (string, error) { return "", errors.New("gone") }
	rc := runLadderReport(nil, getwd, &stdout, &stderr)
	if rc != exitLadderInput || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--project-root") {
		t.Fatalf("rc = %d, stdout = %q, stderr = %q", rc, stdout.String(), stderr.String())
	}
}

func TestLadderReport_JSONWriteFailureFailsLoudly(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	rc := writeLadderJSON(&failingWriter{}, &stderr, ladderReportJSON{})
	if rc != 1 || !strings.Contains(stderr.String(), "broken pipe") {
		t.Fatalf("rc = %d, stderr = %q", rc, stderr.String())
	}
}
