package bridge

import (
	"errors"
	"syscall"
)

var errRealTmuxTestSendsNoSignal = errors.New("a real tmux bridge test sends no signal, because kill-session ends its pane")

func refuseTestSignal(int, syscall.Signal) error { return errRealTmuxTestSendsNoSignal }
