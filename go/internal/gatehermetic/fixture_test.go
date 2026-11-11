package gatehermetic

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/gomodtest"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var writeModule = gomodtest.WriteModule

var realLoader = goload.Loader{Run: sysexec.DefaultRunner}

var stubFiles = map[string]string{
	"go.mod": "module " + modulePath + "\n\ngo 1.23\n",
	"internal/sysexec/sysexec.go": `package sysexec

import (
	"context"
	"io"
	"os/exec"
)

type RunFunc func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

func DefaultRunner(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return 0, nil
}

func WithEnv(run RunFunc, env []string) RunFunc { return run }

func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func Capture(ctx context.Context, run RunFunc, dir, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	return
}

func Output(ctx context.Context, run RunFunc, dir, name string, args ...string) (string, error) {
	return "", nil
}

func CombinedOutput(ctx context.Context, run RunFunc, dir, name, stdin string, args ...string) (string, error) {
	return "", nil
}

func CancelKillsTheProcessGroup(cmd *exec.Cmd) {}

func Unlisted() {}
`,
	"internal/repostate/repostate.go": `package repostate

import (
	"context"

	"` + sysexecPath + `"
)

func Log(ctx context.Context) {
	_ = sysexec.Command(ctx, "git", "log")
}
`,
	"internal/gotool/gotool.go": `package gotool

import (
	"context"
	"os/exec"

	"` + sysexecPath + `"
)

type Call struct {
	Dir  string
	Env  []string
	Args []string
}

func List(ctx context.Context, call Call) *exec.Cmd { return command(ctx, call, "list") }

func Test(ctx context.Context, call Call) *exec.Cmd { return command(ctx, call, "test") }

func command(ctx context.Context, call Call, verb string) *exec.Cmd {
	return sysexec.Command(ctx, "go", append([]string{verb}, call.Args...)...)
}
`,
}

func fixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string]string{}
	for k, v := range stubFiles {
		all[k] = v
	}
	for k, v := range files {
		all[k] = v
	}
	return writeModule(t, all)
}

func gateRoot(pkg, fn string) Root {
	return Root{Package: modulePath + "/internal/" + pkg, Func: fn}
}

func loadFixture(t *testing.T, files map[string]string, roots ...Root) Graph {
	t.Helper()
	g, err := Load(context.Background(), realLoader, Source{ModuleDir: fixtureDir(t, files), Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func refuseFixture(t *testing.T, files map[string]string, roots ...Root) []Refusal {
	t.Helper()
	got, err := Refusals(loadFixture(t, files, roots...), Contract())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func classes(rs []Refusal) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Class)
	}
	slices.Sort(out)
	return out
}

func edges(r Refusal) []string {
	var out []string
	for _, s := range r.Chain {
		out = append(out, s.Edge)
	}
	return out
}

func lastFuncs(r Refusal) string {
	var out []string
	for _, s := range r.Chain {
		out = append(out, s.Func[strings.LastIndex(s.Func, "/")+1:])
	}
	return strings.Join(out, " > ")
}

func replying(stdout string, code int, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, _ = io.WriteString(out, stdout)
		return code, err
	}
}

func wantClasses(t *testing.T, got []Refusal, want ...string) {
	t.Helper()
	slices.Sort(want)
	if !slices.Equal(classes(got), want) {
		t.Errorf("classes = %v, want %v; refusals = %+v", classes(got), want, got)
	}
}
