package triagecap

// wave_seed_consoleroute_test.go — batch-7 wave-0 pin: the inbox seed must
// skip console-routed items and backfill from the next dispatchable
// candidates. The raw top-N seeded exclusively console items, the plan-time
// gate rightly refused them all, and the wave "planned zero lanes" —
// starvation by correct refusal.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInboxBacklog_SkipsConsoleRoutedAndBackfills(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"id":"console-top","weight":0.96,"route":"console-salvage"}`)
	write("b.json", `{"id":"lane-next","weight":0.88}`)
	write("c.json", `{"id":"protected-derived","weight":0.90,"files":["go/internal/guards/role.go"]}`)
	write("d.json", `{"id":"lane-second","weight":0.85}`)

	got := ReadInboxBacklog(evolveDir, func(p string) bool { return p == "go/internal/guards/role.go" })
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if ids["console-top"] || ids["protected-derived"] {
		t.Fatalf("console-routed items must never enter the seed backlog, got %v", got)
	}
	if !ids["lane-next"] || !ids["lane-second"] {
		t.Fatalf("dispatchable items must backfill the seed, got %v", got)
	}
}
