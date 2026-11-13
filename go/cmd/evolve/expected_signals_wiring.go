package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type expectedWiring struct {
	evolveDir string
	console   io.Writer
	now       func() time.Time
}

func wireExpectedSignals(signals *signalcenter.Center, evolveDir string, console io.Writer) {
	wireExpectedSignalsWith(signals, expectedWiring{evolveDir: evolveDir, console: console, now: time.Now})
}

func wireExpectedSignalsWith(signals *signalcenter.Center, w expectedWiring) {
	cfg, err := policy.Load(paths.PolicyPath(w.evolveDir))
	if err != nil {
		fmt.Fprintf(w.console, "[expected-signal] WARN: policy unreadable (%v); the default deadlines apply\n", err)
	}
	stream := filepath.Join(w.evolveDir, signalcenter.StreamFileName)
	x := signalcenter.NewExpectedSignals(signals, signalcenter.DefaultExpectedPairs(), cfg.ExpectedSignal.Deadline, w.now)
	poll := func() {
		if err := x.Poll(stream); err != nil {
			fmt.Fprintf(w.console, "[expected-signal] WARN: the stream was not read (%v)\n", err)
		}
	}
	poll()
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindLoopTick {
			poll()
		}
	})
}
