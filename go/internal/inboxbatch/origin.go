package inboxbatch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	OriginOperator = "operator"
	OriginConsole  = "console"
	OriginCycle    = "cycle"
	OriginRetro    = "retro"
)

var ErrUnknownOrigin = errors.New("unknown origin")

func Origins() []string { return []string{OriginOperator, OriginConsole, OriginCycle, OriginRetro} }

func CheckOrigin(origin string) error {
	if slices.Contains(Origins(), origin) {
		return nil
	}
	return fmt.Errorf("%w %q: want one of %s", ErrUnknownOrigin, origin, strings.Join(Origins(), ", "))
}
