package phasecontract

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const validDiagnosis = `{"schema":"evolve.diagnosis/1","incident_id":"inc-2026-10-27-235078cd",` +
	`"hypothesis":"the gc sweep of PR 1131 holds the CI runner past the hang limit","evidence":["i1","s2"],` +
	`"action":{"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"a boundary run after a merge hangs"}},` +
	`"alternative":{"kind":"revert-last-landing","params":{"sha":"abc1234"}},` +
	`"expected_effect":"the next run of required.yml on main ends under the hang limit","confidence":"medium"}`

var diagnosisEvidence = []string{"i1", "s1", "s2"}

func diagnosisWith(old, repl string) []byte {
	return []byte(strings.Replace(validDiagnosis, old, repl, 1))
}

func TestParseDiagnosis_RefusesAnActionOutsideTheCatalog(t *testing.T) {
	t.Parallel()
	raw := diagnosisWith(`"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"a boundary run after a merge hangs"}`,
		`"kind":"run-shell","params":{"reason":"rm -rf .evolve/incidents"}`)

	if _, err := ParseDiagnosis(raw, "inc-2026-10-27-235078cd", diagnosisEvidence); err == nil || !strings.Contains(err.Error(), `action kind "run-shell" is not in the catalog`) {
		t.Fatalf("an action outside the catalog must be refused, got %v", err)
	}
}

func TestParseDiagnosis_AcceptsADiagnosisOfTheCatalog(t *testing.T) {
	t.Parallel()
	d, err := ParseDiagnosis([]byte(validDiagnosis+"\n"), "inc-2026-10-27-235078cd", diagnosisEvidence)
	if err != nil {
		t.Fatalf("a valid diagnosis was refused: %v", err)
	}
	if d.Action.Kind != ActionOpenRepairItem || !slices.Equal(d.Action.Params.Files, []string{"go/internal/gc/sweep.go"}) || d.Alternative == nil || d.Alternative.Params.SHA != "abc1234" {
		t.Fatalf("decoded %+v", d)
	}
}

func TestParseDiagnosis_EachEvidenceIDMustExistInTheIndex(t *testing.T) {
	t.Parallel()
	raw := diagnosisWith(`"evidence":["i1","s2"]`, `"evidence":["i1","s9"]`)
	if _, err := ParseDiagnosis(raw, "inc-2026-10-27-235078cd", diagnosisEvidence); err == nil || !strings.Contains(err.Error(), `evidence "s9" is not an entry of the evidence index`) {
		t.Fatalf("an evidence id outside the index must be refused, got %v", err)
	}
	empty := diagnosisWith(`"evidence":["i1","s2"]`, `"evidence":[]`)
	if _, err := ParseDiagnosis(empty, "inc-2026-10-27-235078cd", diagnosisEvidence); err == nil || !strings.Contains(err.Error(), "names no evidence") {
		t.Fatalf("a diagnosis with no evidence must be refused, got %v", err)
	}
}

