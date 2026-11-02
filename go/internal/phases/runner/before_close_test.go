package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_TheBeforeCloseCheckReachesTheBridgeRequest(t *testing.T) {
	var check core.CloseCheck = func(context.Context) (string, func(error)) { return "floor findings", nil }
	got := dispatchedBridgeRequest(t, core.PhaseRequest{BeforeClose: check})
	if got.BeforeClose == nil {
		t.Fatal("the bridge request lost the check that runs before the session closes")
	}
	if findings, _ := got.BeforeClose(context.Background()); findings != "floor findings" {
		t.Fatal("the bridge request lost the check that runs before the session closes")
	}
	if plain := dispatchedBridgeRequest(t, core.PhaseRequest{}); plain.BeforeClose != nil {
		t.Fatal("a dispatch with no check carries one")
	}
}
