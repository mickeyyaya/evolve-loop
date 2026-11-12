package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func metricsFinding(id string, sev convergence.Severity, status convergence.Status, falsification convergence.Falsification) convergence.Finding {
	return convergence.Finding{ID: id, Severity: sev, Status: status, Late: falsification != "", Falsification: falsification}
}

func writeRounds(t *testing.T, name string, loop convergence.Loop, rounds ...[]convergence.Finding) string {
	t.Helper()
	in := convergence.Input{Loop: loop, Round: len(rounds) - 1}
	for i, findings := range rounds {
		in.Rounds = append(in.Rounds, convergence.Judgment{Index: i, Findings: findings})
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func metricsFixtures(t *testing.T) (string, string) {
	t.Helper()
	open, deferred, fixedS, filed := convergence.StatusOpen, convergence.StatusDeferred, convergence.StatusFixed, convergence.StatusFiled
	med, high := convergence.SeverityMedium, convergence.SeverityHigh
	survived, refuted := convergence.FalsificationSurvived, convergence.FalsificationRefuted
	three := writeRounds(t, "three.json", convergence.LoopConsoleLane,
		[]convergence.Finding{metricsFinding("d1", med, open, ""), metricsFinding("d2", med, open, ""), metricsFinding("h1", high, open, "")},
		[]convergence.Finding{metricsFinding("d1", med, deferred, ""), metricsFinding("d2", med, deferred, ""), metricsFinding("h1", high, open, survived)},
		[]convergence.Finding{metricsFinding("d1", med, fixedS, ""), metricsFinding("d2", med, deferred, ""), metricsFinding("h1", high, fixedS, survived)},
	)
	two := writeRounds(t, "two.json", convergence.LoopConsoleLane,
		[]convergence.Finding{metricsFinding("x", high, open, ""), metricsFinding("y", high, open, ""), metricsFinding("z", high, open, "")},
		[]convergence.Finding{metricsFinding("x", high, filed, refuted), metricsFinding("y", high, filed, refuted), metricsFinding("z", high, open, survived)},
	)
	return three, two
}

func runMetrics(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	rc := runConvergence(append([]string{"metrics"}, args...), nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestCmd_ConvergenceMetrics_TextPrintsTheThreeReadouts(t *testing.T) {
	convergenceRoot(t, "")
	three, two := metricsFixtures(t)

	rc, stdout, stderr := runMetrics(three, two)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	want := "rounds to land:\n" +
		"  console-lane: median 1.5, max 2, runs 2\n" +
		"deferred then closed: 1 of 2 (0.50)\n" +
		"falsification drop rate: 2 of 4 (0.50)\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestCmd_ConvergenceMetrics_JSONCarriesTheSameNumbers(t *testing.T) {
	convergenceRoot(t, "")
	three, two := metricsFixtures(t)

	rc, stdout, stderr := runMetrics("--json", three, two)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	var got convergence.Metrics
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if len(got.RoundsToLand) != 1 || got.RoundsToLand[0] != (convergence.LoopRounds{Loop: convergence.LoopConsoleLane, Runs: 2, Median: 1.5, Max: 2}) {
		t.Errorf("RoundsToLand = %+v", got.RoundsToLand)
	}
	if got.DeferredClosed != (convergence.DeferredClosed{Deferred: 2, Closed: 1, Share: 0.5}) {
		t.Errorf("DeferredClosed = %+v", got.DeferredClosed)
	}
	if got.Falsification != (convergence.FalsificationDrop{Checked: 4, Refuted: 2, Rate: 0.5}) {
		t.Errorf("Falsification = %+v", got.Falsification)
	}
}

func TestCmd_ConvergenceMetrics_ReadsNoPolicy(t *testing.T) {
	convergenceRoot(t, `{not json`)
	three, _ := metricsFixtures(t)

	rc, _, stderr := runMetrics(three)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q: metrics must not load the policy", rc, stderr)
	}
}

func TestCmd_ConvergenceMetrics_EmptyRoundsStillPrintZeroRates(t *testing.T) {
	convergenceRoot(t, "")
	only := writeRounds(t, "only.json", convergence.LoopCodeReview, []convergence.Finding{})

	rc, stdout, stderr := runMetrics(only)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	for _, want := range []string{
		"  code-review: median 0, max 0, runs 1\n",
		"deferred then closed: 0 of 0 (0.00)\n",
		"falsification drop rate: 0 of 0 (0.00)\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestCmd_ConvergenceMetrics_RejectsBadInvocations(t *testing.T) {
	convergenceRoot(t, "")
	three, _ := metricsFixtures(t)
	malformed := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(malformed, []byte(`{"loop":`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "absent.json")
	cases := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"no file", nil, "usage"},
		{"unknown flag", []string{"--bogus", three}, "flag provided but not defined"},
		{"unreadable file", []string{missing}, missing},
		{"malformed file", []string{malformed}, malformed},
		{"one bad file among good ones", []string{three, missing}, missing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, stdout, stderr := runMetrics(c.args...)
			if rc != exitConvergenceBad {
				t.Fatalf("rc = %d, want %d (stderr %q)", rc, exitConvergenceBad, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing printed", stdout)
			}
			if !strings.Contains(stderr, c.stderr) {
				t.Errorf("stderr %q lacks %q", stderr, c.stderr)
			}
		})
	}
}

func TestCmd_ConvergenceMetrics_UsageNamesBothVerbs(t *testing.T) {
	convergenceRoot(t, "")
	var stdout, stderr bytes.Buffer

	rc := runConvergence(nil, nil, &stdout, &stderr)

	if rc != exitConvergenceBad || !strings.Contains(stderr.String(), "decide") || !strings.Contains(stderr.String(), "metrics") {
		t.Fatalf("rc = %d stderr = %q, want the usage to name decide and metrics", rc, stderr.String())
	}
}

func TestCmd_ConvergenceMetrics_JSONEncodeFailureIsReported(t *testing.T) {
	var stderr bytes.Buffer

	rc := encodeConvergenceMetrics(convergence.Metrics{}, &failingWriter{}, &stderr)

	if rc != exitConvergenceWrite || !strings.Contains(stderr.String(), "encode") {
		t.Fatalf("rc = %d stderr = %q, want exit %d and an encode error", rc, stderr.String(), exitConvergenceWrite)
	}
}
