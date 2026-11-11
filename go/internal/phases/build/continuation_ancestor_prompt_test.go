package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_ContinuationAncestorLineRendersWhenServed(t *testing.T) {
	line := "continuation of cycle-97: salvage 1111111111111111111111111111111111111111 ancestor-of-HEAD=false"
	req := core.PhaseRequest{Context: map[string]string{core.CtxKeyContinuationAncestry: line}}

	got := (hooks{}).ComposePrompt("body", req)

	if !strings.Contains(got, line) {
		t.Errorf("the build prompt must carry the served ancestry line verbatim; got:\n%s", got)
	}
}

func TestComposePrompt_ContinuationAncestorAbsentOrEmptyIsByteIdentical(t *testing.T) {
	absent := (hooks{}).ComposePrompt("body", core.PhaseRequest{Context: map[string]string{}})
	empty := (hooks{}).ComposePrompt("body", core.PhaseRequest{Context: map[string]string{core.CtxKeyContinuationAncestry: ""}})

	if absent != empty {
		t.Errorf("an empty ancestry value must render exactly like an absent one:\n--- absent ---\n%s\n--- empty ---\n%s", absent, empty)
	}
	if strings.Contains(absent, "ancestor-of-HEAD") {
		t.Errorf("a prompt with no served line must not mention the probe; got:\n%s", absent)
	}
}
