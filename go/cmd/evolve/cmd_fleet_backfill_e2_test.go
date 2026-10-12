package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type e2RecordedFleetLaunch struct {
	mu        sync.Mutex
	specs     []fleet.CycleSpec
	simulate  bool
	goalHash  string
	factories int
}

func e2StubFleetLaunch(t *testing.T, exit int, nilLaunch bool) *e2RecordedFleetLaunch {
	t.Helper()
	rec := &e2RecordedFleetLaunch{}
	previous := fleetLaunchFactory
	fleetLaunchFactory = func(_ string, simulate bool, _, goalHash, _ string, _, _ io.Writer) fleet.LaunchFn {
		rec.simulate, rec.goalHash = simulate, goalHash
		rec.factories++
		if nilLaunch {
			return nil
		}
		return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
			rec.mu.Lock()
			defer rec.mu.Unlock()
			rec.specs = append(rec.specs, spec)
			return exit, nil
		}
	}
	t.Cleanup(func() { fleetLaunchFactory = previous })
	return rec
}

func e2StubFleetExecutable(t *testing.T, path string, err error) {
	t.Helper()
	previous := fleetExecutable
	fleetExecutable = func() (string, error) { return path, err }
	t.Cleanup(func() { fleetExecutable = previous })
}

func TestRunFleet_RefusesBadInvocationsBeforeLaunching(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"an unknown flag", []string{"--nope"}, "flag provided but not defined: -nope"},
		{"a non-numeric count", []string{"--count", "two"}, `invalid value "two" for flag -count`},
		{"a zero count", []string{"--count", "0", "--goal-hash", "g"}, "--count must be > 0"},
		{"a negative count", []string{"--count", "-1", "--goal-hash", "g"}, "--count must be > 0"},
		{"no goal hash", []string{"--count", "1"}, "--goal-hash is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := e2StubFleetLaunch(t, 0, false)
			var stdout, stderr bytes.Buffer

			code := runFleet(tc.args, nil, &stdout, &stderr)

			if code != 1 || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("code=%d stderr=%q, want 1 containing %q", code, stderr.String(), tc.wantStderr)
			}
			if rec.factories != 0 {
				t.Fatal("a refused invocation built a launcher")
			}
		})
	}
}

func TestRunFleet_AnUnresolvableBinaryExits1(t *testing.T) {
	rec := e2StubFleetLaunch(t, 0, false)
	e2StubFleetExecutable(t, "", errors.New("no /proc"))
	var stdout, stderr bytes.Buffer

	code := runFleet([]string{"--count", "1", "--goal-hash", "g"}, nil, &stdout, &stderr)

	if code != 1 || stderr.String() != "evolve fleet: cannot resolve binary: no /proc\n" || rec.factories != 0 {
		t.Fatalf("code=%d stderr=%q factories=%d", code, stderr.String(), rec.factories)
	}
}

func TestRunFleet_ANilLauncherFailsValidation(t *testing.T) {
	e2StubFleetLaunch(t, 0, true)
	var stdout, stderr bytes.Buffer

	code := runFleet([]string{"--count", "2", "--goal-hash", "g"}, nil, &stdout, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "evolve fleet: ") || strings.Contains(stderr.String(), "[fleet] cycle") {
		t.Fatalf("code=%d stderr=%q, want a validation refusal and no cycles", code, stderr.String())
	}
}

func TestRunFleet_PlanErrorsExit1WithoutLaunching(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	writeE2File(t, corrupt, "{not an array")
	empty := filepath.Join(dir, "empty.json")
	writeE2File(t, empty, "[]")
	null := filepath.Join(dir, "null.json")
	writeE2File(t, null, "null")
	cases := []struct {
		name       string
		plan       string
		wantStderr string
	}{
		{"a missing plan file", filepath.Join(dir, "absent.json"), "evolve fleet: read --plan " + filepath.Join(dir, "absent.json") + ":"},
		{"a corrupt plan", corrupt, "evolve fleet: parse --plan backlog:"},
		{"an empty backlog", empty, "--plan yielded no schedulable cycles (empty backlog?)"},
		{"a null backlog", null, "--plan yielded no schedulable cycles"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := e2StubFleetLaunch(t, 0, false)
			var stdout, stderr bytes.Buffer

			code := runFleet([]string{"--count", "2", "--goal-hash", "g", "--plan", tc.plan}, nil, &stdout, &stderr)

			if code != 1 || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("code=%d stderr=%q, want 1 containing %q", code, stderr.String(), tc.wantStderr)
			}
			if len(rec.specs) != 0 {
				t.Fatalf("a plan error launched %d cycles", len(rec.specs))
			}
		})
	}
}

