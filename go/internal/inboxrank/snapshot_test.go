package inboxrank_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

var updateOrderGolden = flag.Bool("update", false, "rewrite testdata/order-2026-10-06.golden from the snapshot")

func loadSnapshot(t *testing.T) []inboxbatch.Item {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "inbox-snapshot-2026-10-06.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var items []inboxbatch.Item
	lines := bufio.NewScanner(bytes.NewReader(raw))
	lines.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for lines.Scan() {
		var line struct {
			Path string          `json:"path"`
			Item json.RawMessage `json:"item"`
		}
		var it inboxbatch.Item
		if err := json.Unmarshal(lines.Bytes(), &line); err != nil {
			t.Fatalf("snapshot line %d: %v", len(items)+1, err)
		}
		if err := json.Unmarshal(line.Item, &it); err != nil {
			t.Fatalf("snapshot item %s: %v", line.Path, err)
		}
		it.Path = line.Path
		items = append(items, it)
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return items
}

func renderOrder(ranked []inboxrank.Ranked) string {
	var b strings.Builder
	for _, r := range ranked {
		fmt.Fprintf(&b, "%3d %.6f %s\n", r.Rank, r.Breakdown.Score, r.Item.ID)
	}
	return b.String()
}

func TestOrder_PinsTheSnapshotOrder(t *testing.T) {
	items := loadSnapshot(t)
	if len(items) != 241 {
		t.Fatalf("snapshot holds %d items, want the 241 pending on 2026-10-06", len(items))
	}

	got := renderOrder(inboxrank.Order(items, defaults(), ctxFor(items...)))

	golden := filepath.Join("testdata", "order-2026-10-06.golden")
	if *updateOrderGolden {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (regenerate with -update, on purpose)", err)
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		if g, w := lineAt(gotLines, i), lineAt(wantLines, i); g != w {
			t.Fatalf("the snapshot order drifted from %s at line %d:\n got: %s\nwant: %s", golden, i+1, g, w)
		}
	}
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<missing>"
}

func TestOrder_SpreadsTheSnapshotsWeightTies(t *testing.T) {
	items := loadSnapshot(t)
	tiedAtHalf := 0
	scores := map[string]int{}
	for _, r := range inboxrank.Order(items, defaults(), ctxFor(items...)) {
		if r.Item.Weight == 0.5 {
			tiedAtHalf++
			scores[fmt.Sprintf("%.6f", r.Breakdown.Score)]++
		}
	}

	if tiedAtHalf < 30 || len(scores) < tiedAtHalf/2 {
		t.Errorf("%d items share weight 0.5 but only %d distinct scores: the rank must spread the ties", tiedAtHalf, len(scores))
	}
}
