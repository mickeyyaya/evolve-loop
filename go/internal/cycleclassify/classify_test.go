package cycleclassify

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

func writeReport(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return ws
}

// seedEvents writes the envelopes the real phasestream.Classifier emits for raw stdout and stderr lines, as the live normalizer does.
func seedEvents(t *testing.T, ws, name string, stdoutLines, stderrLines []string) {
	t.Helper()
	clf := phasestream.NewClassifier(
		phasestream.Source{Producer: "normalizer", CLI: "claude-p", Cycle: 1, Phase: "memo", Agent: "memo"},
		"classify-parity-trace", nil)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	emit := func(envs []phasestream.Envelope) {
		for _, e := range envs {
			if err := enc.Encode(e); err != nil {
				t.Fatalf("encode envelope: %v", err)
			}
		}
	}
	for _, ln := range stdoutLines {
		emit(clf.Line([]byte(ln)))
	}
	for _, ln := range stderrLines {
		emit(clf.Stderr([]byte(ln)))
	}
	if err := os.WriteFile(filepath.Join(ws, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write events %s: %v", name, err)
	}
}

func TestClassify_NoReport_IntegrityBreach(t *testing.T) {
	t.Parallel()
	r := Classify(t.TempDir())
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach", r.Class)
	}
	if r.Marker != "" || r.Source != "" {
		t.Fatalf("marker/source should be empty for breach; got %+v", r)
	}
}

func TestClassify_EmptyReport_IntegrityBreach(t *testing.T) {
	t.Parallel()
	r := Classify(writeReport(t, ""))
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("empty report → %q want integrity-breach", r.Class)
	}
}

func TestClassify_Infrastructure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"EPERM", "build started\nEPERM: operation not permitted\n"},
		{"rate-limit", "API call: 429 Too Many Requests"},
		{"overload-529", "got 529 Overloaded from anthropic"},
		{"timeout", "ETIMEDOUT after 30s"},
		{"operation-timed-out", "phase exit: operation timed out"},
		{"sandbox-eperm", "sandbox-exec: deny() Operation not permitted"},
		{"explicit-marker", "INFRASTRUCTURE FAILURE: see above"},
		{"connection-refused", "connect: connection refused"},
		{"sandbox-apply", "sandbox_apply: deny not permitted"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, tc.body))
			if r.Class != ClassInfrastructure {
				t.Fatalf("body=%q → %q want infrastructure (marker=%q)", tc.body, r.Class, r.Marker)
			}
			if r.Marker == "" {
				t.Fatalf("expected non-empty marker")
			}
		})
	}
}

func TestClassify_InfraInEventsStdout(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "## Verdict\nNo errors detected")
	seedEvents(t, ws, "memo-events.ndjson", []string{"429 Too Many Requests"}, nil)
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure from events scan; marker=%q source=%q", r.Class, r.Marker, r.Source)
	}
	if r.Source != "memo-events.ndjson" {
		t.Fatalf("source=%q want memo-events.ndjson", r.Source)
	}
	if r.Marker != "api_429" {
		t.Fatalf("marker=%q want api_429", r.Marker)
	}
}

func TestClassify_InfraInEventsStderr(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "OK")
	seedEvents(t, ws, "builder-events.ndjson", nil, []string{"ETIMEDOUT"})
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure", r.Class)
	}
	if r.Source != "builder-events.ndjson" {
		t.Fatalf("source=%q want builder-events.ndjson", r.Source)
	}
	if r.Marker != "timeout" {
		t.Fatalf("marker=%q want timeout", r.Marker)
	}
}

func TestClassify_ParityInfraRawVsEvents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, marker        string
		stdoutLines, stderr []string
	}{
		{"stdout-529", "api_529", []string{"some progress", "API Error 529 Overloaded"}, nil},
		{"stderr-eperm", "eperm", nil, []string{"sandbox-exec: Operation not permitted"}},
		{"stdout-among-prose", "rate_limit", []string{"Working on the task.", "hit a rate limit, backing off"}, nil},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := writeReport(t, "## Verdict\nclean report, no markers here")
			seedEvents(t, ws, "scout-events.ndjson", tc.stdoutLines, tc.stderr)
			r := Classify(ws)
			if r.Class != ClassInfrastructure {
				t.Fatalf("parity: got %q want infrastructure (marker=%q source=%q)", r.Class, r.Marker, r.Source)
			}
			if r.Marker != tc.marker {
				t.Fatalf("parity marker: got %q want %q", r.Marker, tc.marker)
			}
		})
	}
}

func TestClassify_ParityViaProduce(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "## Verdict\nclean report, no markers")
	if err := os.WriteFile(filepath.Join(ws, "memo-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "memo-stderr.log"),
		[]byte("API Error 529 Overloaded\n"), 0o644); err != nil {
		t.Fatalf("write stderr: %v", err)
	}
	if err := phasestream.Produce(phasestream.ProduceConfig{Workspace: ws, Phase: "memo", CLI: "claude-p", Cycle: 7}); err != nil {
		t.Fatalf("Produce: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("parity: got %q want infrastructure (marker=%q source=%q)", r.Class, r.Marker, r.Source)
	}
	if r.Marker != "api_529" {
		t.Fatalf("parity marker: got %q want api_529", r.Marker)
	}
	if r.Source != "memo-events.ndjson" {
		t.Fatalf("parity source: got %q want memo-events.ndjson", r.Source)
	}
}

func TestClassify_ShipGateConfig(t *testing.T) {
	t.Parallel()
	tests := []string{
		"SHIP_GATE_DENIED: see audit report",
		"shipgate rejected commit",
		"ship-gate denied at HEAD",
		"integrity-fail: Auditor exited with 1",
	}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassShipGateConfig {
				t.Fatalf("body=%q → %q want ship-gate-config (marker=%q)", body, r.Class, r.Marker)
			}
		})
	}
}

