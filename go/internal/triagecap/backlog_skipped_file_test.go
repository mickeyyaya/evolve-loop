package triagecap

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestReadRankedBacklog_NamesAnUnreadableInboxFileInTheLoopLog(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	fixtures.MustWrite(t, filepath.Join(inbox, "a.json"), `{"id":"readable","weight":0.5,"priority_class":"stability"}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "broken.json"), `{not json`)
	var log bytes.Buffer

	got := ids(readRankedBacklog(evolveDir, noProtectedSurface, time.Now(), &log))

	if len(got) != 1 || got[0] != "readable" || !strings.Contains(log.String(), "[triagecap] WARN skipped broken.json") {
		t.Fatalf("backlog = %v, log = %q; want the readable item and a WARN that names broken.json", got, log.String())
	}
}