func TestParseDiagnosis_Refusals(t *testing.T) {
	t.Parallel()
	const repair = `"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"a boundary run after a merge hangs"}`
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"prose around the json", []byte("My diagnosis: " + validDiagnosis), "invalid character"},
		{"an unknown top-level key", diagnosisWith(`"confidence":"medium"`, `"confidence":"medium","tier":"T1"`), `unknown field "tier"`},
		{"a missing key", diagnosisWith(`,"expected_effect":"the next run of required.yml on main ends under the hang limit"`, ``), `missing field "expected_effect"`},
		{"an unknown param", diagnosisWith(repair, `"kind":"halt-and-escalate","params":{"command":"x"}`), `unknown field "command"`},
		{"the wrong schema", diagnosisWith(`evolve.diagnosis/1`, `evolve.diagnosis/2`), `schema "evolve.diagnosis/2"`},
		{"another incident", diagnosisWith(`"incident_id":"inc-2026-10-27-235078cd"`, `"incident_id":"inc-other"`), `incident_id "inc-other" is not the incident`},
		{"an empty hypothesis", diagnosisWith(`"hypothesis":"the gc sweep of PR 1131 holds the CI runner past the hang limit"`, `"hypothesis":" "`), "hypothesis is empty"},
		{"an empty expected effect", diagnosisWith(`"expected_effect":"the next run of required.yml on main ends under the hang limit"`, `"expected_effect":""`), "expected_effect is empty"},
		{"an unknown confidence", diagnosisWith(`"confidence":"medium"`, `"confidence":"sure"`), `confidence "sure"`},
		{"a missing required param", diagnosisWith(repair, `"kind":"open-repair-item","params":{"files":["go/x.go"]}`), `open-repair-item needs the param "repro"`},
		{"a param of another kind", diagnosisWith(repair, `"kind":"revert-last-landing","params":{"sha":"abc1234","item":"x"}`), `revert-last-landing does not take the param "item"`},
		{"an unknown variation", diagnosisWith(repair, `"kind":"retry-with-variation","params":{"item":"a","variation":"luck"}`), `variation "luck"`},
		{"a sha that is not hex", diagnosisWith(repair, `"kind":"revert-last-landing","params":{"sha":"HEAD~1"}`), `sha "HEAD~1"`},
		{"a file outside the repository", diagnosisWith(repair, `"kind":"open-repair-item","params":{"files":["../etc/passwd"],"repro":"r"}`), `file "../etc/passwd"`},
		{"a bad alternative", diagnosisWith(`"kind":"revert-last-landing","params":{"sha":"abc1234"}`, `"kind":"run-shell","params":{}`), `alternative: action kind "run-shell"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseDiagnosis(c.raw, "inc-2026-10-27-235078cd", diagnosisEvidence); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want a refusal with %q, got %v", c.want, err)
			}
		})
	}
}

func TestParseDiagnosis_EachCatalogActionWithItsParamsIsAccepted(t *testing.T) {
	t.Parallel()
	const repair = `"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"a boundary run after a merge hangs"}`
	actions := map[string]string{
		ActionRetryWithVariation: `"kind":"retry-with-variation","params":{"item":"a","variation":"model"}`,
		ActionRerouteFamily:      `"kind":"reroute-family","params":{"item":"a","family":"codex"}`,
		ActionQuarantineItem:     `"kind":"quarantine-item","params":{"item":"a","reason":"it loops"}`,
		ActionRevertLastLanding:  `"kind":"revert-last-landing","params":{"sha":"0123456789abcdef0123456789abcdef01234567"}`,
		ActionOpenRepairItem:     repair,
		ActionHaltAndEscalate:    `"kind":"halt-and-escalate","params":{}`,
	}
	if got := DiagnosisActions(); len(got) != len(actions) {
		t.Fatalf("DiagnosisActions() = %v, want the %d actions of llm-isolation.md §19", got, len(actions))
	}
	for _, kind := range DiagnosisActions() {
		body, ok := actions[kind]
		if !ok {
			t.Fatalf("no test row for the catalog action %q", kind)
		}
		d, err := ParseDiagnosis(diagnosisWith(repair, body), "inc-2026-10-27-235078cd", diagnosisEvidence)
		if err != nil || d.Action.Kind != kind {
			t.Errorf("%s: %v %+v", kind, err, d.Action)
		}
	}
}

func TestDiagnosis_TheTypedValueRoundTripsThroughTheSchemaAtEachConfidence(t *testing.T) {
	t.Parallel()
	for _, confidence := range []string{ConfidenceLow, ConfidenceMedium, ConfidenceHigh} {
		want := Diagnosis{Schema: DiagnosisSchema, IncidentID: "inc-a", Hypothesis: "h", Evidence: []string{"i1"},
			Action:         DiagnosisAction{Kind: ActionRerouteFamily, Params: DiagnosisParams{Item: "a", Family: "codex"}},
			ExpectedEffect: "e", Confidence: confidence}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseDiagnosis(raw, "inc-a", []string{"i1"})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v\n got %+v\nwant %+v", confidence, err, got, want)
		}
	}
}
