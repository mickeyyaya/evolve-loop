//go:build !notmux

package bridge

import "time"

func itDeps(perTick time.Duration) Deps {
	d := Deps{
		Tmux:          execTmux{},
		Sleep:         func(time.Duration) { time.Sleep(perTick) },
		Now:           time.Now,
		SignalProcess: refuseTestSignal,
	}
	return d.withDefaults()
}
