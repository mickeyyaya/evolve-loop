package debugger

import "testing"

func TestDecisionRationale(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantRationale string
	}{
		{
			name:          "new rationale field is read",
			body:          `{"action":"RESHIP","root_cause":"stale ref","rationale":"safe retry"}`,
			wantRationale: "safe retry",
		},
		{
			name:          "legacy reasoning field is still read",
			body:          `{"action":"RESHIP","root_cause":"stale ref","reasoning":"legacy safe retry"}`,
			wantRationale: "legacy safe retry",
		},
		{
			name:          "rationale wins when both fields are present",
			body:          `{"action":"BLOCK","rationale":"new text","reasoning":"old text"}`,
			wantRationale: "new text",
		},
		{
			name:          "legacy reasoning fills a blank rationale",
			body:          `{"action":"BLOCK","rationale":"  ","reasoning":"old text"}`,
			wantRationale: "old text",
		},
		{
			name:          "neither field leaves no rationale signal",
			body:          `{"action":"BLOCK","root_cause":"integrity breach"}`,
			wantRationale: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDecision(t, dir, tc.body)
			_, signals, _ := Classify(dir)
			got, present := signals["debugger.rationale"]
			if got != tc.wantRationale {
				t.Errorf("debugger.rationale = %q, want %q", got, tc.wantRationale)
			}
			if tc.wantRationale == "" && present {
				t.Errorf("debugger.rationale signal present for an empty rationale")
			}
		})
	}
}
