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

	present, routed := itemRouteState(inbox, "a", failing)

	if !present || routed {
		t.Fatalf("an unreadable located item: present=%v routed=%v, want present=true routed=false", present, routed)
	}
}

func TestItemRouteState_ReadsTheConsoleStampThroughTheRealReader(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"a","route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	present, routed := itemRouteState(inbox, "a", os.ReadFile)

	if !present || !routed {
		t.Fatalf("a console-stamped item: present=%v routed=%v, want both true", present, routed)
	}
}
