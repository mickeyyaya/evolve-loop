package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestCanary_CancelledContextTouchesNoBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	probed := 0
	runCLIHealthCanary(ctx, root, nil, func(string) (int, string, string) { probed++; return 1, "cancelled", "" }, &out)
	if probed != 0 {
		t.Errorf("a cancelled context launches no probe, got %d", probed)
	}
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 1 {
		t.Fatalf("the expired bench must survive an interrupt untouched, got %v", benches)
	}
	if !strings.Contains(out.String(), "cancelled") {
		t.Errorf("the skip is one readable line: %q", out.String())
	}
}
