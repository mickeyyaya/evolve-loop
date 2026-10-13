package llmroute

import "testing"

func TestDispatchTiered_ANoCandidatePlanFailsWithoutLaunchingAnything(t *testing.T) {
	launched := false
	launch := func(cli, tier string) (int, error) { launched = true; return 0, nil }

	res := DispatchTiered(Plan{}, launch, func(string, string) {})

	if res.Err == nil || res.Err.Error() != "llmroute: DispatchTiered called with no candidates" {
		t.Fatalf("res = %+v", res)
	}
	if launched {
		t.Error("nothing may be launched for a plan with no candidates")
	}
}
