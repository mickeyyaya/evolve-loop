package audit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	generatorEchoMain = `package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	fmt.Println("echo", strings.Join(os.Args[1:], " "))
	fmt.Fprintln(os.Stderr, "a line on stderr")
	if len(os.Args) > 1 {
		if code, err := strconv.Atoi(os.Args[1]); err == nil {
			os.Exit(code)
		}
	}
}
`
	generatorOtherMain  = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"other\") }\n"
	generatorThirdMain  = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"third\") }\n"
	generatorBrokenMain = "package main\n\nfunc main() { undefinedCall() }\n"
)

type generatorCall struct {
	code   int
	stdout string
	stderr string
	err    error
}

func generatorModule(t *testing.T, main string) string {
	t.Helper()
	goDir := filepath.Join(t.TempDir(), "go")
	fixtures.MustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/generator\n\ngo 1.23\n")
	fixtures.MustWrite(t, filepath.Join(goDir, "cmd", "evolve", "main.go"), main)
	return goDir
}

func callGenerator(run sysexec.RunFunc, goDir string, args ...string) generatorCall {
	var stdout, stderr strings.Builder
	code, err := run(context.Background(), "go", goDir, append([]string{"run", "./cmd/evolve"}, args...), os.Environ(), nil, &stdout, &stderr)
	return generatorCall{code: code, stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func newGeneratorBuilds(t *testing.T) *generatorBuilds {
	t.Helper()
	g := &generatorBuilds{}
	t.Cleanup(func() {
		if err := g.remove(); err != nil {
			t.Errorf("remove generator builds: %v", err)
		}
	})
	return g
}

func TestGeneratorBuilds_OneBuildPerDistinctTree(t *testing.T) {
	t.Parallel()
	g := newGeneratorBuilds(t)
	run := g.runner(sysexec.DefaultRunner)
	first, copyOfFirst, other := generatorModule(t, generatorEchoMain), generatorModule(t, generatorEchoMain), generatorModule(t, generatorOtherMain)

	calls := []generatorCall{
		callGenerator(run, first, "a"),
		callGenerator(run, copyOfFirst, "b"),
		callGenerator(run, first, "c"),
		callGenerator(run, other),
	}

	if got := g.builds.Load(); got != 2 {
		t.Fatalf("four runs over two distinct trees made %d build(s), want 2: the key must be the tree content, not its path", got)
	}
	for i, want := range []string{"echo a\n", "echo b\n", "echo c\n", "other\n"} {
		if calls[i].err != nil || calls[i].code != 0 || calls[i].stdout != want {
			t.Errorf("run %d = %+v, want exit 0 with stdout %q", i, calls[i], want)
		}
	}
	fixtures.MustWrite(t, filepath.Join(copyOfFirst, "cmd", "evolve", "main.go"), generatorOtherMain)
	if got := callGenerator(run, copyOfFirst); got.stdout != "other\n" || g.builds.Load() != 2 {
		t.Errorf("a tree edited to equal a built tree = %+v after %d build(s), want the existing build's output and no new build", got, g.builds.Load())
	}
	fixtures.MustWrite(t, filepath.Join(copyOfFirst, "cmd", "evolve", "main.go"), generatorThirdMain)
	if got := callGenerator(run, copyOfFirst); got.stdout != "third\n" || g.builds.Load() != 3 {
		t.Errorf("an edited generator source = %+v after %d build(s), want its own build and its own output", got, g.builds.Load())
	}
}

func TestGeneratorBuilds_ARunMatchesGoRunByteForByte(t *testing.T) {
	t.Parallel()
	run := newGeneratorBuilds(t).runner(sysexec.DefaultRunner)
	goDir := generatorModule(t, generatorEchoMain)
	for _, args := range [][]string{{"ok"}, {"3"}} {
		shared, real := callGenerator(run, goDir, args...), callGenerator(sysexec.DefaultRunner, goDir, args...)

		if shared != real {
			t.Errorf("args %v: the shared build gives %+v, `go run` gives %+v", args, shared, real)
		}
	}
}

func TestGeneratorBuilds_ABrokenTreeFailsLikeGoRunAndBuildsOnce(t *testing.T) {
	t.Parallel()
	g := newGeneratorBuilds(t)
	run := g.runner(sysexec.DefaultRunner)
	goDir := generatorModule(t, generatorBrokenMain)

	first, second := callGenerator(run, goDir), callGenerator(run, goDir)

	for _, got := range []generatorCall{first, second} {
		if got.err != nil || got.code != 1 || !strings.Contains(got.stderr, "undefinedCall") {
			t.Errorf("a tree that does not compile = %+v, want exit 1 with the compile error on stderr, as `go run` gives", got)
		}
	}
	if n := g.builds.Load(); n != 1 {
		t.Errorf("two runs of one broken tree made %d build(s), want 1", n)
	}
}

func TestGeneratorBuilds_OtherCommandsPassThrough(t *testing.T) {
	t.Parallel()
	g := newGeneratorBuilds(t)
	var seen []string
	run := g.runner(func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		seen = append(seen, name+" "+strings.Join(args, " "))
		return 7, nil
	})
	for _, call := range [][]string{{"go", "vet", "./..."}, {"go", "run", "./cmd/other"}, {"git", "run", "./cmd/evolve"}, {"go", "run"}} {
		if code, err := run(context.Background(), call[0], t.TempDir(), call[1:], nil, nil, nil, nil); code != 7 || err != nil {
			t.Errorf("%v = (%d, %v), want the wrapped runner's (7, nil)", call, code, err)
		}
	}
	if len(seen) != 4 || g.builds.Load() != 0 {
		t.Errorf("pass-through calls reached the runner %d time(s) with %d build(s), want 4 and 0: %v", len(seen), g.builds.Load(), seen)
	}
}

func TestGeneratorBuilds_AnUnreadableTreeIsAnError(t *testing.T) {
	t.Parallel()
	g := newGeneratorBuilds(t)

	got := callGenerator(g.runner(sysexec.DefaultRunner), filepath.Join(t.TempDir(), "missing"))

	if got.err == nil || got.code != -1 || !strings.Contains(got.err.Error(), "hash the generator tree") || g.builds.Load() != 0 {
		t.Errorf("a missing generator tree = %+v, want exit -1 with a hash error and no build", got)
	}
}

func TestGeneratorBuilds_BuildOneAtATimeSoEachBuildReusesTheLastOnesCache(t *testing.T) {
	t.Parallel()
	g := newGeneratorBuilds(t)
	var mu sync.Mutex
	active, peak := 0, 0
	run := g.runner(func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "build" {
			mu.Lock()
			active++
			peak = max(peak, active)
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
		}
		return 0, nil
	})
	var wg sync.WaitGroup
	for i := range 4 {
		goDir := generatorModule(t, generatorOtherMain+strings.Repeat("\n", i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			callGenerator(run, goDir)
		}()
	}
	wg.Wait()

	if g.builds.Load() != 4 || peak != 1 {
		t.Errorf("four distinct trees made %d build(s) with %d at once, want 4 builds, one at a time", g.builds.Load(), peak)
	}
}
