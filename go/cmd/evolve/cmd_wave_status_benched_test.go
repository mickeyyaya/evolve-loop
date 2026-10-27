package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestWaveStatus_ShowsTheFamiliesBenchedAtLaunch(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	store := clihealth.NewStore(p.root, nil)
	now := waveTestClock
	until := now.Add(24 * time.Hour).Truncate(time.Second)
	for _, e := range []clihealth.Entry{
		{Family: "agy-claude", Reason: clihealth.UsageProbePattern, BenchedAt: now, BenchedUntil: until},
		{Family: "agy", Reason: clihealth.UsageProbePattern, BenchedAt: now, BenchedUntil: until},
		{Family: "codex", Reason: "exhausted", BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour)},
	} {
		if err := store.Bench(e); err != nil {
			t.Fatal(err)
		}
	}

	text := newWaveHarness()
	text.run("status", "--project-root", p.root)
	js := newWaveHarness()
	js.run("status", "--json", "--project-root", p.root)

	out := text.stdout.String()
	want := "benched: agy usage_probe until " + until.Format(time.RFC3339)
	if !strings.Contains(out, want) || !strings.Contains(out, "benched: agy-claude usage_probe until") || strings.Contains(out, "benched: codex") {
		t.Errorf("text:\n%s\nwant %q, agy-claude, and no expired codex bench", out, want)
	}
	if first, second := strings.Index(out, "benched: agy usage_probe"), strings.Index(out, "benched: agy-claude"); first < 0 || second < 0 || first > second {
		t.Errorf("text:\n%s\nwant the benches sorted by key", out)
	}
	var got waveStatusReport
	if err := json.Unmarshal(js.stdout.Bytes(), &got); err != nil || len(got.Benched) != 2 || got.Benched[0].Family != "agy" || got.Benched[1].Family != "agy-claude" {
		t.Errorf("json benched = %+v, %v, want agy then agy-claude", got.Benched, err)
	}
}
