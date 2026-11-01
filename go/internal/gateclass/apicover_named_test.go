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
