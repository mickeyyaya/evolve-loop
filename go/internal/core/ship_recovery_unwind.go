package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	inboxRoot     = ".evolve/inbox/"
	consumedInbox = ".evolve/inbox/consumed/"
)

var objectIDRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// auditedChange is what the newest audit bound: the base the lane forked at, the tree it reviewed, and a
// label that names the cycle and run in the carrier commit's trailer.
type auditedChange struct {
	base, tree, label string
}

// unwindShipCommit replaces the lane's commits with one carrier commit of the audited tree on the audited
// base. It returns why it declined, changing nothing, or "" once HEAD is the carrier.
func unwindShipCommit(ctx context.Context, worktree string, audited auditedChange, git gitFn) (string, error) {
	if declined, err := unwindDecline(ctx, worktree, audited, git); declined != "" || err != nil {
		return declined, err
	}
	carrier, err := gitStdout(ctx, git, worktree, "-c", "commit.gpgsign=false", "commit-tree", audited.tree, "-p", audited.base,
		"-m", "evolve: the audited change, unwound from its ship commit", "-m", "Evolve-Carrier: "+audited.label)
	if err != nil {
		return "", err
	}
	_, err = gitStdout(ctx, git, worktree, "reset", "--keep", carrier)
	return "", err
}

func unwindDecline(ctx context.Context, worktree string, audited auditedChange, git gitFn) (string, error) {
	if !objectIDRe.MatchString(audited.base) || !objectIDRe.MatchString(audited.tree) {
		return "the audited base or tree is not an object id", nil
	}
	status, err := gitStdout(ctx, git, worktree, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return "", fmt.Errorf("read the worktree status: %w", err)
	}
	if status != "" {
		return "the worktree holds changes or untracked files ship did not commit", nil
	}
	fork, err := forkPoint(ctx, git, worktree)
	if err != nil {
		return "", fmt.Errorf("resolve the fork point: %w", err)
	}
	if fork != audited.base {
		return "the lane did not fork at the audited base", nil
	}
	_, code, err := git(ctx, worktree, "rev-parse", "--verify", "--quiet", audited.tree+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("look up the audited tree: %w", err)
	}
	if code != 0 {
		return "git does not hold the audited tree", nil
	}
	return consumptionDecline(ctx, worktree, audited.tree, git)
}

// consumptionDecline holds unless HEAD differs from the audited tree by exactly ship's inbox consumption:
// each item removed from the inbox root and written to consumed/ under the same name. A consumed item that
// released a continuation declines too, because its pointer lives only in ship's commit.
func consumptionDecline(ctx context.Context, worktree, tree string, git gitFn) (string, error) {
	out, err := gitStdout(ctx, git, worktree, "diff-tree", "-r", "-z", "--name-status", "--no-renames", tree, "HEAD")
	if err != nil {
		return "", fmt.Errorf("diff the audited tree against HEAD: %w", err)
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
			return fmt.Sprintf("ship's commit changes %s beyond its inbox consumption", p), nil
		}
	}
	if len(removed) != len(consumed) {
		return "the inbox delta is not whole consumption pairs", nil
	}
	for name, p := range consumed {
		if !removed[name] {
			return "the inbox delta is not whole consumption pairs", nil
		}
		released, err := releasedAContinuation(ctx, worktree, p, git)
		if err != nil {
			return "", err
		}
		if released {
			return fmt.Sprintf("consumed item %s released a continuation that only ship's commit records", name), nil
		}
	}
	return "", nil
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
