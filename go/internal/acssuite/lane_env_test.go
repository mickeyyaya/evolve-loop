package acssuite

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func effectiveEnv(env []string) map[string]string {
	out := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[k] = v
		}
	}
	return out
}

func evolveKeys(env map[string]string) []string {
	var keys []string
	for k := range env {
		if strings.HasPrefix(k, "EVOLVE_") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func setLaneIPCEnv(t *testing.T) {
	t.Helper()
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.FleetScopeKey, "lane-scope")
	t.Setenv(ipcenv.FleetWidthKey, "3")
	t.Setenv(ipcenv.CycleStateFileKey, "/plane/.evolve/runs/cycle-1700/cycle-state.json")
	t.Setenv(ipcenv.TmuxSocketKey, "/tmp/evolve-lane.sock")
}

func captureScopeEnvs(envs *[][]string) func(context.Context, string, string, []string) (string, error) {
	return func(_ context.Context, _, _ string, env []string) (string, error) {
		*envs = append(*envs, env)
		return "", nil
	}
}

func TestRun_PredicatesSeeNoLaneIPCEnvironment(t *testing.T) {
	setLaneIPCEnv(t)
	root, plane := t.TempDir(), t.TempDir()
	var envs [][]string
	if _, err := Run(Options{Root: root, ProjectRoot: plane, Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	if len(envs) == 0 {
		t.Fatal("no scope ran")
	}
	for _, raw := range envs {
		env := effectiveEnv(raw)
		if got, want := evolveKeys(env), []string{"EVOLVE_PROJECT_ROOT", ipcenv.WorktreeRootKey}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("predicate EVOLVE_ keys = %v, want exactly the suite's own %v (the lane's fleet, scope, width, cycle-state and tmux keys must not reach a predicate)", got, want)
		}
		if env["EVOLVE_PROJECT_ROOT"] != plane || env[ipcenv.WorktreeRootKey] != root {
			t.Errorf("state root=%q source root=%q, want %q and %q", env["EVOLVE_PROJECT_ROOT"], env[ipcenv.WorktreeRootKey], plane, root)
		}
		if env["PATH"] == "" {
			t.Error("the toolchain environment (PATH) must survive the scrub")
		}
	}
}

func TestRun_WithoutAProjectRootPredicatesGetTheSuiteRootAsTheStateRoot(t *testing.T) {
	setLaneIPCEnv(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", "/caller/plane")
	root := t.TempDir()
	var envs [][]string
	if _, err := Run(Options{Root: root, Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range envs {
		env := effectiveEnv(raw)
		if env["EVOLVE_PROJECT_ROOT"] != root {
			t.Errorf("EVOLVE_PROJECT_ROOT = %q, want the suite root %q: a predicate's state root is the suite's own export, never whatever the caller's environment held", env["EVOLVE_PROJECT_ROOT"], root)
		}
		if _, leaked := env[ipcenv.FleetKey]; leaked {
			t.Errorf("%s leaked into the predicate", ipcenv.FleetKey)
		}
	}
}

func TestRun_ARealPredicateInAFleetLaneSeesNoLaneKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a real go test subprocess")
	}
	setLaneIPCEnv(t)
	root, plane := t.TempDir(), t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod": "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go": "//go:build acs\n\npackage cycle7\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
			"func TestC7_001_NoLaneKeys(t *testing.T) {\n" +
			"\tfor _, k := range []string{\"EVOLVE_FLEET\", \"EVOLVE_FLEET_SCOPE\", \"EVOLVE_FLEET_WIDTH\", \"EVOLVE_CYCLE_STATE_FILE\", \"EVOLVE_TMUX_SOCKET\"} {\n" +
			"\t\tif v, ok := os.LookupEnv(k); ok {\n\t\t\tt.Errorf(\"%s=%q reached the predicate\", k, v)\n\t\t}\n\t}\n}\n\n" +
			"func TestC7_002_StateRoot(t *testing.T) {\n" +
			"\tif got := os.Getenv(\"EVOLVE_PROJECT_ROOT\"); got != " + strconv.Quote(plane) + " {\n\t\tt.Errorf(\"EVOLVE_PROJECT_ROOT=%q\", got)\n\t}\n}\n",
	})
	v, err := Run(Options{Root: root, ProjectRoot: plane, Cycle: 7})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 0 || v.GreenCount != 2 {
		t.Fatalf("green=%d red=%d reds=%+v, want both predicates green under a fleet lane's environment", v.GreenCount, v.RedCount, redsOf(v))
	}
}

func redsOf(v Verdict) []Result {
	var out []Result
	for _, r := range v.Results {
		if r.ResultStr == "red" {
			out = append(out, r)
		}
	}
	return out
}
