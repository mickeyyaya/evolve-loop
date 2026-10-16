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
		if i%2 == 0 {
			rc = 1
		}
		wantRC[name] = rc
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
		if !strings.Contains(resultsStr, fmt.Sprintf("%s\t%d\t", name, want)) {
			t.Errorf("results missing/incorrect for %s (want rc=%d):\n%s", name, want, resultsStr)
		}
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
