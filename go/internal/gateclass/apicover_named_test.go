package gateclass

import "testing"

func TestFlag_StringNamesThePatternThePathAndTheDetail(t *testing.T) {
	t.Parallel()
	cases := map[Flag]string{
		{Pattern: PatternAddedSkip, Path: "a_test.go", Detail: "t.Skip()"}: "added-skip a_test.go (t.Skip())",
		{Pattern: PatternFrozenCheck, Path: "go/acs/cycle1/p_test.go"}:     "frozen-check go/acs/cycle1/p_test.go",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestIsGuardTest_NamesAGuardTestByItsDirectoryOrItsName(t *testing.T) {
	for path, want := range map[string]bool{
		"go/internal/guards/role_test.go":         true,
		"go/internal/widget/widget_guard_test.go": true,
		"go/acs/regression/docgo/docgo_test.go":   true,
		"go/internal/repocontract/x_test.go":      true,
		"go/internal/widget/widget_test.go":       false,
		"go/internal/widget/guard.go":             false,
		"go/internal/widget/safeguards/x_test.go": false,
	} {
		if got := IsGuardTest(path); got != want {
			t.Errorf("IsGuardTest(%q) = %v, want %v", path, got, want)
		}
	}
}
