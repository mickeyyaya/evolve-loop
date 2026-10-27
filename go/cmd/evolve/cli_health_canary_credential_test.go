package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestCanaryCredentialWallRebenchesAndNamesTheOperator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "claude", 1)
	var out bytes.Buffer
	runCLIHealthCanary(context.Background(), root, func(driver string) (int, string, string) {
		return 85, clihealth.CredentialPattern, "Please log in to continue"
	}, &out)
	benches, _ := clihealth.NewStore(root, nil).Load()
	e, ok := benches["claude"]
	if !ok || e.Reason != clihealth.CredentialPattern || e.Strikes != 2 {
		t.Fatalf("credential wall must re-bench with a strike; got %+v (ok=%v)", e, ok)
	}
	if !strings.Contains(out.String(), clihealth.OperatorAction("claude", clihealth.CredentialPattern)) {
		t.Fatalf("the canary must name the operator's fix; stderr:\n%s", out.String())
	}
}
