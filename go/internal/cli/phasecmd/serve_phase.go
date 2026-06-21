// `evolve serve-phase <name>` is the envelope-framed subprocess entry
// that pairs with phaseproto.SubprocessRunner. Where `evolve phase`
// speaks raw PhaseRequest/PhaseResponse JSON for direct human / script
// use, `serve-phase` wraps the same handler in phaseproto.ServeStdio
// so the orchestrator can drive cross-CLI agents (Go, Node, Python)
// through one stable Envelope wire.
//
// Protocol (one process per invocation):
//
//	stdin:  one phaseproto.Envelope{Kind:"request"} line
//	stdout: one phaseproto.Envelope{Kind:"response"|"error"} line
//	stderr: human-readable diagnostics
//	exit:   0 normally (handler errors are wire-level via error envelope);
//	        1 on framing / I/O failure; 10 on bad CLI args.
package phasecmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/phaseproto"
)

func RunServePhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "evolve serve-phase: missing phase name (%s)\n", strings.Join(registry.Names(), "|"))
		return 10
	}
	name := strings.ToLower(args[0])
	factory, ok := registry.For(name)
	if !ok {
		fmt.Fprintf(stderr, "evolve serve-phase: unknown phase %q (known: %s)\n", name, strings.Join(registry.Names(), ", "))
		return 10
	}

	handler := func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
		return factory(req).Run(ctx, req)
	}

	if err := phaseproto.ServeStdio(stdin, stdout, handler); err != nil {
		fmt.Fprintf(stderr, "evolve serve-phase: %s: %v\n", name, err)
		return 1
	}
	return 0
}
