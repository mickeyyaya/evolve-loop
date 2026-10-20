package guards

import (
	"slices"
	"testing"
)

func TestProtectedPathsIn_NamesEachProtectedPathOnceInOrder(t *testing.T) {
	text := "2. Wire the lint into `go/internal/cli/guardcmd/eval.go:40` (pre-flight).\n" +
		"3. Add go/internal/evalqualitycheck/unsatisfiable.go and docs/x.md.\n" +
		"4. Re-check go/internal/cli/guardcmd/eval.go, then .evolve/policy.json."

	got := ProtectedPathsIn(text)

	want := []string{".evolve/policy.json", "go/internal/cli/guardcmd/eval.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("ProtectedPathsIn = %v, want %v: every protected path, located or not, once and sorted; lane paths are not named", got, want)
	}
}

func TestProtectedPathsIn_TextWithoutAProtectedPathNamesNone(t *testing.T) {
	if got := ProtectedPathsIn("wire into pre-flight; touch go/internal/evalqualitycheck/ and docs/a.md"); len(got) != 0 {
		t.Fatalf("ProtectedPathsIn = %v, want none", got)
	}
}
