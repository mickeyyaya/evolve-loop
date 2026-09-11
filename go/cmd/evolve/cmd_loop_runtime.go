package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// loopSignalContext is a package variable so tests can cancel a batch without
// delivering a real process signal.
var loopSignalContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

// loopBatchRuntime owns the process resources whose lifetime is exactly one
// batch. closeSocket runs before stopSignal to preserve the former defer order.
type loopBatchRuntime struct {
	ctx         context.Context
	stopSignal  context.CancelFunc
	closeSocket func()
}

func startLoopBatchRuntime() loopBatchRuntime {
	ctx, stop := loopSignalContext(context.Background())
	if os.Getenv(bridge.TmuxSocketEnv) == "" {
		_ = os.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(os.Getpid()))
	}
	return loopBatchRuntime{
		ctx:         ctx,
		stopSignal:  stop,
		closeSocket: runSocketTeardown(os.Getenv(bridge.TmuxSocketEnv), swarm.ExecKillServer),
	}
}

func (r loopBatchRuntime) close() {
	r.closeSocket()
	r.stopSignal()
}

// runSocketTeardownTimeout bounds exit-time cleanup when tmux is wedged.
const runSocketTeardownTimeout = 10 * time.Second

// runSocketTeardown destroys only the per-run socket derived by this process.
// An operator socket or a socket inherited from another process is preserved.
func runSocketTeardown(socket string, kill func(context.Context, string) error) func() {
	if socket == "" || socket != bridge.DeriveRunSocket(os.Getpid()) {
		return func() {}
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), runSocketTeardownTimeout)
		defer cancel()
		_ = kill(ctx, socket)
	}
}
