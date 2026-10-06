package observer

import (
	"slices"
	"testing"
)

func TestTmuxPaneProbe_CapturesOnlyTheExactListedSession(t *testing.T) {
	t.Parallel()
	const listed = "evolve-bridge-c7-build-pid3-1"
	var capture []string
	run := func(args ...string) ([]byte, error) {
		if args[0] == "ls" {
			return []byte(listed + "\n"), nil
		}
		capture = args
		return []byte("pane"), nil
	}

	newTmuxPaneProbe(7, "build", "", run)()

	if i := slices.Index(capture, "-t"); i < 0 || i+1 >= len(capture) || capture[i+1] != "="+listed+":" {
		t.Fatalf("capture-pane args = %v; a session that ends between ls and capture must never resolve to a longer name", capture)
	}
}
