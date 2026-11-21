// Package lanebudget gives each go test that a fleet lane runs one share of the host CPUs.
// See docs/architecture/packages/internal-lanebudget.md.
package lanebudget

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const LiveWidthFile = "fleet-live-width.json"

var hostCPUs = runtime.NumCPU

var processAlive = runlease.PIDAlive

type liveWidth struct {
	PID   int `json:"pid"`
	Width int `json:"width"`
}

func WriteLiveWidth(path string, pid, width int) error {
	return atomicwrite.JSON(path, liveWidth{PID: pid, Width: max(1, width)})
}

func readLiveWidth() (int, bool) {
	raw, err := os.ReadFile(os.Getenv(ipcenv.FleetLiveWidthKey))
	if err != nil {
		return 0, false
	}
	var live liveWidth
	if json.Unmarshal(raw, &live) != nil || live.Width < 1 || !processAlive(live.PID) {
		return 0, false
	}
	return live.Width, true
}

func Procs(hostCPUs, width int) int {
	if width < 1 {
		return 0
	}
	return max(1, hostCPUs/width)
}

func laneWidth() int {
	spawn, err := strconv.Atoi(strings.TrimSpace(os.Getenv(ipcenv.FleetWidthKey)))
	if err != nil {
		return 0
	}
	if width, ok := readLiveWidth(); ok {
		return width
	}
	return spawn
}

func Current() int { return Procs(hostCPUs(), laneWidth()) }

func HolderProcs(hostCPUs, width int) int {
	share := Procs(hostCPUs, width)
	if share == 0 {
		return 0
	}
	return max(share, hostCPUs-(width-1)*share)
}

func Holder() int { return HolderProcs(hostCPUs(), laneWidth()) }

func ShareFor(held bool) int {
	if held {
		return Holder()
	}
	return Current()
}

func Args(args []string) []string { return ArgsFor(Current(), args) }

func ArgsFor(procs int, args []string) []string {
	if procs == 0 || !isGoTest(args) {
		return args
	}
	value := strconv.Itoa(procs)
	out := []string{args[0]}
	for _, flag := range []string{"-p", "-parallel"} {
		if !hasFlag(args[1:], flag) {
			out = append(out, flag, value)
		}
	}
	return append(out, args[1:]...)
}

func Env(env []string) []string { return EnvFor(Current(), env) }

func EnvFor(procs int, env []string) []string {
	if procs == 0 {
		return env
	}
	if env == nil {
		env = os.Environ()
	}
	return append(slices.Clip(env), "GOMAXPROCS="+strconv.Itoa(procs))
}

func Runner(run sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "go" && isGoTest(args) {
			args, env = Args(args), Env(env)
		}
		return run(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func isGoTest(args []string) bool {
	return len(args) > 0 && args[0] == "test"
}

func hasFlag(args []string, flag string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return arg == flag || strings.HasPrefix(arg, flag+"=")
	})
}
