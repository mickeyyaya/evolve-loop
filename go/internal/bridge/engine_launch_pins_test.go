package bridge

// engine_launch_pins_test.go — ADR-0103 unit 10, the host pins written on
// the pre-extraction code (8e8f080f) and kept: the four required-field
// strings, the stdout-completion read path, the boot-strike clear on a
// non-80 exit, the exit → sentinel table, and the ACS source tokens engine.go
// must keep spelling. Each was proven red against its named mutant before
// the launch-outcome classifier moved out (design §6 tests 1-4, 8).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Test 1 — the gauntlet: CLI, Profile, Workspace, ArtifactPath in that order,
// each its exact string, and the Runner is never invoked.
func TestEngineLaunch_RequiredFields_FourSentinelStrings(t *testing.T) {
	full := core.BridgeRequest{CLI: "claude-p", Profile: "p", Workspace: "w", ArtifactPath: "a"}
	for _, tc := range []struct {
		name string
		zero func(r *core.BridgeRequest)
		want string
	}{
		{"CLI", func(r *core.BridgeRequest) { r.CLI = "" }, "bridge: CLI required"},
		{"Profile", func(r *core.BridgeRequest) { r.Profile = "" }, "bridge: Profile required"},
		{"Workspace", func(r *core.BridgeRequest) { r.Workspace = "" }, "bridge: Workspace required"},
		{"ArtifactPath", func(r *core.BridgeRequest) { r.ArtifactPath = "" }, "bridge: ArtifactPath required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := full
			tc.zero(&req)
			fr := &fakeRunner{}
			_, err := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)}).Launch(context.Background(), req)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("missing %s: err = %v, want %q", tc.name, err, tc.want)
			}
			if len(fr.calls) != 0 {
				t.Fatalf("the gauntlet rejects before any subprocess: %d calls", len(fr.calls))
			}
		})
	}
	// The ORDER: with every later field also missing, the first missing one is named.
	for _, tc := range []struct {
		req  core.BridgeRequest
		want string
	}{
		{core.BridgeRequest{}, "bridge: CLI required"},
		{core.BridgeRequest{CLI: "claude-p"}, "bridge: Profile required"},
		{core.BridgeRequest{CLI: "claude-p", Profile: "p"}, "bridge: Workspace required"},
		{core.BridgeRequest{CLI: "claude-p", Profile: "p", Workspace: "w"}, "bridge: ArtifactPath required"},
	} {
		if _, err := NewEngine(Deps{LookupEnv: mapLookup(nil)}).Launch(context.Background(), tc.req); err == nil || err.Error() != tc.want {
			t.Errorf("cumulative gauntlet: err = %v, want %q", err, tc.want)
		}
	}
}

// Test 2 — Completion == "stdout" reads the captured scrollback (stdoutLog),
// every other contract reads the artifact.
func TestEngineLaunch_StdoutCompletion_ReadsScrollbackNotArtifact(t *testing.T) {
	script := launchFixtureScript(ExitOK, "", "artifact=both")
	resp, _, err := launchThroughFixture(t, context.Background(), Deps{}, script, core.BridgeRequest{Completion: "stdout"})
	if err != nil || resp.Stdout != launchFixtureScrollbackBody {
		t.Fatalf("stdout completion reads the scrollback: err=%v stdout=%q", err, resp.Stdout)
	}
	resp, _, err = launchThroughFixture(t, context.Background(), Deps{}, script, core.BridgeRequest{})
	if err != nil || resp.Stdout != launchFixtureArtifactBody {
		t.Fatalf("the artifact contract reads the artifact: err=%v stdout=%q", err, resp.Stdout)
	}
}

// Test 3 — any exit other than 80 clears the driver's boot strike (the REPL
// booted); exit 80 records one more.
func TestEngineLaunch_NonBootExit_ClearsBootStrike(t *testing.T) {
	seeded := func() *clihealth.Store {
		store := clihealth.NewStore(t.TempDir(), nil)
		if _, err := store.RecordBootStrike(launchOutcomeFixtureCLI); err != nil {
			t.Fatal(err)
		}
		return store
	}
	strikes := func(store *clihealth.Store) int {
		benches, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		return benches[launchOutcomeFixtureCLI].Strikes
	}
	store := seeded()
	if _, _, err := launchThroughFixture(t, context.Background(), Deps{BootTimeoutStore: store}, launchFixtureScript(ExitBadFlags, "empty"), core.BridgeRequest{}); err == nil {
		t.Fatal("exit 10 is an error")
	}
	if got := strikes(store); got != 0 {
		t.Fatalf("a non-80 exit clears the boot strike: strikes = %d", got)
	}
	store = seeded()
	_, _, _ = launchThroughFixture(t, context.Background(), Deps{BootTimeoutStore: store}, launchFixtureScript(ExitREPLBootTimeout, "empty"), core.BridgeRequest{})
	if got := strikes(store); got != 2 {
		t.Fatalf("exit 80 records a second strike: strikes = %d", got)
	}
}

// Test 4 — the exit → sentinel table through Launch: 81 wraps
// ErrArtifactTimeout; exactly {80, 85, 86, 124} and -1-under-cancel wrap
// ErrTransientBridgeFailure; everything else is plain; every error starts
// with "bridge: launch exit=<code>".
func TestEngineLaunch_ExitClassificationTable(t *testing.T) {
	transient := map[int]bool{80: true, 85: true, 86: true, 124: true}
	for _, code := range []int{2, 3, 10, 80, 81, 85, 86, 99, 124, 127, -1, 42} {
		for _, ctxState := range []string{"live", "cancelled"} {
			_, _, err := launchThroughFixture(t, launchCtx(ctxState), Deps{}, launchFixtureScript(code, "last-line-only"), core.BridgeRequest{})
			if err == nil {
				t.Fatalf("exit %d is an error", code)
			}
			wantTransient := transient[code] || (code == -1 && ctxState == "cancelled")
			if errors.Is(err, core.ErrArtifactTimeout) != (code == 81) {
				t.Errorf("exit %d (%s): ErrArtifactTimeout wraps exactly 81: %v", code, ctxState, err)
			}
			if errors.Is(err, core.ErrTransientBridgeFailure) != wantTransient {
				t.Errorf("exit %d (%s): transient = %v, want %v: %v", code, ctxState, !wantTransient, wantTransient, err)
			}
			if prefix := "bridge: launch exit=" + strconv.Itoa(code) + ":"; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("exit %d: the wire prefix %q, got %v", code, prefix, err)
			}
		}
	}
}

// Test 8 — the ACS predicates read engine.go's SOURCE: cycle426 pins the
// token ClearBootStrike, cycle50 pins `codexConfigPath string`, cycle43 pins
// the absence of the retired EVOLVE_ARTIFACT_MAX_EXTENDS dial. The unit's
// split must keep the boot-strike steps and Config in this file.
func TestEngineSourceKeepsTheACSTokens(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "engine.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"ClearBootStrike", "codexConfigPath string"} {
		if !strings.Contains(string(src), token) {
			t.Errorf("engine.go must keep spelling %q (an ACS source pin)", token)
		}
	}
	if strings.Contains(string(src), "EVOLVE_ARTIFACT_MAX_EXTENDS") {
		t.Error("engine.go must not reintroduce EVOLVE_ARTIFACT_MAX_EXTENDS (acs/cycle43)")
	}
}
