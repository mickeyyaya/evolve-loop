package phasespec

import "testing"

func TestRoleOrDefault(t *testing.T) {
	tests := []struct {
		name string
		spec PhaseSpec
		want Role
	}{
		{"explicit role wins", PhaseSpec{Name: "build", Role: "evaluate"}, RoleEvaluate},
		{"scout infers plan", PhaseSpec{Name: "scout"}, RolePlan},
		{"tdd infers plan", PhaseSpec{Name: "tdd"}, RolePlan},
		{"build infers build", PhaseSpec{Name: "build"}, RoleBuild},
		{"audit infers evaluate", PhaseSpec{Name: "audit"}, RoleEvaluate},
		{"tester infers evaluate", PhaseSpec{Name: "tester"}, RoleEvaluate},
		{"ship infers control", PhaseSpec{Name: "ship"}, RoleControl},
		{"architecture-design infers plan", PhaseSpec{Name: "architecture-design"}, RolePlan},
		{"unknown minted phase defaults to plan", PhaseSpec{Name: "some-novel-minted-phase"}, RolePlan},
		{"mis-cased explicit role is normalized", PhaseSpec{Name: "x", Role: "EVALUATE"}, RoleEvaluate},
		{"padded explicit role is trimmed", PhaseSpec{Name: "x", Role: " build "}, RoleBuild},
		{"unrecognized explicit role falls through to name inference", PhaseSpec{Name: "audit", Role: "wizard"}, RoleEvaluate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.spec.RoleOrDefault(); got != tc.want {
				t.Errorf("RoleOrDefault() = %q, want %q", got, tc.want)
			}
		})
	}
}
