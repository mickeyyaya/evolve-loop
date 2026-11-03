package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCloudList_InACellPrintsOneEnvelopeOfTheCloudListSchema(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := dispatchWith(dispatchGate{open: unheldStore(t, "enforce")}, []string{"cloud", "list", "--json", "--project-root", t.TempDir()}, strings.NewReader(""), &stdout, &stderr)
	var env struct {
		Schema string            `json:"schema"`
		OK     bool              `json:"ok"`
		Data   []json.RawMessage `json:"data"`
	}
	if code != cliExitOK || json.Unmarshal(stdout.Bytes(), &env) != nil {
		t.Fatalf("cloud list --json = %d %q %q", code, stdout.String(), stderr.String())
	}
	if env.Schema != "evolve.cloud.list/1" || !env.OK || len(env.Data) != 0 {
		t.Errorf("envelope = %+v, want an empty evolve.cloud.list/1 list", env)
	}
}

func TestRunCloudCommand_AnUnknownSubcommandIsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runCloudCommand([]string{"launch"}, strings.NewReader(""), &stdout, &stderr); code != cliExitUsage || !strings.Contains(stderr.String(), "evolve cloud list") {
		t.Errorf("cloud launch = %d %q; want usage, because no verb starts a cloud session", code, stderr.String())
	}
}

func TestAgentRunDeps_TheCloudSeatIsTheManifestThatDeclaresACloudLaunch(t *testing.T) {
	t.Parallel()
	seat := agentRunDeps().CloudSeat
	if seat == nil || !seat("claude-cloud") || seat("claude-tmux") {
		t.Errorf("agentRunDeps().CloudSeat does not tell the cloud seat from claude-tmux")
	}
}
