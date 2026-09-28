package auditcalibration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const validShadowJSON = `{"cycle":3,"phase":"audit","narrative_verdict":"PASS","chain_verdict":"PASS"}`

func TestLoadPair_LabelsEachUnusableArtifact(t *testing.T) {
	cases := []struct {
		name, shadow, dossierJSON, failReason string
		shadowIsDir, dossierIsDir             bool
		wantReason, wantDetail                string
	}{
		{name: "unreadable shadow", shadowIsDir: true, wantReason: "malformed-shadow"},
		{name: "shadow bound to another cycle", shadow: `{"cycle":4,"narrative_verdict":"PASS","chain_verdict":"PASS"}`, wantReason: "malformed-shadow", wantDetail: "invalid verdict fields or cycle binding"},
		{name: "unreadable dossier", shadow: validShadowJSON, dossierIsDir: true, wantReason: "malformed-dossier"},
		{name: "missing dossier", shadow: validShadowJSON, wantReason: "missing-dossier"},
		{name: "dossier bound to another cycle", shadow: validShadowJSON, dossierJSON: testDossier(4, "PASS"), wantReason: "malformed-dossier", wantDetail: "invalid dossier or cycle binding"},
		{name: "dossier failing Validate", shadow: validShadowJSON, dossierJSON: `{"cycle":3,"goal":"test","final_verdict":"FAIL","phases":[{"name":"audit","verdict":"FAIL"}]}`, wantReason: "malformed-dossier", wantDetail: "invalid dossier or cycle binding"},
		{name: "malformed fail reason", shadow: validShadowJSON, dossierJSON: testDossier(3, "PASS"), failReason: "{not json", wantReason: "malformed-fail-reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dossiers, runs := testCorpus(t)
			runDir := filepath.Join(runs, "cycle-3")
			writeOrMkdir(t, filepath.Join(runDir, "audit-chain-shadow.json"), tc.shadow, tc.shadowIsDir)
			writeOrMkdir(t, filepath.Join(dossiers, "cycle-3.json"), tc.dossierJSON, tc.dossierIsDir)
			if tc.failReason != "" {
				writeTestFile(t, filepath.Join(runDir, "audit-fail-reason.json"), tc.failReason)
			}

			p, ex := loadPair(3, dossiers, runs)

			if p != nil || ex == nil {
				t.Fatalf("loadPair = (%+v, %+v), want an exclusion", p, ex)
			}
			if ex.cycle != 3 || ex.reason != tc.wantReason {
				t.Errorf("exclusion = %+v, want cycle 3 reason %q", ex, tc.wantReason)
			}
			if tc.wantDetail != "" && ex.detail != tc.wantDetail {
				t.Errorf("exclusion detail = %q, want %q", ex.detail, tc.wantDetail)
			}
		})
	}
}

func TestLoadPair_NormalizesVerdictsGateAndOverrides(t *testing.T) {
	cases := []struct {
		name, shadow string
		want         pair
	}{
		{
			name:   "overridden pair with padded verdicts",
			shadow: `{"cycle":3,"narrative_verdict":" PASS ","chain_verdict":" WARN ","shipped_verdict":" FAIL ","overrode_by":["EGPS","contract"]}`,
			want:   pair{cycle: 3, narrative: "PASS", chain: "WARN", gate: "FAIL", shipped: "FAIL", overriddenBy: "EGPS, contract"},
		},
		{
			name:   "unshipped pair falls back to the dossier verdict",
			shadow: `{"cycle":3,"narrative_verdict":"WARN","chain_verdict":"PASS"}`,
			want:   pair{cycle: 3, narrative: "WARN", chain: "PASS", gate: "PASS", shipped: "WARN"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dossiers, runs := testCorpus(t)
			writeTestFile(t, filepath.Join(runs, "cycle-3", "audit-chain-shadow.json"), tc.shadow)
			writeTestFile(t, filepath.Join(dossiers, "cycle-3.json"), testDossier(3, "WARN"))

			p, ex := loadPair(3, dossiers, runs)

			if ex != nil {
				t.Fatalf("loadPair excluded the pair: %+v", ex)
			}
			if !reflect.DeepEqual(*p, tc.want) {
				t.Errorf("pair = %+v, want %+v", *p, tc.want)
			}
		})
	}
}

func TestRender_WritesEverySectionInFixedOrder(t *testing.T) {
	pairs := []pair{
		{cycle: 1, narrative: "PASS", chain: "PASS", gate: "PASS", shipped: "PASS", classes: []string{"a|b", "shared"}},
		{cycle: 2, narrative: "PASS", chain: "WARN", gate: "FAIL", shipped: "FAIL", overriddenBy: "EGPS | x", classes: []string{"shared"}},
		{cycle: 3, narrative: "FAIL", chain: "FAIL", gate: "PASS", shipped: "FAIL"},
	}
	exclusions := []exclusion{{cycle: 4, reason: "missing-shadow", detail: "open  x|y"}}
	want := strings.Join([]string{
		"# Auditor Calibration Report",
		"",
		"Sample rule: canonical `cycle-N` directories under the runs directory, each paired with its committed dossier.",
		"Exclusion rule: missing or malformed pair artifacts are counted and listed; they never enter the agreement matrix.",
		"Interpretation rule: the gate is FAIL when a deterministic override is recorded, otherwise PASS. This report does not recommend changing a persona rubric from a single anecdote.",
		"",
		"Valid pairs: 3",
		"Excluded: 1",
		"",
		"## Narrative × Deterministic Gate Matrix",
		"| Narrative | Gate | Count |",
		"|---|---|---:|",
		"| PASS | PASS | 1 |",
		"| PASS | FAIL | 1 |",
		"| FAIL | PASS | 1 |",
		"",
		"## Defect Classes",
		"| Class | Count |",
		"|---|---:|",
		`| a\|b | 1 |`,
		"| shared | 2 |",
		"",
		"## Force Overrides",
		"| Cycle | Narrative | Shipped | Overrode By |",
		"|---:|---|---|---|",
		`| 2 | PASS | FAIL | EGPS \| x |`,
		"",
		"## Valid Pairs",
		"| Cycle | Narrative | Chain | Gate | Shipped | Overrode By | Defect Classes |",
		"|---:|---|---|---|---|---|---|",
		`| 1 | PASS | PASS | PASS | PASS |  | a\|b, shared |`,
		`| 2 | PASS | WARN | FAIL | FAIL | EGPS \| x | shared |`,
		"| 3 | FAIL | FAIL | PASS | FAIL |  |  |",
		"",
		"## Exclusions",
		"| Cycle | Reason | Detail |",
		"|---:|---|---|",
		`| 4 | missing-shadow | open x\|y |`,
		"",
	}, "\n")

	got := string(render(pairs, exclusions))

	if got != want {
		t.Errorf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func writeOrMkdir(t *testing.T, path, content string, isDir bool) {
	t.Helper()
	switch {
	case isDir:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	case content != "":
		writeTestFile(t, path, content)
	}
}
