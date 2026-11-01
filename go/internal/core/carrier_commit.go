package core

import (
	"context"
	"fmt"
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const carrierTrailerKey = "Evolve-Carrier"

func carrierLabel(cycle int, runID string) string {
	return fmt.Sprintf("cycle-%d/%s", cycle, runID)
}

func commitCarrier(ctx context.Context, git gitFn, worktree string, change AuditedChange, subject string) (string, error) {
	return gitStdout(ctx, git, worktree, "-c", "commit.gpgsign=false", "commit-tree", change.Tree, "-p", change.Base,
		"-m", subject, "-m", carrierTrailerKey+": "+change.Label)
}

func namesMovedMain(code ShipErrorCode) bool {
	return code == CodeAuditBindingHeadMoved || code == CodeGitFleetRebaseNeeded
}

func asFleetRebase(se *ShipError, why string) *ShipError {
	debug := make(map[string]string, len(se.Debug)+1)
	maps.Copy(debug, se.Debug)
	debug[shiperr.OriginalCodeKey] = string(se.Code)
	return &ShipError{
		Code:    CodeGitFleetRebaseNeeded,
		Class:   ShipClassTransient,
		Stage:   se.Stage,
		Message: why + ": " + se.Message,
		Debug:   debug,
	}
}
