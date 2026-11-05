package cycleoutcome

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func buildStage(workspace string, class retryladder.Class) (prebuild, pastBuild bool) {
	entries, err := phasetiming.Read(workspace)
	if err != nil || len(entries) == 0 {
		return false, false
	}
	for _, e := range entries {
		if e.Phase == string(core.PhaseBuild) {
			return false, true
		}
	}
	return class != retryladder.Infra && class != retryladder.Quota, false
}

func routeProtectedAnswers(opts inboxmover.Options, in FailureInputs, stderr io.Writer) []string {
	inboxDir := filepath.Join(in.ProjectRoot, ".evolve", "inbox")
	var routed []string
	for _, d := range committedset.Dispositions(in.Workspace) {
		if !strings.HasPrefix(d.Reason, committedset.ProtectedSurfaceReasonPrefix) {
			continue
		}
		if present, already := itemRouteState(inboxDir, d.ID, os.ReadFile); !present || already {
			continue
		}
		if _, err := inboxmover.RouteConsole(opts, d.ID, d.Reason, in.Cycle); err != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: the protected-surface answer for %s was not routed to the console (%v); it is released\n", d.ID, err)
			continue
		}
		routed = append(routed, d.ID)
	}
	return routed
}
