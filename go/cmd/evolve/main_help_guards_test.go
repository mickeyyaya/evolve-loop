package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
)

func TestUsage_ListsExactlyTheGuardsThatTheDispatcherRuns(t *testing.T) {
	t.Parallel()
	for _, line := range strings.Split(usage, "\n") {
		listed, ok := strings.CutPrefix(strings.TrimSpace(line), "Guards:")
		if !ok {
			continue
		}
		var got []string
		for _, name := range strings.Split(listed, "|") {
			got = append(got, strings.TrimSpace(name))
		}
		if !slices.Equal(got, guardcmd.Names) {
			t.Fatalf("help lists guards %v, the dispatcher runs %v", got, guardcmd.Names)
		}
		return
	}
	t.Fatal("the top-level help has no Guards line")
}
