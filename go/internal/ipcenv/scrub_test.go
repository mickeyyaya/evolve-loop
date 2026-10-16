package ipcenv_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestScrub_DropsEveryEvolveKeyAndKeepsTheToolchain(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		ipcenv.FleetKey + "=1",
		"HOME=/Users/lane",
		ipcenv.CycleStateFileKey + "=/runs/cycle-1677/cycle-state.json",
		"GOFLAGS=-count=1",
		"EVOLVE_TMUX_SOCKET=/tmp/evolve-13934.sock",
		"FOO=EVOLVE_BAR",
		"GOTOOLCHAIN=auto",
	}
	want := []string{"PATH=/usr/bin", "HOME=/Users/lane", "GOFLAGS=-count=1", "FOO=EVOLVE_BAR", "GOTOOLCHAIN=auto"}
	if got := ipcenv.Scrub(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("Scrub =\n  %q\nwant\n  %q", got, want)
	}
	if got := ipcenv.Scrub(nil); len(got) != 0 {
		t.Fatalf("Scrub(nil) = %q, want empty", got)
	}
}

func TestScrub_CoversEveryIPCKey(t *testing.T) {
	keys := []string{ipcenv.FleetKey, ipcenv.FleetScopeKey, ipcenv.FleetWidthKey, ipcenv.WorktreeRootKey, ipcenv.CycleStateFileKey}
	for _, k := range keys {
		if !strings.HasPrefix(k, "EVOLVE_") {
			t.Errorf("%q is outside the EVOLVE_ namespace — Scrub would let it through", k)
		}
		if got := ipcenv.Scrub([]string{k + "=x"}); len(got) != 0 {
			t.Errorf("Scrub kept %q", got)
		}
	}
}
