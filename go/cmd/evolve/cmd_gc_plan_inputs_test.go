package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const channelPolicyJSON = `{"gc":{"mode":"enforce","logs_ttl_days":9},"events":{"channels":{"custom":{"route":"module=loop","max_total_mb":3},"ship":{"ttl_days":2}}}}`

func projectWithPolicy(t *testing.T, raw string) (string, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.PolicyPath(evolveDir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func TestGCPlanInputsOf_LoopAndGCVerbShareOneMapping(t *testing.T) {
	t.Parallel()
	root, evolveDir := projectWithPolicy(t, channelPolicyJSON)
	var stderr bytes.Buffer

	fromLoop, mode, ok := loopGCMode(loopConfig{EvolveDir: evolveDir}, &stderr)
	_, pol, _, loaded := newGCRun(context.Background(), true, &stderr, &stderr).loadFullPolicy(root)

	if !ok || !loaded || mode != "enforce" {
		t.Fatalf("loopGCMode ok=%v mode=%q, loadFullPolicy ok=%v, want both to load the file", ok, mode, loaded)
	}
	if want := gcPlanInputsOf(pol); !reflect.DeepEqual(fromLoop, want) {
		t.Errorf("loop inputs =\n%+v\nwant the same mapping as the gc verb:\n%+v", fromLoop, want)
	}
	if fromLoop.Policy.LogsTTLDays != 9 {
		t.Errorf("gc policy = %+v, want logs_ttl_days 9", fromLoop.Policy)
	}
	names := map[string]int{}
	for _, c := range fromLoop.Channels {
		names[c.Name] = c.TTLDays
	}
	if ttl, ok := names["custom"]; !ok || names["ship"] != 2 || ttl != 0 {
		t.Errorf("channels = %v, want the custom channel and the ship ttl 2 from policy.json", names)
	}
}

func TestGCPlanInputsOf_AnEmptyPolicyGivesTheZeroGCPolicyAndTheDefaultChannels(t *testing.T) {
	t.Parallel()

	got := gcPlanInputsOf(policy.Policy{})

	if !reflect.DeepEqual(got.Policy, gc.Policy{}) || len(got.Channels) != len(policy.EventsDefaults().Channels) {
		t.Errorf("inputs = %+v, want the zero gc policy and %d default channels", got, len(policy.EventsDefaults().Channels))
	}
}

func TestGCRunDirs_PlansTheChannelSegmentsAndNamesTheEventsRule(t *testing.T) {
	t.Parallel()
	_, evolveDir := projectWithPolicy(t, `{"events":{"channels":{"ship":{"ttl_days":1}}}}`)
	dir := filepath.Join(paths.EventChannelsRoot(evolveDir), "ship")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-72 * time.Hour)
	for _, base := range []int64{0, 10} {
		p := filepath.Join(dir, fmt.Sprintf("seg-%020d.ndjson", base))
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), true, &stdout, &stderr).runDirs(evolveDir, gcPlanInputsOf(pol))

	first := filepath.Join(dir, fmt.Sprintf("seg-%020d.ndjson", 0))
	want := fmt.Sprintf("WOULD-DELETE %s (events.channels[\"ship\"].ttl_days)", first)
	if failed || !strings.Contains(stdout.String(), want) {
		t.Errorf("failed=%v stdout=%q stderr=%q, want the line %q", failed, stdout.String(), stderr.String(), want)
	}
	if strings.Contains(stdout.String(), fmt.Sprintf("seg-%020d.ndjson", 10)) {
		t.Errorf("stdout=%q, want the tail segment kept", stdout.String())
	}
}

func TestGCProject_PrintsTheEventsRouteDiagnosticsOnce(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	withGCProcessHost(t, &fakeGCProcessHost{})
	raw := `{"events":{"channels":{"ship":{"route":"kind=no.such.kind"}}}}`
	if err := os.WriteFile(filepath.Join(projectRoot, ".evolve", "policy.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	newGCRun(context.Background(), true, &stdout, &stderr).project(projectRoot)

	if n := strings.Count(stderr.String(), `ship`); n < 1 || strings.Count(stderr.String(), "no.such.kind") != 1 {
		t.Errorf("stderr = %q, want the ship route diagnostic exactly once", stderr.String())
	}
}
