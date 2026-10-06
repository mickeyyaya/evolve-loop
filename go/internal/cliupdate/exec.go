package cliupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const updaterTimeout = 10 * time.Minute

const outputTailBytes = 400

func Exec(run sysexec.RunFunc) func(ctx context.Context, f Family) error {
	return func(ctx context.Context, f Family) error {
		argv := f.UpdateArgv
		if len(argv) == 0 {
			return errors.New("empty updater argv")
		}
		uctx, cancel := context.WithTimeout(ctx, updaterTimeout)
		defer cancel()
		var stdout, stderr strings.Builder
		code, err := run(uctx, argv[0], "", argv[1:], updaterEnv(f.AutoUpdateOffEnv), nil, &stdout, &stderr)
		if err != nil {
			return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
		}
		if code != 0 {
			return fmt.Errorf("%s: exit %d: %s", strings.Join(argv, " "), code, tail(stdout.String()+"\n"+stderr.String()))
		}
		return nil
	}
}

func updaterEnv(offSwitch string) []string {
	if offSwitch == "" {
		return nil
	}
	return slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, offSwitch+"=") })
}

func tail(output string) string {
	output = strings.TrimSpace(output)
	if len(output) > outputTailBytes {
		output = "…" + output[len(output)-outputTailBytes:]
	}
	return strings.Join(strings.Fields(output), " ")
}
