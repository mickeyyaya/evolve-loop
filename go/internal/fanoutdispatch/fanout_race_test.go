package fanoutdispatch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fanout_race_test.go — B5 concurrency-stability coverage for the fan-out
// executor. The existing TestRun_BoundedConcurrency only asserts that all
// workers COMPLETE under a concurrency cap; it never observes that the cap is
// actually honored (it would pass even with a broken semaphore). These tests
// (a) observe the live concurrency bound, and (b) stress isolation +
// partial-failure aggregation at higher N. Run under `-race`.

// TestRun_ConcurrencyBoundIsObserved genuinely verifies the semaphore: each
// worker drops a marker file for the duration of its command, snapshots how
// many markers exist, and records that count. Since a marker's lifetime is a
// subset of the worker's held semaphore slot, no snapshot can exceed the
// configured concurrency — and with N≫concurrency and an overlap sleep, at
// least one snapshot must observe genuine parallelism (guarding against a
// vacuous pass where everything ran serially).
func TestRun_ConcurrencyBoundIsObserved(t *testing.T) {
	t.Parallel()
	const (
		workers     = 8
		concurrency = 3
	)
	dir := t.TempDir()
	active := filepath.Join(dir, "active")
	probe := filepath.Join(dir, "probe")
	for _, d := range []string{active, probe} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	var sb strings.Builder
	for i := 1; i <= workers; i++ {
		name := fmt.Sprintf("w%d", i)
		// touch marker → snapshot active count → hold → remove marker.
		cmd := fmt.Sprintf(
			"touch %s/%s; n=$(ls %s | wc -l | tr -d ' '); echo $n > %s/%s.probe; sleep 0.2; rm -f %s/%s",
			active, name, active, probe, name, active, name)
		fmt.Fprintf(&sb, "%s\t%s\n", name, cmd)
	}
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, sb.String())
	results := filepath.Join(dir, "r.tsv")

	var b bytes.Buffer
	if rc := Run(Config{CommandsFile: cmds, ResultsFile: results, Concurrency: concurrency, TimeoutSecs: 30}, &b); rc != ExitOK {
		t.Fatalf("rc=%d, stderr=%s", rc, b.String())
	}

	maxObserved := 0
	for i := 1; i <= workers; i++ {
		raw, err := os.ReadFile(filepath.Join(probe, fmt.Sprintf("w%d.probe", i)))
		if err != nil {
			t.Fatalf("worker w%d wrote no probe (did it run?): %v", i, err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatalf("worker w%d probe %q not an int: %v", i, raw, err)
		}
		if n > maxObserved {
			maxObserved = n
		}
	}
	if maxObserved > concurrency {
		t.Errorf("observed %d workers running at once, exceeds concurrency cap %d", maxObserved, concurrency)
	}
	if maxObserved < 2 {
		t.Errorf("never observed >1 worker running at once (max=%d) — concurrency not exercised", maxObserved)
	}
}

// TestRun_HighFanoutIsolationPartialFailure stresses the results map, the
// per-worker meta/out files, and the aggregate exit code at higher N than the
// existing N=4 cases: 12 workers at concurrency 4, half failing. Each worker's
// recorded exit code must match its intent (no cross-worker clobbering), Run
// must return non-zero (a worker failed), and every worker must appear once.
func TestRun_HighFanoutIsolationPartialFailure(t *testing.T) {
	t.Parallel()
	const (
		workers     = 12
		concurrency = 4
	)
	dir := t.TempDir()
	wantRC := map[string]int{}
	var sb strings.Builder
	for i := 1; i <= workers; i++ {
		name := fmt.Sprintf("w%d", i)
		rc := 0
		if i%2 == 0 { // even workers fail
			rc = 1
		}
		wantRC[name] = rc
		// Echo a worker-unique line, then exit with the intended code.
		fmt.Fprintf(&sb, "%s\tprintf 'artifact-%s\\n'; exit %d\n", name, name, rc)
	}
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, sb.String())
	results := filepath.Join(dir, "r.tsv")

	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: results, Concurrency: concurrency, TimeoutSecs: 30}, &b)
	if rc != ExitWorkerFail {
		t.Fatalf("rc=%d, want ExitWorkerFail (a worker failed); stderr=%s", rc, b.String())
	}

	body, err := os.ReadFile(results)
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	resultsStr := string(body)
	for name, want := range wantRC {
		// Results TSV rows are "<name>\t<rc>\t<dur>".
		if !strings.Contains(resultsStr, fmt.Sprintf("%s\t%d\t", name, want)) {
			t.Errorf("results missing/incorrect for %s (want rc=%d):\n%s", name, want, resultsStr)
		}
		// Each worker's isolated .out file must carry only its own artifact line.
		out, err := os.ReadFile(filepath.Join(dir, name+".out"))
		if err != nil {
			t.Errorf("worker %s wrote no .out: %v", name, err)
			continue
		}
		if got := strings.TrimSpace(string(out)); got != "artifact-"+name {
			t.Errorf("worker %s .out cross-contaminated: got %q, want %q", name, got, "artifact-"+name)
		}
	}
}
