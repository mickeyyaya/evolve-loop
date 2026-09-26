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

// loopSignalContext is a package var so tests can cancel a batch without a
// real process signal.
var loopSignalContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

// loopBatchRuntime.close runs closeSocket before stopSignal, preserving the
// previous defer order.
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
