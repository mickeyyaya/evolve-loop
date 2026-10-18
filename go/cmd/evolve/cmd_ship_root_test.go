package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func stubShipRunFunc(t *testing.T, run func(ship.Options) (ship.RunResult, error)) *[]ship.Options {
	t.Helper()
	prev := shipRunFn
	t.Cleanup(func() { shipRunFn = prev })
	var calls []ship.Options
	shipRunFn = func(_ context.Context, opts ship.Options) (ship.RunResult, error) {
		calls = append(calls, opts)
		return run(opts)
	}
	return &calls
}

func stubShipRun(t *testing.T, res ship.RunResult, err error) *[]ship.Options {
	t.Helper()
	return stubShipRunFunc(t, func(ship.Options) (ship.RunResult, error) { return res, err })
}

func TestRunShipCmd_RefusesABadCommandLineBeforeItRunsShip(t *testing.T) {
	calls := stubShipRun(t, ship.RunResult{}, nil)
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"an unknown flag":          {[]string{"--no-such-flag", "msg"}, "flag provided but not defined"},
		"push-only with a message": {[]string{"--push-only", "msg"}, "--push-only takes no commit message"},
		"no commit message":        {nil, "usage: evolve ship"},
		"two commit messages":      {[]string{"one", "two"}, "extra positional args"},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer

			rc := runShipCmd(tc.args, nil, io.Discard, &stderr)

			if rc != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("rc = %d, stderr = %q, want 1 and %q", rc, stderr.String(), tc.want)
			}
		})
	}
	if len(*calls) != 0 {
		t.Errorf("ship ran %d times for a refused command line", len(*calls))
	}
}

func TestRunShipCmd_ResolvesTheRootsFromTheFlagThenTheEnvironmentThenTheCwd(t *testing.T) {
	cwd := t.TempDir()
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return cwd, nil }
	calls := stubShipRun(t, ship.RunResult{}, nil)
	flagRoot, flagPlugin := filepath.Join(t.TempDir(), "flag-root"), filepath.Join(t.TempDir(), "flag-plugin")
	envRoot, envPlugin := filepath.Join(t.TempDir(), "env-root"), filepath.Join(t.TempDir(), "env-plugin")
	for name, tc := range map[string]struct {
		args                 []string
		envRoot, envPlugin   string
		wantRoot, wantPlugin string
	}{
		"the flags win":               {[]string{"--project-root", flagRoot, "--plugin-root", flagPlugin, "msg"}, envRoot, envPlugin, flagRoot, flagPlugin},
		"the environment is next":     {[]string{"msg"}, envRoot, envPlugin, envRoot, envPlugin},
		"the cwd is the last project": {[]string{"msg"}, "", "", cwd, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EVOLVE_PROJECT_ROOT", tc.envRoot)
			t.Setenv("EVOLVE_PLUGIN_ROOT", tc.envPlugin)
			before := len(*calls)

			rc := runShipCmd(tc.args, nil, io.Discard, io.Discard)

			if rc != 0 || len(*calls) != before+1 {
				t.Fatalf("rc = %d, ship calls = %d, want 0 and one more call", rc, len(*calls)-before)
			}
			if got := (*calls)[before]; got.ProjectRoot != tc.wantRoot || got.PluginRoot != tc.wantPlugin {
				t.Errorf("roots = %q and %q, want %q and %q", got.ProjectRoot, got.PluginRoot, tc.wantRoot, tc.wantPlugin)
			}
		})
	}
}

func TestRunShipCmd_AnUnreadableCwdIsRefused(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd refused") }
	calls := stubShipRun(t, ship.RunResult{}, nil)
	var stderr bytes.Buffer

	rc := runShipCmd([]string{"msg"}, nil, io.Discard, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "evolve ship: cwd: getwd refused") || len(*calls) != 0 {
		t.Errorf("rc = %d, stderr = %q, ship calls = %d, want 1, the cwd error and no ship run", rc, stderr.String(), len(*calls))
	}
}