func TestRunFleet_APlanLaunchesDisjointCyclesAndDefersTheBridge(t *testing.T) {
	plan := filepath.Join(t.TempDir(), "plan.json")
	writeE2File(t, plan, `[{"id":"A","files":["a.go"]},{"id":"B","files":["b.go"]},{"id":"C","files":["a.go","b.go"]}]`)
	rec := e2StubFleetLaunch(t, 0, false)
	e2StubFleetExecutable(t, "/bin/evolve-fake", nil)
	chdirForTest(t, t.TempDir())
	var stdout, stderr bytes.Buffer

	code := runFleet([]string{"--count", "2", "--goal-hash", "hash-1", "--simulate", "--plan", plan}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	errText := stderr.String()
	for _, want := range []string{
		"[fleet] deferred to a later wave (bridges concurrent cycles): C\n",
		"[fleet] advisor plan: 2 disjoint cycles, 1 deferred\n",
		"[fleet] 2/2 cycles ok",
	} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, errText)
		}
	}
	scopes := map[string]string{}
	for _, s := range rec.specs {
		if s.GoalHash != "hash-1" {
			t.Fatalf("spec %v lost the goal hash", s.Scope)
		}
		scopes[strings.Join(s.Scope, ",")] = s.GoalHash
	}
	if !reflect.DeepEqual(scopes, map[string]string{"A": "hash-1", "B": "hash-1"}) {
		t.Fatalf("launched scopes = %v, want A and B only", scopes)
	}
	if !rec.simulate || rec.goalHash != "hash-1" {
		t.Fatalf("factory got simulate=%v goalHash=%q", rec.simulate, rec.goalHash)
	}
}

func TestRunFleet_WithoutAPlanLaunchesCountIdenticalCyclesAndFailsOnAFailedLane(t *testing.T) {
	rec := e2StubFleetLaunch(t, 4, false)
	chdirForTest(t, t.TempDir())
	var stdout, stderr bytes.Buffer

	code := runFleet([]string{"--count", "3", "--goal-hash", "same"}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code=%d, want 1 when every lane exits 4; stderr=%q", code, stderr.String())
	}
	if len(rec.specs) != 3 {
		t.Fatalf("launched %d cycles, want 3", len(rec.specs))
	}
	for _, s := range rec.specs {
		if s.GoalHash != "same" || len(s.Scope) != 0 {
			t.Fatalf("spec = %+v, want an unscoped spec carrying the goal hash", s)
		}
	}
	if strings.Contains(stderr.String(), "advisor plan") {
		t.Fatal("an unplanned fleet reported an advisor plan")
	}
}

func e2StubSoak(t *testing.T, launch fleet.LaunchFn) {
	t.Helper()
	previousLaunch, previousKiller := soakLaunchFn, soakKiller
	soakLaunchFn = launch
	soakKiller = swarm.TmuxKiller(func(context.Context, string) error { return nil })
	t.Cleanup(func() { soakLaunchFn, soakKiller = previousLaunch, previousKiller })
}

func TestRunFleetSoak_AnUnknownFlagExits1(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runFleet([]string{"soak", "--bogus"}, nil, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "flag provided but not defined: -bogus") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunFleetSoak_AnUnresolvableBinaryExits1WithoutAnInjectedLauncher(t *testing.T) {
	e2StubSoak(t, nil)
	e2StubFleetExecutable(t, "", errors.New("no exe"))
	var stdout, stderr bytes.Buffer

	code := runFleetSoak([]string{"--count", "1", "--evolve-dir", t.TempDir()}, nil, &stdout, &stderr)

	if code != 1 || stderr.String() != "evolve fleet soak: cannot resolve binary: no exe\n" || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunFleetSoak_AFailedInvariantFailsTheSoakEvenWhenEveryLaunchPasses(t *testing.T) {
	e2StubSoak(t, func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil })
	var stdout, stderr bytes.Buffer

	code := runFleetSoak([]string{"--count", "2", "--evolve-dir", t.TempDir(), "--toml-path", filepath.Join(t.TempDir(), "absent.toml")}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code=%d, want 1 when invariant 4 fails; stdout=%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "| 4 — no torn config     | read error:") || !strings.Contains(stdout.String(), "| FAIL |") {
		t.Fatalf("the report must show invariant 4 failing:\n%s", stdout.String())
	}
}

func TestRunFleetSoak_AFailedLaunchFailsTheSoakWithPassingInvariants(t *testing.T) {
	e2StubSoak(t, func(context.Context, fleet.CycleSpec) (int, error) { return 2, nil })
	var stdout, stderr bytes.Buffer

	code := runFleetSoak([]string{"--count", "1", "--evolve-dir", t.TempDir()}, nil, &stdout, &stderr)

	if code != 1 || strings.Contains(stdout.String(), "FAIL") {
		t.Fatalf("code=%d stdout=%s, want 1 with every invariant PASS", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "evolve fleet soak: 1/1 launches failed") {
		t.Fatalf("stderr=%q, want the failed launch counted", stderr.String())
	}
}
