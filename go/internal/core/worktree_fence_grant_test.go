package core

import "testing"

func TestGrantFenceCarriesFilesAndDirectories(t *testing.T) {
	t.Parallel()
	f := GrantFence([]string{"a.go"}, []string{"gen"})

	if got := f.WritablePaths(); len(got) != 1 || got[0] != "a.go" {
		t.Errorf("WritablePaths = %v, want [a.go]", got)
	}
	if got := f.WritableDirPaths(); len(got) != 1 || got[0] != "gen" {
		t.Errorf("WritableDirPaths = %v, want [gen]", got)
	}
	if got := (*WorktreeFence)(nil).WritableDirPaths(); got != nil {
		t.Errorf("a nil fence grants no directory, got %v", got)
	}
	if !(PhaseRequest{Fence: GrantFence(nil, []string{"gen"})}).WritesSource() {
		t.Error("a directory grant alone is a writer")
	}
	if (PhaseRequest{Fence: GrantFence(nil, nil)}).WritesSource() {
		t.Error("an empty grant is read-only")
	}
}