func TestClassify_ShipGateBeatsAuditFail(t *testing.T) {
	t.Parallel()
	body := `
Verdict: FAIL
But actually SHIP_GATE_DENIED — the audit was PASS originally.
`
	r := Classify(writeReport(t, body))
	if r.Class != ClassShipGateConfig {
		t.Fatalf("got %q want ship-gate-config (ship-gate must beat audit-fail)", r.Class)
	}
}

func TestClassify_AuditFail(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"**Verdict: FAIL** — defects above threshold\n",
		"Verdict: WARN — defects above threshold",
		"verdict: fail",
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassAuditFail {
				t.Fatalf("body=%q → %q want audit-fail", body, r.Class)
			}
		})
	}
}

func TestClassify_BuildFail(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"Build status: FAIL — 3/12 tests RED",
		"tests RED across the board",
		"builder failed after 3 retries",
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassBuildFail {
				t.Fatalf("body=%q → %q want build-fail", body, r.Class)
			}
		})
	}
}

func TestClassify_InfraBeatsEverything(t *testing.T) {
	t.Parallel()
	body := `
Verdict: FAIL
Build status: FAIL — tests RED
SHIP_GATE_DENIED
EPERM: sandbox blocked write
`
	r := Classify(writeReport(t, body))
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure (highest priority)", r.Class)
	}
}

func TestClassify_UnclassifiableReport_Breach(t *testing.T) {
	t.Parallel()
	body := "Cycle completed. Nothing surprising to report.\nVerdict: SHIPPED"
	r := Classify(writeReport(t, body))
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (no markers)", r.Class)
	}
}

func TestClassify_SortedEventsScan(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "OK")
	seedEvents(t, ws, "zeta-events.ndjson", nil, []string{"EPERM"})
	seedEvents(t, ws, "alpha-events.ndjson", nil, []string{"EPERM"})
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure", r.Class)
	}
	if r.Source != "alpha-events.ndjson" {
		t.Fatalf("source=%q want alpha-events.ndjson (sorted scan)", r.Source)
	}
}

func TestClassify_Pass0_StructuredClassBeatsRegex(t *testing.T) {
	// Prose alone would regex-classify audit-fail.
	ws := writeReport(t, "# Cycle Report\nVerdict: FAIL\n")
	line := phasecontract.RenderVerdictSentinelWithFailure("tdd", "FAIL",
		&phasecontract.FailureBlock{Class: "code-build-fail", Defects: []string{"red suite"}})
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte("## Tests\nFAIL\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Classify(ws)
	if got.Class != Classification("code-build-fail") {
		t.Errorf("Class = %q, want code-build-fail (structured sentinel, normalized)", got.Class)
	}
	if got.Source != "test-report.md" {
		t.Errorf("Source = %q, want test-report.md", got.Source)
	}
}

func TestClassify_Pass0_UnknownClassFallsThroughToRegex(t *testing.T) {
	ws := writeReport(t, "# Cycle Report\nVerdict: FAIL\n")
	line := phasecontract.RenderVerdictSentinelWithFailure("tdd", "FAIL",
		&phasecontract.FailureBlock{Class: "totally-novel-class"})
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte("## Tests\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Classify(ws)
	if got.Class != ClassAuditFail {
		t.Errorf("Class = %q, want audit-fail (regex fallback; unknown structured class is not trusted)", got.Class)
	}
}

func TestClassify_Pass0_PassSentinelInert(t *testing.T) {
	ws := writeReport(t, "# Cycle Report\nVerdict: FAIL\n")
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"),
		[]byte("## Findings\n"+phasecontract.RenderVerdictSentinel("scout", "PASS")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Classify(ws); got.Class != ClassAuditFail {
		t.Errorf("Class = %q, want audit-fail (PASS sentinel must be inert)", got.Class)
	}
}

func TestClassify_Pass0_WarnDoesNotSuppressInfra(t *testing.T) {
	ws := writeReport(t, "# Cycle Report\nsandbox-exec: Operation not permitted\n")
	line := phasecontract.RenderVerdictSentinelWithFailure("audit", "WARN",
		&phasecontract.FailureBlock{Class: "code-audit-warn", Defects: []string{"w1"}})
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\nWARN\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Classify(ws); got.Class != ClassInfrastructure {
		t.Errorf("Class = %q, want infrastructure (WARN sentinel must not suppress the infra pass)", got.Class)
	}
}

func TestClassify_Pass0_SkipsRetrospectiveReport(t *testing.T) {
	ws := writeReport(t, "# Cycle Report\nVerdict: FAIL\n")
	line := phasecontract.RenderVerdictSentinelWithFailure("retrospective", "FAIL",
		&phasecontract.FailureBlock{Class: "infrastructure-transient"})
	if err := os.WriteFile(filepath.Join(ws, "retrospective-report.md"), []byte("# Retro\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Classify(ws); got.Class != ClassAuditFail {
		t.Errorf("Class = %q, want audit-fail (retrospective report excluded from Pass 0)", got.Class)
	}
}
