package phasespec

import (
	"strings"
	"testing"
)

func TestValidateLogAccess_RejectsEachBadKey(t *testing.T) {
	t.Parallel()
	cases := map[string]LogAccess{
		"log_access.layers":      {Layers: []string{"D4"}},
		"log_access.categories":  {Categories: []string{""}},
		"log_access.scope":       {Scope: "world"},
		"log_access.line_budget": {LineBudget: -1},
	}
	for want, a := range cases {
		a := a
		v := ValidateLogAccess(&a)
		if len(v) != 1 || !strings.HasPrefix(v[0], want) {
			t.Errorf("ValidateLogAccess(%+v) = %v, want one violation of %s", a, v, want)
		}
	}
	if v := ValidateLogAccess(nil); v != nil {
		t.Errorf("no log_access = %v, want none", v)
	}
	if v := ValidateLogAccess(&LogAccess{Layers: []string{"D0", "D3"}, Scope: "run", LineBudget: 10}); v != nil {
		t.Errorf("a valid log_access = %v", v)
	}
}

func TestLogAccessAllows_NilAllowsEveryLayer(t *testing.T) {
	t.Parallel()
	var none *LogAccess
	declared := &LogAccess{Layers: []string{"D0"}}
	if !none.Allows("D3") || !declared.Allows("D0") || declared.Allows("D1") {
		t.Error("Allows: nil allows all, a declared access allows only its layers")
	}
}
