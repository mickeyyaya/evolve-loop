package ship

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const committedDocsDest = "docs/"

var committedDocRoots = []string{committedDocsDest, "knowledge-base/"}

func refuseCommittedDocDeletion(ctx context.Context, opts *Options, revs ...string) error {
	out, err := captureGitOutput(ctx, opts, append([]string{"diff", "--name-status", "-z", "-M", "-l0", "--diff-filter=DR"}, revs...)...)
	if err != nil {
		return err
	}
	deleted := committedDocDeletions(out)
	if len(deleted) == 0 {
		return nil
	}
	return shipErr(shiperr.CodeCommittedDocDeleted, core.ShipClassPrecondition, core.StageAtomicShip,
		"ship: refusing to land the deletion of committed documentation: "+strings.Join(deleted, ", ")+
			"; archive it with `git mv <file> docs/private/research/archived-YYYY-MM-DD/<file>` instead",
		"paths", strings.Join(deleted, ","))
}

func committedDocDeletions(nameStatus string) []string {
	var deleted []string
	fields := strings.Split(nameStatus, "\x00")
	for len(fields) > 1 {
		status, paths := fields[0], fields[1:]
		n := 1
		if strings.HasPrefix(status, "R") {
			n = 2
		}
		if len(paths) < n {
			break
		}
		if underCommittedDocRoot(paths[0]) && (n == 1 || !strings.HasPrefix(paths[1], committedDocsDest)) {
			deleted = append(deleted, paths[0])
		}
		fields = paths[n:]
	}
	return deleted
}

func underCommittedDocRoot(path string) bool {
	lower := strings.ToLower(path)
	for _, root := range committedDocRoots {
		if strings.HasPrefix(lower, root) {
			return true
		}
	}
	return false
}
