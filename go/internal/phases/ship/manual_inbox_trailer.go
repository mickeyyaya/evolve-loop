package ship

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func fillManualBranchInboxTrailer(ctx context.Context, opts *Options) {
	if closesinbox.Declares([]byte(opts.CommitMessage)) {
		return
	}
	branch, err := currentBranch(ctx, opts)
	if err != nil || branch == "" {
		return
	}
	id := path.Base(branch)
	scan, err := inboxbatch.ScanDir(filepath.Join(opts.ProjectRoot, ".evolve", "inbox"))
	if err != nil {
		return
	}
	for _, it := range scan.Items {
		if it.ID == id {
			opts.CommitMessage = strings.TrimRight(opts.CommitMessage, "\n") + "\n\nCloses-Inbox: " + id
			return
		}
	}
}
