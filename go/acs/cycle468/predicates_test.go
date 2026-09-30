//go:build acs

package cycle468

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fixturePkg     = "github.com/mickeyyaya/evolve-loop/go/acs/cycle466"
	fixtureCycle   = 466
	flakyTest      = "TestC466_004_RepoGatesRaceCleanFullSuite"
	steadyTest     = "TestC466_003_NegativeAndEmptyRejectedSequentialFallback"
	flakyACID      = "cycle466/" + flakyTest
	parseErrorACID = "egps/go-lane-parse-error"

	acssuitePkg = "github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	auditPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

func evTerminal(test, action string) string {
	return `{"Action":"` + action + `","Package":"` + fixturePkg + `","Test":"` + test + `","Elapsed":0.01}`
}

func evOutput(test, body string) string {
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return `{"Action":"output","Package":"` + fixturePkg + `","Test":"` + test + `","Output":` + string(b) + `}`
}

func evStream(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

const raceContentionOutput = "    predicates_test.go:137: full-package -race regression on cmd/evolve, internal/fleet, internal/policy, internal/triagecap is red (exit=1)\n"

type exitErr struct{ code int }

func (e *exitErr) Error() string { return fmt.Sprintf("exit status %d", e.code) }

type seamStep struct {
	raw string
	err error
}

type scriptedSeam struct {
	calls int
	steps []seamStep
}

func (s *scriptedSeam) exec(_ context.Context, _, pattern string, _ []string) (string, error) {
	if !strings.HasPrefix(pattern, "./acs/cycle") {
		return "", nil
	}
	s.calls++
	i := s.calls - 1
	if i >= len(s.steps) {
		i = len(s.steps) - 1
	}
	return s.steps[i].raw, s.steps[i].err
}

func runAndWriteVerdict(t *testing.T, seam *scriptedSeam) (acssuite.Verdict, string, map[string]any) {
	t.Helper()
	v, err := acssuite.Run(acssuite.Options{Root: t.TempDir(), Cycle: fixtureCycle, GoExec: seam.exec})
	if err != nil {
		t.Fatalf("acssuite.Run: %v", err)
	}
	dst, err := acssuite.WriteVerdict(filepath.Join(t.TempDir(), ".evolve"), v)
	if err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("written verdict is not valid JSON: %v", err)
	}
	return v, string(raw), doc
}

func resultByACID(t *testing.T, doc map[string]any, acID string) map[string]any {
	t.Helper()
	results, _ := doc["results"].([]any)
	for _, r := range results {
		m, ok := r.(map[string]any)
		if ok && m["ac_id"] == acID {
			return m
		}
	}
	t.Fatalf("written verdict has no results[] entry with ac_id=%q", acID)
	return nil
}

