package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/citelemetry"
)

type telemetryFailingWriter struct{}

func (telemetryFailingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func telemetryJobsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jobs.json")
	body := `{"jobs":[{"name":"build (macOS)","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:10:00Z","steps":[{"name":"test","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:05:00Z"}]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTelemetry(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runCI(append([]string{"telemetry"}, args...), nil, &out, &errb)
	return out.String(), errb.String(), code
}

func TestRunCITelemetry_PrintsTheMarkdownReportAndTheJSONReport(t *testing.T) {
	t.Parallel()
	jobs := telemetryJobsFile(t)
	out, errOut, code := runTelemetry("--jobs", jobs)
	if code != 0 || !strings.Contains(out, "| L5 | build (macOS) | 600 |") || !strings.Contains(out, "macOS job: 600 s") {
		t.Errorf("markdown: code=%d out=%q err=%q", code, out, errOut)
	}
	out, errOut, code = runTelemetry("--jobs", jobs, "--json")
	if code != 0 || !strings.Contains(out, `"macos_job_s": 600`) {
		t.Errorf("json: code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestRunCITelemetry_FailsLoudlyAndPrintsNoReport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"no jobs flag", nil, exitUsage},
		{"an operand", []string{"--jobs", telemetryJobsFile(t), "extra"}, exitUsage},
		{"unknown flag", []string{"--nope"}, exitUsage},
		{"missing jobs file", []string{"--jobs", filepath.Join(t.TempDir(), "absent.json")}, exitIO},
	}
	for _, c := range cases {
		out, errOut, code := runTelemetry(c.args...)
		if code != c.code || out != "" || !strings.Contains(errOut, ciTelemetryPrefix) {
			t.Errorf("%s: code=%d out=%q err=%q, want %d, no report and the prefix", c.name, code, out, errOut, c.code)
		}
	}
}

func TestRunCITelemetry_ReportsAWriteFailureInBothFormats(t *testing.T) {
	t.Parallel()
	for _, asJSON := range []bool{false, true} {
		if err := writeTelemetryReport(telemetryFailingWriter{}, citelemetry.Report{}, asJSON); err == nil {
			t.Errorf("asJSON=%v: no error from a failing writer", asJSON)
		}
	}
	var errb bytes.Buffer
	code := runCITelemetry([]string{"--jobs", telemetryJobsFile(t)}, telemetryFailingWriter{}, &errb)
	if code != exitIO || !strings.Contains(errb.String(), "disk full") {
		t.Errorf("code=%d err=%q, want exitIO naming the write failure", code, errb.String())
	}
}

func TestRunCI_UsageNamesTheTelemetryVerb(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runCI(nil, nil, &out, &errb); code != exitUsage || !strings.Contains(errb.String(), "evolve ci telemetry") {
		t.Errorf("code=%d err=%q", code, errb.String())
	}
}
