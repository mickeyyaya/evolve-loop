package phasespec

import (
	"encoding/json"
	"testing"
)

func TestAmplifiedLogAccess_EmptyDeclaredLayersAllowNothing(t *testing.T) {
	t.Parallel()
	a := &LogAccess{}
	for _, l := range []string{"D0", "D1", "D2", "D3", ""} {
		if a.Allows(l) {
			t.Errorf("empty declared layers allowed %q", l)
		}
	}
}

func TestAmplifiedLogAccess_LayerMatchIsExact(t *testing.T) {
	t.Parallel()
	a := &LogAccess{Layers: []string{"D1"}}
	for _, l := range []string{"d1", "D1 ", "D", "D11"} {
		if a.Allows(l) {
			t.Errorf("Allows(%q) = true, want false", l)
		}
	}
}

func TestAmplifiedLogAccess_ZeroBudgetIsValidAndEachBadFieldIsReported(t *testing.T) {
	t.Parallel()
	if v := ValidateLogAccess(&LogAccess{}); v != nil {
		t.Errorf("zero value = %v, want none", v)
	}
	v := ValidateLogAccess(&LogAccess{Layers: []string{"D9"}, Scope: "x", LineBudget: -5})
	if len(v) < 3 {
		t.Errorf("three bad fields gave %d violations: %v", len(v), v)
	}
}

func TestAmplifiedLogAccess_JSONRoundTripKeepsFields(t *testing.T) {
	t.Parallel()
	in := LogAccess{Layers: []string{"D0", "D2"}, Categories: []string{"phase"}, Scope: "run", LineBudget: 7}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out LogAccess
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Allows("D2") || out.Allows("D1") || out.Scope != "run" || out.LineBudget != 7 || len(out.Categories) != 1 {
		t.Errorf("round trip lost data: %+v", out)
	}
}
