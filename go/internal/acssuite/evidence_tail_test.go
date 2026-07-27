package acssuite

// evidence_tail_test.go — the evidence must carry the FAILURE, not the boot
// noise. Measured across all five red predicates of cycles 1107/1115/1116/1117
// and again on cycle-1123: evidence_excerpt was head-truncated at evidenceMax,
// a go-test run's first 600 chars are compiler/WARN noise, so "--- FAIL:" was
// present in NONE of them and for 1107/1116/1123 the failing test name is
// permanently unrecoverable — the reason those cycles' false reds could never
// be confirmed or refuted from disk. The excerpt must be TAIL-anchored (Go
// test output accumulates assertion detail and the FAIL line at the end), and
// a red Result must either name the failing inner tests or say why it cannot.

import (
	"fmt"
	"strings"
	"testing"
)

// TestExcerpt_IsTailAnchored pins the ~10-line core fix: when output exceeds
// evidenceMax, keep the END. A meta-predicate's t.Logf carries the inner
// subprocess's full output — noise first, `--- FAIL:` at the tail.
func TestExcerpt_IsTailAnchored(t *testing.T) {
	noise := strings.Repeat("[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled\n", 20)
	failure := "--- FAIL: TestInnerThing (0.01s)\n    thing_test.go:42: the actual assertion"
	got := excerpt(noise + failure)

	if len(got) > evidenceMax+len("…") {
		t.Fatalf("excerpt length %d exceeds evidenceMax %d", len(got), evidenceMax)
	}
	if !strings.Contains(got, "--- FAIL: TestInnerThing") {
		t.Errorf("excerpt dropped the failure line — it kept the HEAD (boot noise) and truncated the tail:\n%q\n"+
			"a red whose excerpt is all noise is undiagnosable; 1107/1116/1123's failing test names were lost exactly this way", got)
	}
	if !strings.Contains(got, "the actual assertion") {
		t.Errorf("excerpt dropped the assertion detail at the very end: %q", got)
	}
}

// TestParseGoTestJSON_RedCarriesInnerFailingTestNames pins the structured
// half: a red predicate whose output embeds inner `--- FAIL:` lines (the
// meta-predicate shape) surfaces those names in Result.FailingTests, so the
// verdict names WHAT failed even after truncation.
func TestParseGoTestJSON_RedCarriesInnerFailingTestNames(t *testing.T) {
	noise := strings.Repeat("build noise line\\n", 60)
	stream := `{"Action":"run","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green"}
{"Action":"output","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green","Output":"` + noise + `"}
{"Action":"output","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green","Output":"    --- FAIL: TestZZAudit_Probe (0.00s)\n"}
{"Action":"output","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green","Output":"--- FAIL: TestC9999_006_suites_stay_green (41.03s)\n"}
{"Action":"fail","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green","Elapsed":41.03}
`
	results := parseGoTestJSON(strings.NewReader(stream), 9999)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(results), results)
	}
	r := results[0]
	if r.ResultStr != "red" {
		t.Fatalf("result = %q, want red", r.ResultStr)
	}
	if len(r.FailingTests) == 0 {
		t.Fatalf("red Result carries no FailingTests — the inner failure name is exactly what 1107/1116/1123 lost; excerpt truncation must not be the only carrier")
	}
	joined := strings.Join(r.FailingTests, ",")
	if !strings.Contains(joined, "TestZZAudit_Probe") {
		t.Errorf("FailingTests = %v, want the INNER failing test TestZZAudit_Probe — that name is the false-red diagnosis", r.FailingTests)
	}
}

// TestParseGoTestJSON_RedWithoutFailLineRecordsWhy (negative / the
// unextractable case): a red with no `--- FAIL:` anywhere (compile failure,
// timeout, signal) must say so explicitly rather than emitting a content-free
// exit code.
func TestParseGoTestJSON_RedWithoutFailLineRecordsWhy(t *testing.T) {
	stream := `{"Action":"run","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_001_thing"}
{"Action":"output","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_001_thing","Output":"predicates_test.go:9:2: undefined: somesymbol\n"}
{"Action":"fail","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_001_thing","Elapsed":0.4}
`
	results := parseGoTestJSON(strings.NewReader(stream), 9999)
	if len(results) != 1 || results[0].ResultStr != "red" {
		t.Fatalf("want one red result, got %+v", results)
	}
	r := results[0]
	if len(r.FailingTests) != 0 {
		t.Errorf("FailingTests = %v, want empty — no --- FAIL line exists to extract", r.FailingTests)
	}
	if r.EvidenceNote == "" {
		t.Errorf("EvidenceNote is empty — a red with no extractable test name must record WHY (compile failure/timeout/signal), or the verdict is a content-free exit 1")
	}
}

// TestExtractFailingTests_DedupesAndBounds keeps the extractor honest on
// pathological output: duplicates collapse, and the list is bounded so a
// 10k-failure sweep cannot bloat the verdict JSON.
func TestExtractFailingTests_DedupesAndBounds(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("--- FAIL: TestSame (0.0s)\n")
	}
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestDistinct%d (0.0s)\n", i)
	}
	got := extractFailingTests(b.String())
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Fatalf("duplicate %q in FailingTests %v", n, got)
		}
		seen[n] = true
	}
	if len(got) != maxFailingTests {
		t.Errorf("extractor returned %d names, cap is %d — the bound must actually engage on a mass failure", len(got), maxFailingTests)
	}
	if len(got) == 0 || !seen["TestSame"] {
		t.Errorf("extractor missed TestSame entirely: %v", got)
	}
}
