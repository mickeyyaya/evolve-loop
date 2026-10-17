package ciparitygate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type goTestCall struct {
	args string
	env  []string
}

func goTestRecorder(calls *[]goTestCall, list string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, name, _ string, args, env []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if args[0] == "list" {
			_, _ = io.WriteString(so, list)
		}
		if name == "go" && args[0] == "test" {
			*calls = append(*calls, goTestCall{args: strings.Join(args, " "), env: env})
		}
		return 0, nil
	}
}

func runEachGoTestGate(t *testing.T) []goTestCall {
	t.Helper()
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls []goTestCall
	g := New(goTestRecorder(&calls, filepath.Join(goDir, "internal", "p")+"\n"), fixedSet("./internal/p/..."))
	req := tierRequest(root, "")
	if _, err := g.IntegrationTier(req); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ACSDurable(req); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ApicoverEnforce(req); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("go test calls = %+v, want the tier, the acs-durable and the apicover runs", calls)
	}
	return calls
}

func TestGoTestGates_ALaneRunsEachGoTestOnTheLaneCPUBudget(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, strconv.Itoa(runtime.NumCPU()))

	calls := runEachGoTestGate(t)

	wantArgs := []string{
		"test -race -count=1 -p 1 -parallel 1 -tags integration ./internal/p/...",
		"test -p 1 -parallel 1 -count=1 -tags acs ./acs/regression/...",
		"test -p 1 -parallel 1 -tags integration acs -coverprofile=",
	}
	for i, c := range calls {
		if !strings.HasPrefix(c.args, wantArgs[i]) || c.env[len(c.env)-1] != "GOMAXPROCS=1" {
			t.Errorf("go test call %d = %q with env tail %q, want prefix %q and GOMAXPROCS=1", i, c.args, c.env[len(c.env)-1:], wantArgs[i])
		}
	}
}

func TestGoTestGates_AConsoleRunKeepsTheFixedTierParallelismAndNoGOMAXPROCS(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "")

	calls := runEachGoTestGate(t)

	if !strings.Contains(calls[0].args, "-p 4 -parallel 4") {
		t.Errorf("tier argv = %q, want the console parallelism of 4", calls[0].args)
	}
	for i, c := range calls {
		if strings.Contains(c.args, "-p 1 ") || slices.ContainsFunc(c.env, func(kv string) bool { return strings.HasPrefix(kv, "GOMAXPROCS=") }) {
			t.Errorf("go test call %d = %q with env %q, want no lane budget", i, c.args, c.env)
		}
	}
}
