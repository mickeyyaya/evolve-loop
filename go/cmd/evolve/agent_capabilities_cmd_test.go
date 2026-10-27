package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

func TestAgentCapabilities_InACellPrintsOneEnvelopeWithTheCoreVerbSchemas(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := dispatchWith(dispatchGate{open: unheldStore(t, "enforce")}, []string{"agent", "capabilities", "--json"}, strings.NewReader(""), &stdout, &stderr)
	var env struct {
		Schema string                  `json:"schema"`
		OK     bool                    `json:"ok"`
		Data   agentclass.Capabilities `json:"data"`
	}
	if code != cliExitOK || json.Unmarshal(stdout.Bytes(), &env) != nil || stderr.Len() > 0 {
		t.Fatalf("agent capabilities --json in a cell = %d %q %q", code, stdout.String(), stderr.String())
	}
	if env.Schema != "evolve.agent.capabilities/1" || !env.OK || env.Data.DefaultClass != agentclass.Denied {
		t.Fatalf("envelope = %+v", env)
	}
	outputs := map[string]string{}
	for _, v := range env.Data.Verbs {
		outputs[v.Verb] = v.Output
	}
	if outputs["wave status"] != "evolve.wave.status/1" || outputs["code find"] != "text/1" {
		t.Fatalf("an A core verb names its envelope schema and another A verb names text/1, got %v", outputs)
	}
}

func TestCoreVerbSchema_IsTheEnvelopeSchemaOfACoreVerbOnly(t *testing.T) {
	t.Parallel()
	if got := coreVerbSchema("wave next"); got != "evolve.wave.next/2" {
		t.Errorf("coreVerbSchema(wave next) = %q", got)
	}
	if got := coreVerbSchema("code find"); got != "" {
		t.Errorf("coreVerbSchema(code find) = %q, want none", got)
	}
}
