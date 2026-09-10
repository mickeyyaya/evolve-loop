package gitexec

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// RelationKind classifies the local checkout's history against a remote ref.
type RelationKind string

const (
	RelationCurrent  RelationKind = "current"  // HEAD == remote
	RelationBehind   RelationKind = "behind"   // HEAD is an ancestor of remote (fast-forwardable)
	RelationAhead    RelationKind = "ahead"    // remote is an ancestor of HEAD (unpublished local commits)
	RelationDiverged RelationKind = "diverged" // neither contains the other
)

// MainRelation is the ONE resolution of "where is the local main relative to
// origin/main" — the wave boundary (fast-forward or not, halt or not) and the
// lane base (which ref a fresh lane starts from) both read this struct and
// render its String, so the two can never disagree on the state or the words
// (2026-09-09 token-waste root cause #3).
type MainRelation struct {
	Kind   RelationKind
	Local  string // HEAD sha
	Remote string // remote ref sha
	Ahead  int    // commits on HEAD not on remote
	Behind int    // commits on remote not on HEAD
	Ref    string // the remote ref compared against (e.g. "origin/main")
}

// RelationToRemote resolves HEAD against remoteRef (already fetched by the
// caller). Every git failure is an error — the callers decide their own
// fail-open disposition; nothing here guesses a state.
func (g Git) RelationToRemote(ctx context.Context, remoteRef string) (MainRelation, error) {
	rel := MainRelation{Ref: remoteRef}
	local, _, code, err := g.Capture(ctx, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return rel, fmt.Errorf("rev-parse HEAD: rc=%d: %w", code, err)
	}
	remote, _, code, err := g.Capture(ctx, "rev-parse", remoteRef)
	if err != nil || code != 0 {
		return rel, fmt.Errorf("rev-parse %s: rc=%d: %w", remoteRef, code, err)
	}
	rel.Local, rel.Remote = strings.TrimSpace(local), strings.TrimSpace(remote)
	counts, _, code, err := g.Capture(ctx, "rev-list", "--left-right", "--count", "HEAD..."+remoteRef)
	if err != nil || code != 0 {
		return rel, fmt.Errorf("rev-list --left-right --count HEAD...%s: rc=%d: %w", remoteRef, code, err)
	}
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return rel, fmt.Errorf("rev-list --left-right --count: unexpected output %q", strings.TrimSpace(counts))
	}
	ahead, aerr := strconv.Atoi(fields[0])
	behind, berr := strconv.Atoi(fields[1])
	if aerr != nil || berr != nil {
		return rel, fmt.Errorf("rev-list --left-right --count: non-integer counts %q", strings.TrimSpace(counts))
	}
	rel.Ahead, rel.Behind = ahead, behind
	switch {
	case rel.Local == rel.Remote:
		rel.Kind = RelationCurrent
	case rel.Ahead == 0:
		rel.Kind = RelationBehind
	case rel.Behind == 0:
		rel.Kind = RelationAhead
	default:
		rel.Kind = RelationDiverged
	}
	return rel, nil
}

// String renders the relation in the one sentence both consumers print.
func (r MainRelation) String() string {
	switch r.Kind {
	case RelationCurrent:
		return fmt.Sprintf("local main is current with %s (%.12s)", r.Ref, r.Local)
	case RelationBehind:
		return fmt.Sprintf("local main is BEHIND %s by %d commit(s)", r.Ref, r.Behind)
	case RelationAhead:
		return fmt.Sprintf("local main is AHEAD of %s by %d commit(s) (unpublished landings) — the integration HEAD is the local %.12s", r.Ref, r.Ahead, r.Local)
	case RelationDiverged:
		return fmt.Sprintf("local main (%.12s) has DIVERGED from %s (%.12s): %d ahead, %d behind — reconcile the plane (merge %s) before any lane is dispatched; the loop never merges", r.Local, r.Ref, r.Remote, r.Ahead, r.Behind, r.Ref)
	}
	return "local main relation unknown"
}
