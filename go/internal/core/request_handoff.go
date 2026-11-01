package core

import (
	"context"
	"fmt"
)

type RequestCell struct {
	ProjectRoot string
	Workspace   string
	Cycle       int
	Phase       string
}

type RequestHandoff func(ctx context.Context, cell RequestCell) error

func WithRequestHandoff(h RequestHandoff) Option {
	return func(o *Orchestrator) { o.requestHandoff = h }
}

func (o *Orchestrator) RequestHandoffWired() bool { return o.requestHandoff != nil }

func (cr *cycleRun) handoffRequests(phase Phase) {
	if cr.o.requestHandoff == nil {
		return
	}
	cell := RequestCell{ProjectRoot: cr.req.ProjectRoot, Workspace: cr.cs.WorkspacePath, Cycle: cr.cycle, Phase: string(phase)}
	if err := cr.o.requestHandoff(cr.ctx, cell); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN request channel of %s: %v\n", phase, err)
	}
}
