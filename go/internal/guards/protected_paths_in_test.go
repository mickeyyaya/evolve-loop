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

func TestProtectedPathsIn_AModuleRelativePathIsReadUnderGo(t *testing.T) {
	got := ProtectedPathsIn("wire it into internal/cli/guardcmd/eval.go and cmd/evolve/main.go; skills/audit/SKILL.md stays")

	want := []string{"go/internal/cli/guardcmd/eval.go", "skills/audit/SKILL.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("ProtectedPathsIn = %v, want %v: a Go agent writes paths relative to the go/ module", got, want)
	}
}

func TestPathsIn_NamesEachPathOrFileToken(t *testing.T) {
	for text, want := range map[string]int{
		"go/internal/x/x.go and docs/a.md": 2,
		"Makefile.am":                      1,
		"-":                                0,
		"n/a":                              0,
		"none":                             0,
		"e.g. none":                        0,
		"version 1.5 or v1.2":              0,
	} {
		if got := len(PathsIn(text)); got != want {
			t.Errorf("PathsIn(%q) = %d paths, want %d", text, got, want)
		}
	}
}
