package fanoutdispatch

import (
	"path/filepath"
	"testing"
)

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
		{Name: "beta", Command: "sh -c 'a\tb'"},
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
