package triagecap

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestReadRankedBacklog_AtEnforceTheWaveSeedLeavesOutALaneItemWithAConsoleCompanion(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"workflow": {"repair": {"fixability": {"stage": "enforce"}}}}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "a.json"), `{"id":"x-lane","weight":0.9,"priority_class":"stability"}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "b.json"), `{"id":"x-console","weight":0.5,"priority_class":"stability","route":"console-manual"}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "c.json"), `{"id":"free","weight":0.5,"priority_class":"stability"}`)
	var log bytes.Buffer

	got := ids(readRankedBacklog(evolveDir, noProtectedSurface, time.Now(), &log))

	if !slices.Equal(got, []string{"free"}) {
		t.Errorf("backlog = %v, want only the free item: x-lane waits on x-console\n%s", got, log.String())
	}
}
