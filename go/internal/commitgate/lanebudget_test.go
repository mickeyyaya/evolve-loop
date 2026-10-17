package commitgate

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
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func goLaneCalls(t *testing.T) (tests, vets []string, testEnv []string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier,go-reviewer"
	o.Env = os.Environ()
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff --git a/x.go b/x.go\n", exit: 1},
	}}
	inner := sr.run()
	o.Runner = func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "go" && args[0] == "test" {
			tests, testEnv = append(tests, strings.Join(args, " ")), env
		}
		if name == "go" && args[0] == "vet" {
			vets = append(vets, strings.Join(args, " "))
		}
		return inner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}

	if res := o.Run(context.Background()); res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	return tests, vets, testEnv
}

func TestRun_ALaneGoTestRunsOnTheLaneCPUBudget(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "4")
	procs := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))

	tests, vets, env := goLaneCalls(t)

	if want := []string{"test -p " + procs + " -parallel " + procs + " ./."}; !slices.Equal(tests, want) || env[len(env)-1] != "GOMAXPROCS="+procs {
		t.Errorf("go test = %q with env tail %q, want %q and GOMAXPROCS=%s", tests, env[len(env)-1:], want, procs)
	}
	if !slices.Equal(vets, []string{"vet ./."}) {
		t.Errorf("go vet = %q, want it unchanged", vets)
	}
}

func TestRun_AConsoleGoTestKeepsTheGoDefaults(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "")

	tests, _, env := goLaneCalls(t)

	if !slices.Equal(tests, []string{"test ./."}) || env != nil {
		t.Errorf("go test = %q with env %q, want it unchanged", tests, env)
	}
}
