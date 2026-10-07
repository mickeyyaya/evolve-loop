package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func ids(cs []FleetCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func TestReadInboxBacklog_OnATieTheDeclaredSurfaceRanksFirst(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.json": `{"id":"placeholder","weight":0.5,"files":["TBD"]}`,
		"b.json": `{"id":"declared","weight":0.5,"files":["go/internal/inboxbatch/item.go"]}`,
	} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := ids(ReadInboxBacklog(dir, nil)); !reflect.DeepEqual(got, []string{"declared", "placeholder"}) {
		t.Errorf("backlog = %v, want the declared surface first on an equal score", got)
	}
}
