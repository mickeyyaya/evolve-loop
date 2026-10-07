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

func NewRunServePhase(install RouterInstaller) func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return phaseCommand{install: install}.serve
}

func (c phaseCommand) serve(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
		if err := c.installRouter(req.ProjectRoot); err != nil {
			return core.PhaseResponse{}, err
		}
		return factory(req).Run(ctx, req)
	}

	if err := phaseproto.ServeStdio(stdin, stdout, handler); err != nil {
		fmt.Fprintf(stderr, "evolve serve-phase: %s: %v\n", name, err)
		return 1
	}
	return 0
}
