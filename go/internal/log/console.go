package log

import (
	"fmt"
	"io"
	"os"
)

// Console is the unified human-facing logger. It concentrates the codebase's
// scattered fmt.Fprintf(os.Stderr/os.Stdout, ...) printers behind one injectable
// seam: informational output goes to Out, warnings and errors go to Err. This is
// additive — the structured emitters (SidecarWriter, NewJSONLogger) are
// unchanged; Console is for the operator-facing prose those printers emit.
//
// A zero-value Console is safe (nil sinks are no-ops); production code uses
// Default(); tests inject *strings.Builder sinks and assert routing.
type Console struct {
	Out   io.Writer // informational sink (Default: os.Stdout)
	Err   io.Writer // warning/error sink (Default: os.Stderr)
	Quiet bool      // suppress Infof; Warnf/Errorf always emit
}

// Default returns a Console wired to the process streams (Out=stdout, Err=stderr) —
// for CLI user-facing output where Info belongs on stdout.
func Default() Console {
	return Console{Out: os.Stdout, Err: os.Stderr}
}

// Diag returns a stderr-only diagnostics Console (Out and Err both os.Stderr).
// Use it for component/operational logging where ALL levels — including Info —
// belong on stderr (the Unix convention: stdout is reserved for program data).
// It is the behavior-preserving target for the codebase's many
// fmt.Fprintf(os.Stderr, ...) diagnostic printers.
func Diag() Console {
	return Console{Out: os.Stderr, Err: os.Stderr}
}

// Infof writes an informational line to Out. Suppressed when Quiet or Out is nil.
func (c Console) Infof(format string, args ...any) {
	if c.Quiet || c.Out == nil {
		return
	}
	fmt.Fprintf(c.Out, format, args...)
}

// Warnf writes a warning to Err (never Out). Always emitted, even when Quiet —
// a suppressed warning is worse than a noisy one.
func (c Console) Warnf(format string, args ...any) {
	if c.Err == nil {
		return
	}
	fmt.Fprintf(c.Err, format, args...)
}

// Errorf writes an error to Err. Always emitted.
func (c Console) Errorf(format string, args ...any) {
	if c.Err == nil {
		return
	}
	fmt.Fprintf(c.Err, format, args...)
}
