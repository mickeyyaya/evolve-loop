package bridge

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestCellEnv_EachStageGivesTheChildTheDepthAndPhaseMarkers(t *testing.T) {
	t.Setenv(ipcenv.DispatchDepthKey, "")
	for _, stage := range []string{"off", "shadow", "enforce"} {
		env := dispatchEnv(Deps{EnvStage: stage}, seatConfig("claude-p"))
		for _, want := range []string{ipcenv.DispatchDepthKey + "=1", ipcenv.PhaseKey + "=build"} {
			if !slices.Contains(env, want) {
				t.Errorf("stage %s: the child misses %s, so the refusals of a nested dispatch do not fire; env names %v", stage, want, envNames(env))
			}
		}
	}
}

func TestCellEnv_AChildIsOneLevelDeeperThanItsLauncher(t *testing.T) {
	t.Setenv(ipcenv.DispatchDepthKey, "2")
	env := dispatchEnv(Deps{EnvStage: "enforce"}, seatConfig("codex"))
	if !slices.Contains(env, ipcenv.DispatchDepthKey+"=3") || slices.Contains(env, ipcenv.DispatchDepthKey+"=2") {
		t.Fatalf("a launcher at depth 2 must give its child depth 3 only; env %v", env)
	}
}

func TestCellMarkers_ASeatWithNoAgentGetsTheDepthOnly(t *testing.T) {
	t.Parallel()
	markers := Deps{Env: map[string]string{ipcenv.DispatchDepthKey: "x"}}.cellMarkers(&Config{})
	if len(markers) != 1 || markers[ipcenv.DispatchDepthKey] != "1" {
		t.Fatalf("cellMarkers = %v, want only %s=1", markers, ipcenv.DispatchDepthKey)
	}
}

func TestPrimePaneShell_ExportsTheMarkersIntoAPaneThatInheritsTheServerEnv(t *testing.T) {
	t.Setenv(ipcenv.DispatchDepthKey, "")
	tmux := &fakeTmux{}
	deps := Deps{Tmux: tmux, Stderr: &bytes.Buffer{}}
	if err := primePaneShell(context.Background(), deps, &Config{Agent: "scout"}, "s", "/wt"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"export " + ipcenv.DispatchDepthKey + "=1", "export " + ipcenv.PhaseKey + "=scout"} {
		if !slices.Contains(tmux.sentKeys, want) {
			t.Errorf("pane keys = %q, want %q", tmux.sentKeys, want)
		}
	}
}

type launchCountingDriver struct{ launches int }

func (d *launchCountingDriver) Name() string { return "counting" }

func (d *launchCountingDriver) Launch(context.Context, *Config, Deps) (int, error) {
	d.launches++
	return ExitOK, nil
}

func TestLaunchSwept_RefusesSandboxOffInsideAPhaseWithANamedErrorAndASignal(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	driver := &launchCountingDriver{}
	deps := Deps{Signals: center, SandboxMode: " off ", LookupEnv: mapLookup(map[string]string{ipcenv.PhaseKey: "build"})}
	cfg := &Config{CLI: "claude-tmux", Agent: "scout", Cycle: 4}

	rc, err := launchSwept(context.Background(), driver, cfg, deps)

	if !errors.Is(err, ErrSandboxOffInPhase) || rc != ExitSafetyGate || driver.launches != 0 {
		t.Fatalf("launchSwept = (%d, %v), launches %d; want (%d, ErrSandboxOffInPhase) and no launch", rc, err, driver.launches, ExitSafetyGate)
	}
	var got []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == isolation.ViolationCode(3) {
			got = append(got, e)
		}
	}
	if len(got) != 1 || got[0].Fields["action"] != "refused" || got[0].Fields["phase"] != "build" || got[0].Fields["cell"] != "scout" || got[0].Fields["target"] != "--sandbox-off" || got[0].Cycle != 4 {
		t.Fatalf("want one %s with action=refused for the cell scout launched from the phase build; got %+v", isolation.ViolationCode(3), got)
	}
}

func TestRefuseSandboxOffInPhase_AllowsTheOptOutOutsideAPhaseAndAnyOtherModeInside(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		mode string
		env  map[string]string
	}{
		"off outside a phase":  {"off", nil},
		"auto inside a phase":  {"auto", map[string]string{ipcenv.PhaseKey: "build"}},
		"unset inside a phase": {"", map[string]string{ipcenv.PhaseKey: "build"}},
	} {
		deps := Deps{SandboxMode: tc.mode, Env: tc.env, LookupEnv: mapLookup(nil)}
		if err := deps.refuseSandboxOffInPhase(&Config{Agent: "scout"}); err != nil {
			t.Errorf("%s: refused with %v", name, err)
		}
	}
}

func TestRecipeSession_ExportsTheMarkersBeforeTheLaunch(t *testing.T) {
	t.Setenv(ipcenv.DispatchDepthKey, "")
	tx := &fakeTmux{paneSeq: []string{"ready\n❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "x4-sess")
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	depth, phase, launch := slices.Index(tx.sentKeys, "export "+ipcenv.DispatchDepthKey+"=1"), slices.Index(tx.sentKeys, "export "+ipcenv.PhaseKey+"=recipe"), slices.Index(tx.sentKeys, d.launchCmd)
	if depth < 0 || phase < 0 || launch < depth || launch < phase {
		t.Fatalf("the markers must reach the pane before the launch line; keys %q", tx.sentKeys)
	}
}
