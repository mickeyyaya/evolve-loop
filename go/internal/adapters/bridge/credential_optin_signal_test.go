package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bypass"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestNewDefault_EachCredentialOptInThatIsOnEmitsOneSignal(t *testing.T) {
	dir := t.TempDir()
	writePolicyJson(t, dir, `{"bridge":{"allow_anthropic_base_url":true,"allow_openai_api_key":true}}`)
	center := signalcenter.New()
	NewDefault(dir, center)
	center.Flush()
	var names []string
	for _, e := range center.Recent() {
		if e.Code == bypass.CodeUsed && e.Fields["actor"] == "policy.json" {
			names = append(names, e.Fields["name"])
		}
	}
	if len(names) != 2 || names[0] != "bridge.allow_anthropic_base_url" || names[1] != "bridge.allow_openai_api_key" {
		t.Errorf("opt-in signals = %v, want one for each opt-in that is on", names)
	}
	quiet := signalcenter.New()
	NewDefault(t.TempDir(), quiet)
	quiet.Flush()
	for _, e := range quiet.Recent() {
		if e.Code == bypass.CodeUsed {
			t.Errorf("no opt-in must emit no %s; got %+v", bypass.CodeUsed, e)
		}
	}
}
