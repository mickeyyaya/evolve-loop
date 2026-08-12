package acsrunner

// schema_lockstep_test.go — the two producers of acs-verdict.json must write a
// shape the SAME reader can grade.
//
// The runner's own doc comment claimed the producers "stay in schema lockstep".
// They did not: `evolve acs run` omitted `ship_eligible`, and the audit gate
// reads that field as a POINTER precisely so a pre-stamp verdict stays
// back-compatible — absent means "no opinion", which the gate honours by NOT
// blocking. So a verdict written through this path silently disabled the
// ship-eligibility gate, and the comment asserted the opposite.
//
// The lesson this pins is the session's recurring one: a claim of lockstep in
// prose is not lockstep. The only durable form is a test that decodes BOTH
// writers' output through the reader's own expectations.

import (
	"encoding/json"
	"strings"
	"testing"
)

// greenStream / redStream are `go test -json` transcripts in the shape the
// runner really consumes (same fixture style as runner_test.go).
func greenStream() string {
	return strings.Join([]string{
		`{"Action":"run","Package":"acs/cycle-42","Test":"TestC42_001"}`,
		`{"Action":"pass","Package":"acs/cycle-42","Test":"TestC42_001","Elapsed":0.1}`,
	}, "\n") + "\n"
}

func redStream() string {
	return strings.Join([]string{
		`{"Action":"run","Package":"acs/cycle-42","Test":"TestC42_001"}`,
		`{"Action":"pass","Package":"acs/cycle-42","Test":"TestC42_001","Elapsed":0.1}`,
		`{"Action":"run","Package":"acs/cycle-42","Test":"TestC42_002"}`,
		`{"Action":"fail","Package":"acs/cycle-42","Test":"TestC42_002","Elapsed":0.2}`,
	}, "\n") + "\n"
}

// ParseTestJSONForTest is the parse the production runner performs.
func ParseTestJSONForTest(t *testing.T, stream string, cycle int) Verdict {
	t.Helper()
	v, err := ParseTestJSON(strings.NewReader(stream), cycle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return v
}

// gateView is what the audit's EGPS reader actually looks for
// (internal/phases/audit readACSVerdict). ShipEligible is a POINTER on purpose:
// nil is the back-compat "unstamped" case the gate must not block on, which is
// exactly why a producer that omits it is dangerous rather than merely untidy.
type gateView struct {
	SchemaVersion string   `json:"schema_version"`
	Cycle         int      `json:"cycle"`
	RedCount      int      `json:"red_count"`
	RedIDs        []string `json:"red_ids"`
	Verdict       string   `json:"verdict"`
	ShipEligible  *bool    `json:"ship_eligible"`
}

func decodeAsGate(t *testing.T, v Verdict) gateView {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got gateView
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the gate cannot decode this producer's output: %v\n%s", err, raw)
	}
	return got
}

// A GREEN run must present as shippable to the gate, explicitly — not by the
// gate failing to find an opinion.
func TestVerdict_GreenRunIsExplicitlyShipEligible(t *testing.T) {
	t.Parallel()
	v := ParseTestJSONForTest(t, greenStream(), 42)
	got := decodeAsGate(t, v)

	if got.ShipEligible == nil {
		t.Fatal("ship_eligible is ABSENT — the audit reads it as 'unstamped' and declines to block, so this producer silently disables the ship-eligibility gate")
	}
	if !*got.ShipEligible {
		t.Error("a run with no reds must be ship-eligible")
	}
	if got.Verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS — readers other than the EGPS gate key off this field", got.Verdict)
	}
	if got.SchemaVersion == "" {
		t.Error("schema_version is absent; a consumer cannot tell which producer wrote the file")
	}
}

// And the direction that matters most: a RED run must be unambiguously
// unshippable through every field the gate consults.
func TestVerdict_RedRunIsExplicitlyNotShipEligible(t *testing.T) {
	t.Parallel()
	v := ParseTestJSONForTest(t, redStream(), 42)
	got := decodeAsGate(t, v)

	if got.RedCount == 0 {
		t.Fatal("fixture: expected a red")
	}
	if got.ShipEligible == nil || *got.ShipEligible {
		t.Error("a run with reds must be explicitly NOT ship-eligible — leaving the gate to infer it from red_count alone is the asymmetry that let this path ship")
	}
	if got.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL", got.Verdict)
	}
	if len(got.RedIDs) == 0 {
		t.Error("red_ids is empty — the gate's diagnostic names which predicates failed, and an operator holding a block needs them")
	}
}

// TestVerdict_TruncatedStreamIsNotShipEligible — the review BLOCK, and the
// defect this change introduced while closing another.
//
// A predicate whose stream carries `run` but never `pass`/`fail`/`skip` is what
// a SIGKILLed, timed-out or panic-aborted `go test -json` produces — the
// watchdog kills hung suites, so it is a real shape, not a hypothetical. The
// aggregation's default arm counted that as GREEN. Before this change the
// producer wrote no opinion at all and the gate's nil back-compat treated the
// file as unstamped; making the opinion explicit turned silence into an
// affirmative "PASS, ship-eligible" — and cmd_acs writes the verdict even on
// the error path by design, so nothing downstream would have caught it.
func TestVerdict_TruncatedStreamIsNotShipEligible(t *testing.T) {
	t.Parallel()
	truncated := strings.Join([]string{
		`{"Action":"run","Package":"acs/cycle-42","Test":"TestC42_001"}`,
		`{"Action":"pass","Package":"acs/cycle-42","Test":"TestC42_001","Elapsed":0.1}`,
		`{"Action":"run","Package":"acs/cycle-42","Test":"TestC42_002"}`,
	}, "\n") + "\n"
	v := ParseTestJSONForTest(t, truncated, 42)
	got := decodeAsGate(t, v)

	if got.ShipEligible == nil || *got.ShipEligible {
		t.Error("a suite killed mid-run claimed ship-eligibility — an incomplete predicate is not a passing one")
	}
	if got.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL: the run did not finish, so it did not pass", got.Verdict)
	}
	if v.GreenCount != 1 {
		t.Errorf("green_count = %d, want 1 — only the predicate that actually reported PASS is green", v.GreenCount)
	}
}

// The ship gate reads predicate_suite.total; acsrunner wrote only a flat
// `total`, so every acsrunner-written verdict printed "total=0" in the block
// message an operator reads while holding a block (review MEDIUM).
func TestVerdict_CarriesTheNestedTotalTheShipGateReads(t *testing.T) {
	t.Parallel()
	v := ParseTestJSONForTest(t, redStream(), 42)
	var view struct {
		Total          int `json:"total"`
		PredicateSuite struct {
			Total int `json:"total"`
		} `json:"predicate_suite"`
	}
	raw, _ := json.Marshal(v)
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	// Names PredicateSuiteCounts: the nested block exists solely so the ship
	// gate reads a real number, and a type nobody names is one a future
	// refactor drops without noticing which message went blank.
	var _ PredicateSuiteCounts = v.PredicateSuite
	if view.PredicateSuite.Total != view.Total || view.Total == 0 {
		t.Errorf("predicate_suite.total = %d, flat total = %d — the ship gate reads the nested one and printed 0",
			view.PredicateSuite.Total, view.Total)
	}
}
