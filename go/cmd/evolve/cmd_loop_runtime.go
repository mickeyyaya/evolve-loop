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

var loopSignalContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

type loopBatchRuntime struct {
	ctx         context.Context
	stopSignal  context.CancelFunc
	closeSocket func()
	ownsSocket  bool
}

func startLoopBatchRuntime() loopBatchRuntime {
	ctx, stop := loopSignalContext(context.Background())
	owns := os.Getenv(bridge.TmuxSocketEnv) == ""
	if owns {
		_ = os.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(os.Getpid()))
	}
	return loopBatchRuntime{
		ownsSocket:  owns,
		ctx:         ctx,
		stopSignal:  stop,
		closeSocket: runSocketTeardown(os.Getenv(bridge.TmuxSocketEnv), swarm.ExecKillServer),
	}
}

func (r loopBatchRuntime) close() {
	r.closeSocket()
	r.stopSignal()
	if r.ownsSocket {
		_ = os.Unsetenv(bridge.TmuxSocketEnv)
	}
}

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
