package bridge

import (
	"context"
	"fmt"
	"io"
)

// LaunchArgs is the argv-faithful launch entry point: it parses the same
// flag surface as `tools/agent-bridge/bin/bridge launch` (with BRIDGE_*
// env fallbacks; flags win), runs the launch pipeline, and returns a
// bridge exit code (one of the Exit* constants). It is what the
// `evolve bridge launch` CLI shim calls, and the surface the BATS parity
// tests target.
//
// The pipeline it will implement (M2 engine + M3–M5 drivers):
//
//	parse args/env → load profile → validate required → resolve effective
//	config (model/permission-mode/stream-output/session-name precedence) →
//	preflight (require-full, empty-prompt, stale-workspace, orphan sweep) →
//	dispatch to the registered Driver → optional --json report.
//
// Launch (the core.Bridge entry) will share this pipeline by mapping a
// core.BridgeRequest onto the same resolved Config.
//
// M1 STATUS: stub. Returns a sentinel (-1) and writes a not-implemented
// note to stderr so the parity tests compile and fail RED. M2 replaces
// this body with the real flow.
func (e *Engine) LaunchArgs(ctx context.Context, args []string, env map[string]string, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = env
	_ = stdout
	fmt.Fprintln(stderr, "bridge: LaunchArgs not implemented (M1 scaffold)")
	return launchNotImplemented
}

// launchNotImplemented is the M1 stub sentinel. It is deliberately not a
// real Exit* value so no parity test can pass against the stub by
// accident (every test asserts a concrete Exit* code and/or output).
const launchNotImplemented = -1