func warningStrings(doc map[string]any) []string {
	ws, _ := doc["warnings"].([]any)
	var out []string
	for _, w := range ws {
		if s, ok := w.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestC468_001_FlakyRedFlipsGreenWithVisibleAnnotation(t *testing.T) {
	seam := &scriptedSeam{steps: []seamStep{
		{raw: evStream(
			evTerminal(steadyTest, "pass"),
			evOutput(flakyTest, raceContentionOutput),
			evTerminal(flakyTest, "fail"),
		), err: &exitErr{1}},
		{raw: evStream(
			evTerminal(steadyTest, "pass"),
			evTerminal(flakyTest, "pass"),
		)},
	}}
	v, _, doc := runAndWriteVerdict(t, seam)

	if seam.calls != 2 {
		t.Errorf("current-cycle scope invocations = %d, want exactly 2 (one run + one retry)", seam.calls)
	}
	if v.Verdict != "PASS" || !v.ShipEligible || v.RedCount != 0 || v.GreenCount != 2 {
		t.Errorf("verdict=%q ship=%v red=%d green=%d, want PASS/true/0/2 (flake absorbed, gate invariant PASS ⟺ red_count==0 intact)",
			v.Verdict, v.ShipEligible, v.RedCount, v.GreenCount)
	}
	if v.PredicateSuite.Total != 2 {
		t.Errorf("total=%d, want 2 (retry must not duplicate results)", v.PredicateSuite.Total)
	}

	flipped := resultByACID(t, doc, flakyACID)
	if flipped["result"] != "green" || flipped["exit_code"] != float64(0) {
		t.Errorf("flaky result=%v exit_code=%v, want green/0 after passing on retry", flipped["result"], flipped["exit_code"])
	}
	if flipped["flaky"] != "passed-on-retry" {
		t.Errorf(`written flaky result carries flaky=%v, want "passed-on-retry" (the visible annotation is the whole point)`, flipped["flaky"])
	}
	steady := resultByACID(t, doc, "cycle466/"+steadyTest)
	if _, has := steady["flaky"]; has {
		t.Errorf("steady green result carries a flaky annotation %v, want none", steady["flaky"])
	}

	warns := warningStrings(doc)
	if len(warns) != 1 {
		t.Errorf("written verdict warnings=%v, want exactly 1 naming the flaky test", warns)
	} else if !strings.Contains(warns[0], flakyACID) || !strings.Contains(warns[0], "passed-on-retry") {
		t.Errorf("warning %q must name the flaky predicate %q and say passed-on-retry", warns[0], flakyACID)
	}
}

func TestC468_002_DeterministicRedStaysRedRetryBoundedToOne(t *testing.T) {
	firstRun := evStream(
		evOutput(flakyTest, "first-run evidence: "+raceContentionOutput),
		evTerminal(flakyTest, "fail"),
	)
	secondRun := evStream(
		evOutput(flakyTest, "second-run evidence: still red\n"),
		evTerminal(flakyTest, "fail"),
	)
	seam := &scriptedSeam{steps: []seamStep{
		{raw: firstRun, err: &exitErr{1}},
		{raw: secondRun, err: &exitErr{1}},
	}}
	v, raw, doc := runAndWriteVerdict(t, seam)

	if seam.calls != 2 {
		t.Errorf("current-cycle scope invocations = %d, want exactly 2 (retry is bounded to ONCE; a 3rd invocation is unbounded retry)", seam.calls)
	}
	if v.Verdict != "FAIL" || v.ShipEligible || v.RedCount != 1 || v.PredicateSuite.Total != 1 {
		t.Errorf("verdict=%q ship=%v red=%d total=%d, want FAIL/false/1/1 (a deterministic red must still block ship)",
			v.Verdict, v.ShipEligible, v.RedCount, v.PredicateSuite.Total)
	}
	if len(v.RedIDs) != 1 || v.RedIDs[0] != flakyACID {
		t.Errorf("red_ids=%v, want [%s]", v.RedIDs, flakyACID)
	}

	red := resultByACID(t, doc, flakyACID)
	if red["result"] != "red" {
		t.Errorf("result=%v, want red", red["result"])
	}
	if _, has := red["flaky"]; has {
		t.Errorf("deterministic red carries a flaky annotation %v — the annotation must mean passed-on-retry, nothing else", red["flaky"])
	}
	ev, _ := red["evidence_excerpt"].(string)
	if !strings.Contains(ev, "first-run evidence") {
		t.Errorf("evidence_excerpt=%q, want the FIRST run's evidence kept for a red that stays red", ev)
	}
	if strings.Contains(raw, `"warnings"`) {
		t.Errorf("written verdict carries a warnings key on a no-flake FAIL:\n%s", raw)
	}
}

func TestC468_003_SyntheticParseErrorRedNotRetried(t *testing.T) {
	huge := `{"Action":"output","Package":"` + fixturePkg + `","Test":"` + flakyTest + `","Output":"` +
		strings.Repeat("x", 2*1024*1024) + `"}`
	seam := &scriptedSeam{steps: []seamStep{
		{raw: huge + "\n", err: &exitErr{1}},
	}}
	v, raw, _ := runAndWriteVerdict(t, seam)

	if seam.calls != 1 {
		t.Errorf("current-cycle scope invocations = %d, want exactly 1 (a parse-error RED is not a test failure and must never be retried)", seam.calls)
	}
	if v.Verdict != "FAIL" || v.RedCount != 1 || v.ShipEligible {
		t.Errorf("verdict=%q red=%d ship=%v, want FAIL/1/false (partial parse must block loudly)", v.Verdict, v.RedCount, v.ShipEligible)
	}
	if len(v.RedIDs) != 1 || v.RedIDs[0] != parseErrorACID {
		t.Errorf("red_ids=%v, want [%s]", v.RedIDs, parseErrorACID)
	}
	if strings.Contains(raw, `"flaky"`) || strings.Contains(raw, `"warnings"`) {
		t.Errorf("written verdict carries flaky/warnings keys for a non-retryable parse-error RED:\n%s", excerptFor(raw))
	}
}

func TestC468_004_NoFlakeDegradePathByteIdentical(t *testing.T) {
	seam := &scriptedSeam{steps: []seamStep{
		{raw: evStream(evTerminal(steadyTest, "pass"))},
	}}
	_, raw, _ := runAndWriteVerdict(t, seam)

	const golden = `{
  "schema_version": "1.0",
  "cycle": 466,
  "predicate_suite": {
    "this_cycle_count": 1,
    "regression_suite_count": 0,
    "red_team_count": 0,
    "skipped_count": 0,
    "total": 1
  },
  "results": [
    {
      "ac_id": "cycle466/TestC466_003_NegativeAndEmptyRejectedSequentialFallback",
      "predicate": "go/acs/cycle466/...:TestC466_003_NegativeAndEmptyRejectedSequentialFallback",
      "exit_code": 0,
      "result": "green",
      "duration_ms": 10,
      "is_regression": false
    }
  ],
  "green_count": 1,
  "red_count": 0,
  "skip_count": 0,
  "red_ids": null,
  "verdict": "PASS",
  "ship_eligible": true
}`
	if raw != golden {
		t.Errorf("no-flake verdict JSON is not byte-identical to the pre-change golden\n--- got ---\n%s\n--- want ---\n%s", raw, golden)
	}
}

func TestC468_005_RaceVetApicoverCleanOnTouchedPackages(t *testing.T) {
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "test", "-race", "-count=1", acssuitePkg+"/...", auditPkg+"/...")
	if code != 0 {
		t.Errorf("full-package -race regression on internal/acssuite + internal/phases/audit is red (exit=%d)\n%s\n%s", code, excerptFor(stdout), excerptFor(stderr))
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, vetErr, vetCode, _ := acsassert.SubprocessOutput("bash", "-c",
		"cd "+goDir+" && go vet ./internal/acssuite/... ./internal/phases/audit/...")
	if vetCode != 0 {
		t.Errorf("go vet ./internal/acssuite/... ./internal/phases/audit/... is red (exit=%d)\n%s\n%s", vetCode, vetOut, vetErr)
	}
	apicoverCmd := "T=$(mktemp -d) && cd " + goDir + " && " +
		"go test -coverprofile=\"$T/cover.txt\" ./internal/acssuite/ >/dev/null && " +
		"go tool cover -func=\"$T/cover.txt\" > \"$T/cover.func.txt\" && " +
		"go run ./cmd/apicover -enforce -cover \"$T/cover.func.txt\" $(go list -f '{{.Dir}}' ./internal/acssuite)"
	apiOut, apiErr, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/acssuite is red (exit=%d)\n%s\n%s", apiCode, apiOut, apiErr)
	}
}

func excerptFor(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 600 {
		return s
	}
	return s[:600] + "…"
}
