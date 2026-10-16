package acsrunner

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func ParseTestJSONForTest(t *testing.T, stream string, cycle int) Verdict {
	t.Helper()
	v, err := ParseTestJSON(strings.NewReader(stream), cycle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return v
}

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
	var _ PredicateSuiteCounts = v.PredicateSuite
	if view.PredicateSuite.Total != view.Total || view.Total == 0 {
		t.Errorf("predicate_suite.total = %d, flat total = %d — the ship gate reads the nested one and printed 0",
			view.PredicateSuite.Total, view.Total)
	}
}
