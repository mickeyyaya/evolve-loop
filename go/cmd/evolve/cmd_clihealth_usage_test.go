package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

func installUsagePanes(t *testing.T, panes map[string]string, errs map[string]error) *[]string {
	t.Helper()
	prev := clihealthUsagePaneFn
	var probed []string
	clihealthUsagePaneFn = func(string) (func(context.Context, string) (string, error), func()) {
		return func(_ context.Context, family string) (string, error) {
			probed = append(probed, family)
			if err := errs[family]; err != nil {
				return "", err
			}
			return panes[family], nil
		}, func() {}
	}
	t.Cleanup(func() { clihealthUsagePaneFn = prev })
	return &probed
}

func TestClihealthUsage_PrintsEachCLIsTypedWindowsAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	probed := installUsagePanes(t,
		map[string]string{"claude": usageFixture(t, "claude_usage_fable_exhausted.txt"), "agy": usageFixture(t, "agy_usage_claude_drained.txt")},
		map[string]error{"ollama": fmt.Errorf("no usage control: %w", clicontrol.ErrUnsupported)})

	out, errOut, code := runDispatch("clihealth", "usage", "--json", "--project-root", root, "claude", "agy", "ollama")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !reflect.DeepEqual(*probed, []string{"claude", "agy", "ollama"}) {
		t.Errorf("probed %v", *probed)
	}
	var readouts []usageReadout
	if err := json.Unmarshal([]byte(out), &readouts); err != nil || len(readouts) != 3 {
		t.Fatalf("readouts = %v (err %v) from %s", readouts, err, out)
	}
	fable := readouts[0].Windows[2]
	if readouts[0].CLI != "claude" || fable.Model != "Fable" || !fable.Exhausted || fable.Family != "claude" {
		t.Errorf("claude readout = %+v", readouts[0])
	}
	if drained := readouts[1].Windows[3]; readouts[1].CLI != "agy" || drained.Family != "agy-claude" || !drained.Exhausted {
		t.Errorf("agy readout = %+v", readouts[1])
	}
	if readouts[2].CLI != "ollama" || readouts[2].Note == "" || readouts[2].Error != "" {
		t.Errorf("ollama readout = %+v; a CLI with no usage command is a note, not an error", readouts[2])
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the read-only view wrote under .evolve (stat err %v)", err)
	}
}

func TestClihealthUsage_TheTableNamesEveryWindowAndAFailedProbeExitsOne(t *testing.T) {
	installUsagePanes(t,
		map[string]string{"claude": usageFixture(t, "claude_usage_2.1.291.txt")},
		map[string]error{"codex": errors.New("boot timed out")})

	out, errOut, code := runDispatch("clihealth", "usage", "--project-root", t.TempDir(), "claude", "codex")

	if code != 1 || !strings.Contains(errOut, "codex") || !strings.Contains(errOut, "boot timed out") {
		t.Fatalf("exit %d, stderr %q; a failed probe is named and exits 1", code, errOut)
	}
	for _, want := range []string{"CLI", "SCOPE", "claude", "session", "all models", "Fable", "7%", "64%", "Oct 11 at 9pm"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table lacks %q:\n%s", want, out)
		}
	}
}
