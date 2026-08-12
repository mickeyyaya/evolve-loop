package main

// cmd_salvage_test.go — cycle-1442 audit H2: `evolve salvage report` shipped as
// a new operator-facing surface at 0.0% coverage (`runSalvage 0.0%`,
// `runSalvageReport 0.0%`, no test in this package referencing either symbol).
// The number it prints is the one an operator reads as "the gate coerced N
// verdicts", so an untested renderer is an untested claim about the gate.
//
// Driven through the real entry points with a real on-disk sidecar pair — no
// seams stubbed — because the defect class here is exactly a renderer that
// diverges from what the gate writes.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// salvageCLIFixture writes a project root whose .evolve holds a baseline
// sidecar (2 records, 1 recoverable) and an applied sidecar (1 salvage, plus
// one crash-torn line).
func salvageCLIFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolve, 0o755); err != nil {
		t.Fatal(err)
	}
	baseline := strings.Join([]string{
		`{"event_type":"bad_verdict_classified","phase":"audit","recoverable":true,"pattern":"fenced-json","reason":"r"}`,
		`{"event_type":"bad_verdict_classified","phase":"build","recoverable":false,"pattern":"","reason":"r"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(evolve, deliverable.BadVerdictBaselineFile), []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}
	applied := `{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json"}` + "\n" +
		`{"event_type":"salvage_app` + "\n" // torn by a crash mid-append
	if err := os.WriteFile(filepath.Join(evolve, deliverable.SalvageAppliedFile), []byte(applied), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunSalvageReport_ProseRendersCountsAndNamesSkippedRecords(t *testing.T) {
	root := salvageCLIFixture(t)
	var out, errBuf strings.Builder

	if rc := runSalvageReport([]string{"-project-root", root}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc = %d, want 0 — one torn line must not brick the whole report (cycle-1442 M2); stderr=%q", rc, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{
		"2 bad_verdict deliverable(s) classified, 1 recoverable, 1 actually salvaged",
		"recoverable-malformed rate: 0.500 (50.0%)",
		"fenced-json",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prose missing %q\n---\n%s", want, got)
		}
	}
	// Tolerance must stay honest: the skipped record is named, not dropped.
	if !strings.Contains(got, "unreadable record(s) skipped") {
		t.Errorf("a skipped sidecar record must be surfaced — silent tolerance is how a forged torn line hides salvages\n---\n%s", got)
	}
}

func TestRunSalvageReport_JSONEnvelopeMatchesProseNumbers(t *testing.T) {
	root := salvageCLIFixture(t)
	var out, errBuf strings.Builder

	if rc := runSalvageReport([]string{"-json", "-project-root", root}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr=%q", rc, errBuf.String())
	}
	var env struct {
		Total       int     `json:"total"`
		Recoverable int     `json:"recoverable"`
		Rate        float64 `json:"rate"`
		Saved       int     `json:"saved"`
		Malformed   int     `json:"malformed"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("-json must emit a decodable envelope: %v\n%s", err, out.String())
	}
	if env.Total != 2 || env.Recoverable != 1 || env.Saved != 1 || env.Rate != 0.5 {
		t.Errorf("envelope = %+v, want total=2 recoverable=1 saved=1 rate=0.5 (same numbers the prose renders)", env)
	}
	// The machine-readable consumer is precisely the one that cannot see the
	// prose WARN, so tolerance that is honest only in prose is silent exactly
	// where it is parsed (diff-review MEDIUM).
	if env.Malformed != 1 {
		t.Errorf("malformed = %d, want 1 — the JSON envelope must carry the skipped-record count, not just the prose", env.Malformed)
	}
}

// A never-salvaged project is the NORMAL state: absent sidecars report zero
// through the same envelope rather than erroring.
func TestRunSalvageReport_AbsentSidecarsAreZeroNotError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errBuf strings.Builder
	if rc := runSalvageReport([]string{"-project-root", root}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc = %d, want 0 for a project that never salvaged; stderr=%q", rc, errBuf.String())
	}
	if !strings.Contains(out.String(), "no bad_verdict deliverables classified yet") {
		t.Errorf("empty state must render its own line, got %q", out.String())
	}
}

// The dispatcher's own arms: usage on no subcommand, named refusal on an
// unknown one. Both return 10 (the repo's usage-error code), never 0 — a
// mistyped subcommand that exits success reads as "nothing to report".
func TestRunSalvage_UsageAndUnknownSubcommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no subcommand", nil, "usage: salvage report"},
		{"unknown subcommand", []string{"summarise"}, `unknown subcommand "summarise"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf strings.Builder
			if rc := runSalvage(tc.args, nil, &out, &errBuf); rc != 10 {
				t.Errorf("rc = %d, want 10", rc)
			}
			if !strings.Contains(errBuf.String(), tc.want) {
				t.Errorf("stderr missing %q, got %q", tc.want, errBuf.String())
			}
		})
	}
}
