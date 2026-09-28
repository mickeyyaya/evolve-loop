package aggregator

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeWorkerArtifact(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAggregate_StderrLinesPerFailure(t *testing.T) {
	dir := t.TempDir()
	w := writeWorkerArtifact(t, dir, "w.md", "Verdict: PASS\n")
	empty := writeWorkerArtifact(t, dir, "e.md", "")
	blocker := writeWorkerArtifact(t, dir, "blocker", "x")
	outDir := filepath.Join(dir, "outdir")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "nope.md")
	calls := 0
	flaky := func(p string) ([]byte, error) {
		calls++
		if calls == 1 {
			return os.ReadFile(p)
		}
		return nil, errors.New("read vanished")
	}
	out := filepath.Join(dir, "o.md")
	cases := []struct {
		name string
		in   Inputs
		want string
	}{
		{"empty output", Inputs{Phase: "scout", Workers: []string{w}}, "[aggregator] usage: aggregator <phase> <output> <worker-artifact>...\n"},
		{"no workers", Inputs{Phase: "scout", Output: out}, "[aggregator] error: at least one worker artifact required\n"},
		{"missing", Inputs{Phase: "scout", Output: out, Workers: []string{missing}}, "[aggregator] error: worker artifact not found: " + missing + "\n"},
		{"empty", Inputs{Phase: "scout", Output: out, Workers: []string{empty}}, "[aggregator] error: worker artifact is empty: " + empty + "\n"},
		{"unknown", Inputs{Phase: "weird", Output: out, Workers: []string{w}}, "[aggregator] error: unknown phase 'weird'\n"},
		{"mkdir", Inputs{Phase: "scout", Output: filepath.Join(blocker, "o.md"), Workers: []string{w}}, "[aggregator] error: mkdir " + blocker + ": "},
		{"read", Inputs{Phase: "scout", Output: out, Workers: []string{w}, ReadFile: flaky}, "[aggregator] error: read worker: read vanished\n"},
		{"rename", Inputs{Phase: "scout", Output: outDir, Workers: []string{w}}, "[aggregator] error: write " + outDir + ": "},
	}
	for _, tc := range cases {
		var b bytes.Buffer
		if rc := Aggregate(tc.in, &b); rc != ExitUsageErr {
			t.Errorf("%s: rc=%d", tc.name, rc)
		}
		if !strings.HasPrefix(b.String(), tc.want) {
			t.Errorf("%s: stderr=%q want prefix %q", tc.name, b.String(), tc.want)
		}
	}
	left, err := filepath.Glob(outDir + ".tmp.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestAggregate_StampsUTCTimeAndCreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	w := writeWorkerArtifact(t, dir, "w.md", "body\n")
	out := filepath.Join(dir, "a", "b", "o.md")
	zone := time.FixedZone("X", 8*3600)
	rc := Aggregate(Inputs{Phase: "scout", Output: out, Workers: []string{w}, Now: func() time.Time { return time.Date(2026, 5, 23, 20, 0, 0, 0, zone) }}, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "at 2026-05-23T12:00:00Z.") {
		t.Errorf("body=%s", body)
	}
	out2 := filepath.Join(dir, "o2.md")
	if rc := Aggregate(Inputs{Phase: "scout", Output: out2, Workers: []string{w}}, os.Stderr); rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	body2, _ := os.ReadFile(out2)
	if strings.Contains(string(body2), "0001-01-01") {
		t.Errorf("zero time: %s", body2)
	}
}

func aggregateCrossCLI(t *testing.T, bodies ...string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	var ws []string
	for i, b := range bodies {
		ws = append(ws, writeWorkerArtifact(t, dir, string(rune('a'+i))+".md", b))
	}
	out := filepath.Join(dir, "o.md")
	rc := Aggregate(Inputs{Phase: "cross-cli-vote", Output: out, Workers: ws, Now: fixedNow()}, os.Stderr)
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return rc, string(body)
}

func TestAggregate_CrossCLIVoteGoldenReportWithVeto(t *testing.T) {
	rc, got := aggregateCrossCLI(t, "Verdict: PASS\n", "Verdict: FAIL\n", "nothing\n")
	want := "Verdict: FAIL\n\n" +
		"# Aggregated Cross-CLI Consensus Audit\n\n" +
		"_Aggregated by aggregator.sh at 2026-05-23T12:00:00Z. CLIs voting: 3 (PASS=1, FAIL=1-veto-active=yes, quorum=2)._\n\n" +
		"## Consensus Decision\n\n" +
		"**Verdict**: FAIL\n\n" +
		"**Reason**: cross-cli-vote: at least one CLI returned FAIL (veto rule)\n\n" +
		"**Per-CLI verdicts**: a=PASS, b=FAIL, c=MISSING\n\n" +
		"**Protocol**: MAJORITY-PASS with FAIL-VETO. Any FAIL forces consensus FAIL (defends against false-positive PASS from sycophantic same-vendor agreement). >= quorum PASS with no FAIL → consensus PASS. Otherwise WARN.\n\n" +
		"## Per-CLI Audit Reports\n\n" +
		"### Worker: a\n\nVerdict: PASS\n\n\n" +
		"### Worker: b\n\nVerdict: FAIL\n\n\n" +
		"### Worker: c\n\nnothing\n\n\n"
	if rc != ExitVerdictBad || got != want {
		t.Errorf("rc=%d\n--- got ---\n%s\n--- want ---\n%s", rc, got, want)
	}
}

func TestAggregate_CrossCLIVoteQuorumBoundary(t *testing.T) {
	cases := []struct {
		bodies []string
		want   string
	}{
		{[]string{"Verdict: PASS\n", "Verdict: PASS\n", "Verdict: WARN\n"}, "**Verdict**: PASS\n\n**Reason**: cross-cli-vote: 2 of 3 CLIs returned PASS (quorum=2)\n\n"},
		{[]string{"Verdict: PASS\n", "Verdict: WARN\n"}, "**Verdict**: PASS\n\n**Reason**: cross-cli-vote: 1 of 2 CLIs returned PASS (quorum=1)\n\n"},
		{[]string{"Verdict: PASS\n", "Verdict: WARN\n", "Verdict: WARN\n"}, "**Verdict**: WARN\n\n**Reason**: cross-cli-vote: 1 of 3 PASS (below quorum=2); ships per fluent default unless workflow.strict_audit\n\n"},
	}
	for _, tc := range cases {
		rc, got := aggregateCrossCLI(t, tc.bodies...)
		if rc != ExitOK || !strings.Contains(got, tc.want) || !strings.Contains(got, "FAIL=0-veto-active=no") {
			t.Errorf("rc=%d got:\n%s\nwant substring:\n%s", rc, got, tc.want)
		}
	}
}
