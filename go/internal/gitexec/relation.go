package gitexec

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type RelationKind string

const (
	RelationCurrent  RelationKind = "current"
	RelationBehind   RelationKind = "behind"
	RelationAhead    RelationKind = "ahead"
	RelationDiverged RelationKind = "diverged"
)

type MainRelation struct {
	Kind   RelationKind
	Local  string
	Remote string
	Ahead  int
	Behind int
	Ref    string
}

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
