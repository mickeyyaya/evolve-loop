package bridge

import (
	"errors"
	"syscall"
	"time"
)

var errRealTmuxTestSendsNoSignal = errors.New("a real tmux bridge test sends no signal, because kill-session ends its pane")

func refuseTestSignal(int, syscall.Signal) error { return errRealTmuxTestSendsNoSignal }

func itDeps(perTick time.Duration) Deps {
	d := Deps{
		Tmux:          execTmux{},
		Sleep:         func(time.Duration) { time.Sleep(perTick) },
		Now:           time.Now,
		SignalProcess: refuseTestSignal,
	}
	return d.withDefaults()
}
