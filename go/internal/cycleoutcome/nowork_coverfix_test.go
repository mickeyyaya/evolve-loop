package cycleoutcome

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestItemRouteState_ALocatedItemThatCannotBeReadIsPresentAndNotConsoleRouted(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"a","route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	failing := func(string) ([]byte, error) { return nil, errors.New("read raced away") }

	path, routed := itemRouteState(inbox, "a", failing)

	if path != filepath.Join(inbox, "a.json") || routed {
		t.Fatalf("an unreadable located item: path=%q routed=%v, want the located path and routed=false", path, routed)
	}
}

func TestItemRouteState_ReadsTheConsoleStampThroughTheRealReader(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"a","route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	path, routed := itemRouteState(inbox, "a", os.ReadFile)

	if path != filepath.Join(inbox, "a.json") || !routed {
		t.Fatalf("a console-stamped item: path=%q routed=%v, want the located path and routed=true", path, routed)
	}
}
