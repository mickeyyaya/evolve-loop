package bridge

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const seatHolderMode = 0o700

var errSeatHome = errors.New("the home directory is empty")

func ensureSeatHolder(home string) ([]string, error) {
	dir := paths.SeatHolderDir(home)
	if dir == "" {
		return nil, errSeatHome
	}
	if err := os.MkdirAll(dir, seatHolderMode); err != nil {
		return nil, err
	}
	spellings := []string{dir}
	if real := canonicalOrSelf(dir); real != dir {
		spellings = append(spellings, real)
	}
	return spellings, nil
}

func seatHolderRefusal(err error) string {
	return fmt.Sprintf("[bridge] the seat token holder cannot be created: %v. The launch is refused, because the cell would run with no deny on the holder.\n", err)
}

func withSeatHolderDeny(cfg sandbox.Config, holders ...string) sandbox.Config {
	cfg.DenyReadPaths = append(slices.Clone(cfg.DenyReadPaths), holders...)
	return cfg
}
