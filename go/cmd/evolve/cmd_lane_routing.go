package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func laneForbidden(projectRoot string, warn io.Writer) func(string) bool {
	return laneForbiddenTo(projectRoot, signalcenter.ConsoleSink(warn))
}

func laneForbiddenTo(projectRoot string, emit signalcenter.Listener) func(string) bool {
	return lanerouting.LazyForbidden(projectRoot, build.ProfileName, emit)
}
