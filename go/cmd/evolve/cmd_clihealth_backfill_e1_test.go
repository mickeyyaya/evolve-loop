package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestRunClihealth_AnUnknownFlagExits2(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runClihealth([]string{"list", "--bogus"}, nil, &out, &errb)

	if code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -bogus") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 2 and the flag error", code, out.String(), errb.String())
	}
}

func TestClihealthClear_AnUnwritableStoreIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	store := clihealth.NewStore(root, func() time.Time { return time.Now().Add(-3 * time.Hour) })
	if _, ok := store.Active()["codex"]; !ok {
		t.Fatal("fixture: the bench must read as active at the store's clock")
	}
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })
	var errb bytes.Buffer

	code := clihealthClear(store, "codex", &errb)

	if code != 1 || !strings.HasPrefix(errb.String(), "evolve clihealth: ") || strings.Contains(errb.String(), "is not benched") {
		t.Errorf("code=%d stderr=%q, want 1 and the write error", code, errb.String())
	}
	_ = os.Chmod(evolveDir, 0o755)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 1 {
		t.Errorf("benches = %v, want the bench kept after a failed clear", benches)
	}
}

func TestProductionUsagePanes_AWorkspaceThatCannotBeMadeFailsEveryProbe(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	probe, cleanup := productionUsagePanes(t.TempDir())
	defer cleanup()
	pane, err := probe(context.Background(), "claude")

	if pane != "" || err == nil || !strings.HasPrefix(err.Error(), "usage workspace: ") {
		t.Errorf("probe = (%q, %v), want the workspace error", pane, err)
	}
}

func TestProductionUsagePanes_ProbesThroughTheBridgeAndCleansItsWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	probe, cleanup := productionUsagePanes(t.TempDir())
	made, _ := filepath.Glob(filepath.Join(tmp, "evolve-clihealth-usage-*"))
	_, err := probe(context.Background(), "ollama")
	cleanup()
	left, _ := filepath.Glob(filepath.Join(tmp, "evolve-clihealth-usage-*"))

	if !errors.Is(err, clicontrol.ErrUnsupported) {
		t.Errorf("probe(ollama) err = %v, want ErrUnsupported from its manifest", err)
	}
	if len(made) != 1 || len(left) != 0 {
		t.Errorf("workspaces made=%v left=%v, want one made and none left", made, left)
	}
}

func TestClihealthUsage_NoFamilyNamedProbesEveryInstalledOne(t *testing.T) {
	usageProbeFakePATH(t, "ollama")
	probed := installUsagePanes(t, nil, map[string]error{"ollama": clicontrol.ErrUnsupported})
	var out, errb bytes.Buffer

	code := clihealthUsage(usageView{projectRoot: t.TempDir()}, &out, &errb)

	if code != 0 || len(*probed) != 1 || (*probed)[0] != "ollama" {
		t.Errorf("code=%d probed=%v stderr=%q, want only the installed ollama probed", code, *probed, errb.String())
	}
	if !strings.Contains(out.String(), "ollama  no_collector (its manifest maps no usage command)") {
		t.Errorf("stdout = %q, want the note row", out.String())
	}
}

func TestReadUsage_ACLIThatDeclaresWindowsAndReadsNoneIsAnError(t *testing.T) {
	t.Parallel()
	r := readUsage(context.Background(), func(context.Context, string) (string, error) { return "nothing here", nil }, "claude")

	if r.CLI != "claude" || r.Note != "" || r.State != clicontrol.UsageUnparsed || len(r.Windows) != 0 || !strings.HasPrefix(r.Error, "no usage window read") {
		t.Errorf("readout = %+v; claude declares controls.usage.windows, so a screen with none is an error", r)
	}
}

func TestReadUsage_ACLIThatDeclaresNoWindowsGetsANote(t *testing.T) {
	t.Parallel()
	r := readUsage(context.Background(), func(context.Context, string) (string, error) { return "nothing here", nil }, "codex")

	if r.Error != "" || len(r.Windows) != 0 || r.State != clicontrol.UsageNoCollector || !strings.HasPrefix(r.Note, "its manifest declares no controls.usage.windows") {
		t.Errorf("readout = %+v, want no windows, the state no_collector and the no-declaration note", r)
	}
}

func TestPrintUsageReadouts_TextRowsWindowsThenNotes(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	printUsageReadouts([]usageReadout{
		{Observation: clihealthObservation("claude", quotastate.UsageWindow{Scope: "session", Kind: "percent", PercentUsed: 42, ResetsText: "5pm", Family: "claude"})},
		{Note: "no usage window read", Observation: clihealthObservation("agy")},
	}, false, &out)

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "CLI") {
		t.Fatalf("stdout = %q, want a header, one window row and one note row", out.String())
	}
	if f := strings.Fields(lines[1]); strings.Join(f, " ") != "claude session percent 42% 5pm claude false false" {
		t.Errorf("window row = %q", lines[1])
	}
	if strings.Join(strings.Fields(lines[2]), " ") != "agy (no usage window read)" {
		t.Errorf("note row = %q", lines[2])
	}
}

func TestWindowTarget(t *testing.T) {
	t.Parallel()
	cases := map[string]quotastate.UsageWindow{
		"(none)":                {},
		"(none) ":               {Model: "opus"},
		"claude":                {Family: "claude"},
		"agy-claude/sonnet-4.6": {Family: "agy-claude", Model: "sonnet-4.6"},
	}
	for want, w := range cases {
		if got := windowTarget(w); got != strings.TrimSpace(want) {
			t.Errorf("windowTarget(%+v) = %q, want %q", w, got, strings.TrimSpace(want))
		}
	}
}

func clihealthObservation(cli string, windows ...quotastate.UsageWindow) usageprobe.Observation {
	return usageprobe.Observation{CLI: cli, Windows: windows}
}
