package triagecap

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestReadRankedBacklog_NamesEveryUnclassedQueuedItemInTheLoopLog(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	fixtures.MustWrite(t, filepath.Join(inbox, "a.json"), `{"id":"classed","weight":0.5,"priority_class":"stability"}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "b.json"), `{"id":"unclassed-ready","weight":0.5}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "c.json"), `{"id":"unclassed-waiting","weight":0.5,"priority_class":"urgent","deps":["classed"]}`)
	var log bytes.Buffer
	got := ids(readRankedBacklog(evolveDir, noProtectedSurface, time.Now(), &log))
	if len(got) != 2 || got[0] != "classed" {
		t.Errorf("backlog = %v, want the classed item first and the unclassed one below it", got)
	}
	for _, id := range []string{"unclassed-ready", "unclassed-waiting"} {
		if !strings.Contains(log.String(), "[triagecap] WARN inbox rank: "+id+": unknown priority_class") {
			t.Errorf("the loop log must name %s, ranked below every class:\n%s", id, log.String())
		}
	}
	if strings.Contains(log.String(), "WARN inbox rank: classed:") {
		t.Errorf("a classed item is not warned:\n%s", log.String())
	}
}

func TestReadRankedBacklog_AnUnreadableInboxIsWarnedAndEmpty(t *testing.T) {
	evolveDir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(evolveDir, "inbox"), "not a directory")
	var log bytes.Buffer
	if got := readRankedBacklog(evolveDir, noProtectedSurface, time.Now(), &log); len(got) != 0 || !strings.Contains(log.String(), "[triagecap] WARN inbox backlog unreadable:") {
		t.Errorf("backlog = %v, log = %q", got, log.String())
	}
}
