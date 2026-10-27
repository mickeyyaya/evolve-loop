package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestCanary_AnExpiredModelBenchIsNotProbedAsAFamily(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	past := time.Now().Add(-3 * time.Hour)
	store := clihealth.NewStore(root, func() time.Time { return past })
	if _, err := store.BenchModelUntil("claude", "Opus", clihealth.Wall{Pattern: clihealth.UsageProbePattern, Evidence: "Opus week 100% used", Reset: past.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var probed []string
	var out bytes.Buffer

	runCLIHealthCanary(context.Background(), root, func(driver string) (int, string, string) {
		probed = append(probed, driver)
		return 0, "", ""
	}, &out)

	if len(probed) != 0 || out.Len() != 0 {
		t.Fatalf("probed=%v output=%q; the canary treated the model bench as a family", probed, out.String())
	}
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 1 {
		t.Errorf("benches=%v; the canary must leave the model bench for the model probe", benches)
	}
}
