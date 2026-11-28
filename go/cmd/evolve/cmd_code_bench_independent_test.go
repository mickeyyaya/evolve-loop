package main

import (
	"strconv"
	"strings"
	"testing"
)

type benchTableRow struct {
	set, backend, query string
	precision, recall   float64
	p50, p95            string
}

func parseBenchTable(t *testing.T, stdout string) []benchTableRow {
	t.Helper()
	var rows []benchTableRow
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[2] != "callers" {
			continue
		}
		precision, perr := strconv.ParseFloat(fields[3], 64)
		recall, rerr := strconv.ParseFloat(fields[4], 64)
		if perr != nil || rerr != nil {
			t.Fatalf("row %q: precision and recall must be numbers: %v %v", line, perr, rerr)
		}
		rows = append(rows, benchTableRow{set: fields[0], backend: fields[1], query: fields[2], precision: precision, recall: recall, p50: fields[5], p95: fields[6]})
	}
	return rows
}

func findBenchRow(rows []benchTableRow, set, backend string) (benchTableRow, bool) {
	for _, row := range rows {
		if row.set == set && row.backend == backend {
			return row, true
		}
	}
	return benchTableRow{}, false
}

func TestCodeBench_IndependentTruth_EveryBackendIsScoredOnBothTruthSets(t *testing.T) {
	t.Parallel()
	root, _ := benchProject(t, benchLibSource)

	rc, stdout, stderr := runBenchVerb(t, benchHost("{ 2.50 1.00 1.00 }"), "--backends", "gotypes,trail", "--project-root", root, "--symbols", "5")

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q, want 0", rc, stderr)
	}
	rows := parseBenchTable(t, stdout)
	for _, set := range []string{"go-types", "independent"} {
		for _, backend := range []string{"gotypes", "trail"} {
			if _, ok := findBenchRow(rows, set, backend); !ok {
				t.Errorf("no %s row for backend %s in\n%s", set, backend, stdout)
			}
		}
	}
	if len(rows) != 4 {
		t.Errorf("rows = %d, want 4 (two backends on two sets)\n%s", len(rows), stdout)
	}
}

func TestCodeBench_IndependentTruth_TheHeaderNamesTheSetAndThePercentiles(t *testing.T) {
	t.Parallel()
	root, _ := benchProject(t, benchLibSource)

	_, stdout, _ := runBenchVerb(t, benchHost(""), "--backends", "gotypes", "--project-root", root)

	header := strings.Fields(strings.SplitN(stdout, "\n", 2)[0])
	want := []string{"set", "backend", "query", "precision", "recall", "p50", "p95"}
	if len(header) < len(want) {
		t.Fatalf("header = %v, want it to start with %v", header, want)
	}
	for i, column := range want {
		if header[i] != column {
			t.Errorf("header column %d = %q, want %q (header %v)", i, header[i], column, header)
		}
	}
}

func TestCodeBench_IndependentTruth_GotypesIsNotPerfectOnTheSetItDidNotProduce(t *testing.T) {
	t.Parallel()
	root, _ := benchProject(t, benchLibSource)

	_, stdout, stderr := runBenchVerb(t, benchHost(""), "--backends", "gotypes", "--project-root", root)

	rows := parseBenchTable(t, stdout)
	derived, okDerived := findBenchRow(rows, "go-types", "gotypes")
	independent, okIndependent := findBenchRow(rows, "independent", "gotypes")
	if !okDerived || !okIndependent {
		t.Fatalf("want a gotypes row on each set, got\n%s\nstderr=%q", stdout, stderr)
	}
	if derived.recall != 1 {
		t.Errorf("go-types recall = %v, want 1.00: the derived set is the index's own view", derived.recall)
	}
	if independent.recall <= 0 || independent.recall >= 1 {
		t.Errorf("independent recall = %v, want 0 < recall < 1: gotypes resolves the static calls and misses the calls through function values", independent.recall)
	}
	if independent.p50 == "" || independent.p95 == "" {
		t.Errorf("independent row has no p50/p95: %+v", independent)
	}
}

func TestCodeBench_IndependentTruth_AnAbsentIndexStillFailsBeforeAnySetIsPrinted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"index_dir":"`+root+`/none","trail_bin":"/nonexistent/trail"}}`)

	rc, stdout, stderr := runBenchVerb(t, benchHost(""), "--project-root", root)

	if rc != codeExitAbsent || stdout != "" || !strings.Contains(stderr, "absent") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want exit %d, no table and the absence", rc, stdout, stderr, codeExitAbsent)
	}
}