func TestRunShipCmd_AnUnresolvableRootWarnsAndKeepsThePath(t *testing.T) {
	prev := shipAbsoluteRoot
	t.Cleanup(func() { shipAbsoluteRoot = prev })
	shipAbsoluteRoot = func(_, p string, warn func(string)) string {
		warn("could not resolve " + p)
		return p
	}
	calls := stubShipRun(t, ship.RunResult{}, nil)
	var stderr bytes.Buffer

	rc := runShipCmd([]string{"--project-root", "rel/root", "msg"}, nil, io.Discard, &stderr)

	if rc != 0 || !strings.HasPrefix(stderr.String(), "evolve ship: WARN: could not resolve rel/root\n") {
		t.Errorf("rc = %d, stderr = %q, want 0 and the resolve WARN first", rc, stderr.String())
	}
	if len(*calls) != 1 || (*calls)[0].ProjectRoot != "rel/root" {
		t.Errorf("ship calls = %+v, want one call that keeps the unresolved path", *calls)
	}
}

func TestRunShipCmd_PassesTheFlagsAndTheStreamsToShip(t *testing.T) {
	calls := stubShipRun(t, ship.RunResult{}, nil)
	root := t.TempDir()
	stdin, stdout, stderr := strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{}

	runShipCmd([]string{"--class", "manual", "--dry-run", "--bypass-commit-gate", "--bypass-prefix-gate", "--project-root", root, "feat: x"}, stdin, stdout, stderr)
	runShipCmd([]string{"--push-only", "--project-root", root}, nil, io.Discard, io.Discard)

	if len(*calls) != 2 {
		t.Fatalf("ship calls = %d, want 2", len(*calls))
	}
	manual, pushOnly := (*calls)[0], (*calls)[1]
	if manual.Class != ship.ClassManual || manual.CommitMessage != "feat: x" || !manual.DryRun || !manual.BypassCommitGate || !manual.BypassPrefixGate || manual.PushOnly {
		t.Errorf("manual options = %+v", manual)
	}
	if manual.Stdin != stdin || manual.Stdout != stdout || manual.Stderr != stderr || manual.Signals == nil {
		t.Errorf("the streams and the Center must reach ship: %+v", manual)
	}
	if !pushOnly.PushOnly || pushOnly.CommitMessage != "" || pushOnly.Class != ship.ClassCycle {
		t.Errorf("push-only options = %+v, want the push-only mode with no message and the default class", pushOnly)
	}
}

func TestRunShipCmd_MapsTheShipResultToTheExitCode(t *testing.T) {
	for name, tc := range map[string]struct {
		res     ship.RunResult
		err     error
		wantRC  int
		wantLog bool
	}{
		"a green ship":                       {ship.RunResult{ExitCode: ship.ExitOK, Logs: []string{"[ship] OK: pushed to origin/main", "[ship] OK: done"}}, nil, 0, true},
		"a failed ship keeps its own code":   {ship.RunResult{ExitCode: 3, Logs: []string{"[ship] FAIL: gate"}}, errors.New("gate refused"), 3, true},
		"a failed ship with no code":         {ship.RunResult{ExitCode: ship.ExitOK}, errors.New("no code"), int(ship.ExitFailure), false},
		"a clean result with a nonzero code": {ship.RunResult{ExitCode: 2}, nil, 2, false},
	} {
		t.Run(name, func(t *testing.T) {
			stubShipRun(t, tc.res, tc.err)
			var stderr bytes.Buffer

			rc := runShipCmd([]string{"--project-root", t.TempDir(), "msg"}, nil, io.Discard, &stderr)

			if rc != tc.wantRC {
				t.Errorf("rc = %d, want %d", rc, tc.wantRC)
			}
			printed := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if tc.wantLog && !slices.Equal(printed, tc.res.Logs) {
				t.Errorf("stderr lines = %q, want the ship logs %q in order", printed, tc.res.Logs)
			}
			if !tc.wantLog && strings.TrimSpace(stderr.String()) != "" {
				t.Errorf("stderr = %q, want nothing", stderr.String())
			}
		})
	}
}

func TestRunShipCmd_ClosesItsCenterBeforeItReturns(t *testing.T) {
	useRealRootEvents(t)
	root := t.TempDir()
	stubShipRunFunc(t, func(opts ship.Options) (ship.RunResult, error) {
		opts.Signals.Emit(rootEvent(signalcenter.ModuleShip, signalcenter.KindShipLanded, 7))
		return ship.RunResult{}, nil
	})

	rc := runShipCmd([]string{"--project-root", root, "msg"}, nil, io.Discard, io.Discard)

	evolveDir := filepath.Join(root, ".evolve")
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	for _, name := range []string{"ship", "cycle", "signals"} {
		if got := channelKinds(t, evolveDir, name); !slices.Equal(got, []string{"ship/ship.landed"}) {
			t.Errorf("channel %s holds %v, want the landing stamped with the role ship once runShipCmd returned", name, got)
		}
	}
}
