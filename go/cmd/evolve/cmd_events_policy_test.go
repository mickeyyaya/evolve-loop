package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestEventsChannels_ListsTheChannelsOfPolicyJSONAndPrintsTheRouteDiagnosticsOnce(t *testing.T) {
	t.Parallel()
	root, _ := projectWithPolicy(t, `{"events":{"channels":{"custom":{"route":"module=loop","segment_mb":2},"ship":{"route":"kind=no.such.kind"},"cycle":{"route":"code=NO_SUCH_CODE_Y"}}}}`)
	var stdout, stderr bytes.Buffer

	rc := runEvents([]string{"channels", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got struct {
		Channels []eventChannelRow `json:"channels"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || rc != cliExitOK {
		t.Fatalf("rc = %d err = %v stdout = %s", rc, err, stdout.String())
	}
	seen := map[string]eventChannelRow{}
	for _, c := range got.Channels {
		seen[c.Name] = c
	}
	if c, ok := seen["custom"]; !ok || c.SegmentMB != 2 {
		t.Errorf("custom = %+v, want the channel of policy.json with segment_mb 2", seen["custom"])
	}
	if seen["ship"].Route != "kind=ship.*" {
		t.Errorf("ship route = %q, want the compiled route kept after the refusal", seen["ship"].Route)
	}
	if n := strings.Count(stderr.String(), "\n"); n != 2 || !strings.Contains(stderr.String(), "ship") || !strings.Contains(stderr.String(), "cycle") {
		t.Errorf("stderr = %q, want one line each for ship and cycle", stderr.String())
	}
}

func TestEventsChannels_AFailedPolicyLoadListsTheCompiledDefaultsWithOneWarning(t *testing.T) {
	t.Parallel()
	root, _ := projectWithPolicy(t, `{"events":{"nope":1}}`)
	var stdout, stderr bytes.Buffer

	rc := runEvents([]string{"channels", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitOK || !strings.Contains(stdout.String(), "signals") || strings.Count(stderr.String(), "\n") != 1 || !strings.Contains(stderr.String(), "compiled defaults apply") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want the defaults and one warning line", rc, stdout.String(), stderr.String())
	}
}

func TestEventsWatch_ResolvesTheChannelsOfPolicyJSONWhenTheEnvCarriesNone(t *testing.T) {
	t.Parallel()
	root, _ := projectWithPolicy(t, `{"events":{"channels":{"custom":{"route":"module=loop"}}}}`)
	appendEvents(t, root, "custom", loopEvent(1, signalcenter.KindLoopExit))
	env := testWatchEnv(&eventsTestWaiter{}, context.Background())
	env.events.Channels = nil

	rc, out, errOut := runTestWatch(t, env, root, "--channel", "custom", "--since", "all", "--until", "kind=loop.exit")

	if rc != cliExitOK || !strings.Contains(out, "custom") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want the custom channel of policy.json to resolve", rc, out, errOut)
	}
}
