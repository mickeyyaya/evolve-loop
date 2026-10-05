package commitgate

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

const recordHistoryCommand = "cd go && go run ./cmd/commentaudit history -base HEAD -label \"<this change>\" -out " + commentaudit.HistoryArchiveDir

type changeSides struct {
	paths         []string
	before, after func(string) ([]byte, error)
}

func (o Options) changeAgainstHead(ctx context.Context, files []string) (changeSides, error) {
	paths, err := o.changeWithRenameSources(ctx, files)
	if err != nil {
		return changeSides{}, err
	}
	return changeSides{paths: paths, before: commentaudit.ReadAtBase(o.git(ctx), "HEAD"), after: o.readOnDisk}, nil
}

func (o Options) refuseUnrecordedHistory(ctx context.Context, files []string, res *Result) int {
	unrecorded, err := o.unrecordedHistory(ctx, files)
	if err != nil {
		res.log("history check: %v", err)
		return historyCheckFault(err)
	}
	for _, e := range unrecorded {
		res.log("REJECTED: %s:%d removes a comment that carries history, and no page under %s records it", e.File, e.Line, commentaudit.HistoryArchiveDir)
	}
	if len(unrecorded) > 0 {
		res.log("the history a comment carried is kept, not deleted: run `%s` and stage the pages it writes (docs/conventions/code-comments.md)", recordHistoryCommand)
		return ExitFail
	}
	return ExitPass
}

func (o Options) unrecordedHistory(ctx context.Context, files []string) ([]commentaudit.HistoryEntry, error) {
	change, err := o.changeAgainstHead(ctx, files)
	if err != nil {
		return nil, err
	}
	removed, err := commentaudit.RemovedHistoryAcrossDiff(change.paths, change.before, change.after)
	if err != nil {
		return nil, err
	}
	return commentaudit.UnrecordedHistory(removed, change.before, change.after)
}

func historyCheckFault(err error) int {
	if commentaudit.IsUnparsable(err) {
		return ExitFail
	}
	return ExitGitFatal
}

func (o Options) refuseRewrittenHistory(ctx context.Context, files []string, res *Result) int {
	change, err := o.changeAgainstHead(ctx, files)
	var rewritten []string
	if err == nil {
		rewritten, err = commentaudit.RewrittenHistoryPages(change.paths, change.before, change.after)
	}
	if err != nil {
		res.log("history archive check: %v", err)
		return ExitGitFatal
	}
	for _, page := range rewritten {
		res.log("REJECTED: %s rewrites the comment history archive: a page only grows, by sections `commentaudit history` appends", page)
	}
	if len(rewritten) > 0 {
		return ExitFail
	}
	return ExitPass
}
