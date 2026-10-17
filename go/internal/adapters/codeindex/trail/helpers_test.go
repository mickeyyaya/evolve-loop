package trail_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/trail"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	testBin      = "/hub/dev/.tools/trail/node_modules/.bin/trail"
	testIndexDir = "/hub/dev/code-index"
)

type call struct {
	name string
	args []string
	env  []string
}

type callLog struct {
	mu    sync.Mutex
	calls []call
}

func (l *callLog) add(c call) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, c)
}

func (l *callLog) only(t *testing.T) call {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) != 1 {
		t.Fatalf("calls = %d, want exactly 1", len(l.calls))
	}
	return l.calls[0]
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type reply struct {
	stdout, stderr string
	exit           int
	err            error
}

func replying(t *testing.T, log *callLog, answer func(args []string) reply) sysexec.RunFunc {
	return func(_ context.Context, name, _ string, args, env []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		log.add(call{name: name, args: args, env: env})
		r := answer(args)
		_, _ = io.WriteString(stdout, r.stdout)
		_, _ = io.WriteString(stderr, r.stderr)
		return r.exit, r.err
	}
}

func positionals(args []string) []string {
	for i, a := range args {
		if a == "--" {
			return args[i+1:]
		}
	}
	return args[len(args)-1:]
}

func fixtureReplies(t *testing.T) func(args []string) reply {
	noSymbol := fixture(t, "err_no_symbol.txt")
	return func(args []string) reply {
		pos := positionals(args)
		switch {
		case args[0] == "callers" && pos[0] == "runStagedCatalogRefresh" && args[2] == "in":
			return reply{stdout: fixture(t, "callers_in.txt")}
		case args[0] == "callers" && pos[0] == "makeCatalogRefresher" && args[2] == "out":
			return reply{stdout: fixture(t, "callers_out.txt")}
		case args[0] == "callers":
			return reply{stderr: noSymbol, exit: 1}
		case args[0] == "skeleton" && pos[0] == "go/cmd/evolve/cmd_models_live.go":
			return reply{stdout: fixture(t, "skeleton.txt")}
		case args[0] == "skeleton":
			return reply{stdout: fixture(t, "skeleton_empty.txt")}
		case args[0] == "ask" && pos[0] == "catalog refresh":
			return reply{stdout: fixture(t, "ask.txt")}
		case args[0] == "ask":
			return reply{stdout: fixture(t, "ask_empty.txt")}
		default:
			return reply{stdout: fixture(t, "map.txt")}
		}
	}
}

func absentReplies(t *testing.T) func(args []string) reply {
	noGraph := fixture(t, "err_no_graph.txt")
	return func([]string) reply { return reply{stderr: noGraph, exit: 1} }
}

func newBackend(run sysexec.RunFunc) trail.Backend {
	return trail.New(trail.Config{Bin: testBin, IndexDir: testIndexDir, Env: []string{"PATH=/usr/bin"}, Run: run})
}

func hasEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func joined(args []string) string { return strings.Join(args, " ") }
