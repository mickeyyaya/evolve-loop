package bridge

import (
	"context"
	"fmt"
	"io"
)

func ProbeBootAttempts(driverName string) int {
	m, err := loadManifestRaw(driverName)
	if err != nil {
		return 1
	}
	return 1 + m.ProbeBootRetries
}

type BootProbe struct {
	Driver string
	Log    io.Writer
}

func (p BootProbe) Retry(ctx context.Context, attempt func() (rc int, wall string)) int {
	stderr, driverName := p.Log, p.Driver
	if stderr == nil {
		stderr = io.Discard
	}
	attempts := ProbeBootAttempts(driverName)
	pfx := "[" + driverName + "]"
	for n := 1; ; n++ {
		rc, wall := attempt()
		coldStart := rc == ExitREPLBootTimeout && wall == ""
		switch {
		case !coldStart:
			if n > 1 {
				fmt.Fprintf(stderr, "%s cold start: boot attempt %d of %d got past the REPL boot (rc=%d)\n", pfx, n, attempts, rc)
			}
			return rc
		case n >= attempts:
			if attempts > 1 {
				fmt.Fprintf(stderr, "%s FAIL: the REPL never drew its prompt on any of %d boot attempts; this is no cold start\n", pfx, attempts)
			}
			return rc
		case ctx.Err() != nil:
			return rc
		}
		fmt.Fprintf(stderr, "%s cold start: boot attempt %d of %d timed out before the REPL drew its prompt; retrying (the %s manifest's probe_boot_retries=%d)\n", pfx, n, attempts, driverName, attempts-1)
	}
}
