package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasAssistantEvents_NoAssistantEnvelopeIsFalse(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "builder-events.ndjson")
	if err := os.WriteFile(path, []byte(`{"kind":"tool_use"}`+"\n"+`{"kind":"result"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasAssistantEvents(path) {
		t.Fatal("a stream without an assistant_text envelope must not count as output present")
	}
}
