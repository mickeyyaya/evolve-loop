package fanoutdispatch

import (
	"path/filepath"
	"testing"
)

// TestReadCommands_ParsesNameAndCommandFields pins the Command struct's parsing
// contract: ReadCommands splits each TSV row at the FIRST tab into
// Command.Name and Command.Command (the remainder, tabs preserved). Names the
// Command type and reads both exported fields — the existing suite only checks
// the slice length, never the field mapping.
func TestReadCommands_ParsesNameAndCommandFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "cmds.tsv")
	writeFile(t, p, "alpha\techo hi\nbeta\tsh -c 'a\tb'\n")

	got, err := ReadCommands(p)
	if err != nil {
		t.Fatalf("ReadCommands: %v", err)
	}
	want := []Command{
		{Name: "alpha", Command: "echo hi"},
		{Name: "beta", Command: "sh -c 'a\tb'"}, // split at FIRST tab only; later tabs stay in Command
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
