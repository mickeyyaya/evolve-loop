package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const shippedRoutesGolden = "testdata/cli_routing_shipped_routes.golden.json"

func shippedRoutes(t *testing.T) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := runCLIRouting([]string{"show", "--static", "--json", "--project-root", checkedInRoot(t)}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("cli-routing show over the checked-in tree exited %d: %s", code, stderr.String())
	}
	var report routingReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	for i := range report.Agents {
		for j := range report.Agents[i].Tiers {
			report.Agents[i].Tiers[j].Model = ""
		}
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestCLIRoutingShow_EveryShippedProfileKeepsItsResolvedRoute(t *testing.T) {
	got := shippedRoutes(t)
	if *updateAdvisorGolden {
		if err := os.WriteFile(filepath.FromSlash(shippedRoutesGolden), got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(filepath.FromSlash(shippedRoutesGolden))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the resolved route of a shipped profile changed; the floors and seat defaults must keep every route (rerun with -update only for an intended change):\n%s", got)
	}
}
