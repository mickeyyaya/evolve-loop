package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	inboxRoot     = ".evolve/inbox/"
	consumedInbox = ".evolve/inbox/consumed/"
)

var objectIDRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

type AuditedChange struct {
	Base, Tree, Label string
}

type unwindOutcome int

const (
	unwindNone unwindOutcome = iota
	unwindDone
	unwindKeepsConsumption
)

type unwindVerdict struct {
	declined         string
	keepsConsumption bool
}

func (o *Orchestrator) unwindBeforeFleetRebase(ctx context.Context, projectRoot string, cycle int, cs CycleState) unwindOutcome {
	if cs.ActiveWorktree == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return unwindNone
	}
	tree, err := o.latestAuditedTree(ctx, cs.RunID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ship unwind skipped: read the audited tree: %v\n", cycle, err)
		return unwindNone
	}
	audited := AuditedChange{Base: cs.WorktreeBaseSHA, Tree: tree, Label: fmt.Sprintf("cycle-%d/%s", cycle, cs.RunID)}
	verdict, err := unwindDecline(ctx, cs.ActiveWorktree, audited, gitCapture)
	if err == nil && verdict.declined == "" {
		err = carryAuditedTree(ctx, cs.ActiveWorktree, audited, gitCapture)
	}
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ship unwind skipped: %v\n", cycle, err)
		return unwindNone
	case verdict.keepsConsumption:
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d ship unwind declined: %s; the rebase replays ship's commit and pends it, so the identity proof runs\n", cycle, verdict.declined)
		return unwindKeepsConsumption
	case verdict.declined != "":
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d ship unwind declined: %s\n", cycle, verdict.declined)
		return unwindNone
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d unwound its ship commit to the audited tree %s on %s before the fleet rebase\n", cycle, tree, cs.WorktreeBaseSHA)
	return unwindDone
}

func UnwindToAuditedShape(ctx context.Context, worktree string, audited AuditedChange) (string, error) {
	declined, err := unwindShipCommit(ctx, worktree, audited, gitCapture)
	if declined != "" || err != nil {
		return declined, err
	}
	return "", pendRebasedChange(ctx, worktree, gitCapture)
}

func unwindShipCommit(ctx context.Context, worktree string, audited AuditedChange, git gitFn) (string, error) {
	if verdict, err := unwindDecline(ctx, worktree, audited, git); verdict.declined != "" || err != nil {
		return verdict.declined, err
	}
	return "", carryAuditedTree(ctx, worktree, audited, git)
}

func carryAuditedTree(ctx context.Context, worktree string, audited AuditedChange, git gitFn) error {
	carrier, err := gitStdout(ctx, git, worktree, "-c", "commit.gpgsign=false", "commit-tree", audited.Tree, "-p", audited.Base,
		"-m", "evolve: the audited change, unwound from its ship commit", "-m", "Evolve-Carrier: "+audited.Label)
	if err != nil {
		return err
	}
	_, err = gitStdout(ctx, git, worktree, "reset", "--keep", carrier)
	return err
}

func unwindDecline(ctx context.Context, worktree string, audited AuditedChange, git gitFn) (unwindVerdict, error) {
	declined := func(reason string) (unwindVerdict, error) { return unwindVerdict{declined: reason}, nil }
	if !objectIDRe.MatchString(audited.Base) || !objectIDRe.MatchString(audited.Tree) {
		return declined("the audited base or tree is not an object id")
	}
	status, err := gitStdout(ctx, git, worktree, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return unwindVerdict{}, fmt.Errorf("read the worktree status: %w", err)
	}
	if status != "" {
		return declined("the worktree holds changes or untracked files ship did not commit")
	}
	fork, err := forkPoint(ctx, git, worktree)
	if err != nil {
		return unwindVerdict{}, fmt.Errorf("resolve the fork point: %w", err)
	}
	if fork != audited.Base {
		return declined("the lane did not fork at the audited base")
	}
	_, code, err := git(ctx, worktree, "rev-parse", "--verify", "--quiet", audited.Tree+"^{tree}")
	if err != nil {
		return unwindVerdict{}, fmt.Errorf("look up the audited tree: %w", err)
	}
	if code != 0 {
		return declined("git does not hold the audited tree")
	}
	return consumptionDecline(ctx, worktree, audited.Tree, git)
}

func consumptionDecline(ctx context.Context, worktree, tree string, git gitFn) (unwindVerdict, error) {
	declined := func(reason string) (unwindVerdict, error) { return unwindVerdict{declined: reason}, nil }
	out, err := gitStdout(ctx, git, worktree, "diff-tree", "-r", "-z", "--name-status", "--no-renames", tree, "HEAD")
	if err != nil {
		return unwindVerdict{}, fmt.Errorf("diff the audited tree against HEAD: %w", err)
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	removed, consumed := map[string]bool{}, map[string]string{}
	for i := 0; i+1 < len(fields); i += 2 {
		status, p := fields[i], fields[i+1]
		switch name := path.Base(p); {
		case status == "D" && p == inboxRoot+name:
			removed[name] = true
		case (status == "A" || status == "M") && p == consumedInbox+name:
			consumed[name] = p
		default:
			return declined(fmt.Sprintf("ship's commit changes %s beyond its inbox consumption", p))
		}
	}
	if len(removed) != len(consumed) {
		return declined("the inbox delta is not whole consumption pairs")
	}
	for name, p := range consumed {
		if !removed[name] {
			return declined("the inbox delta is not whole consumption pairs")
		}
		released, err := releasedAContinuation(ctx, worktree, p, git)
		if err != nil {
			return unwindVerdict{}, err
		}
		if released {
			return unwindVerdict{declined: fmt.Sprintf("consumed item %s released a continuation that only ship's commit records", name), keepsConsumption: true}, nil
		}
	}
	return unwindVerdict{}, nil
}

func releasedAContinuation(ctx context.Context, worktree, consumedPath string, git gitFn) (bool, error) {
	body, err := gitStdout(ctx, git, worktree, "show", "HEAD:"+consumedPath)
	if err != nil {
		return false, fmt.Errorf("read consumed item %s: %w", consumedPath, err)
	}
	var item struct {
		Released []json.RawMessage `json:"released_continuations"`
	}
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		return false, fmt.Errorf("read consumed item %s: %w", consumedPath, err)
	}
	return len(item.Released) > 0, nil
}

// pendRebasedChange soft-resets the carrier onto the commit it sits on, so the change is pending again: the
// shape Audit binds and ship commits. After a clean replay that is the new base; after an aborted one it is
// the audited base, which restores the audited shape.
func pendRebasedChange(ctx context.Context, worktree string, git gitFn) error {
	onto, err := forkPoint(ctx, git, worktree)
	if err != nil {
		return err
	}
	_, err = gitStdout(ctx, git, worktree, "reset", "--soft", onto)
	return err
}
