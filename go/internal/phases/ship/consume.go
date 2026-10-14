package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

func consumesInboxAtLanding(opts *Options) bool {
	switch opts.Class {
	case ClassCycle:
		return true
	case ClassManual:
		return opts.WorkspacePath != ""
	default:
		return false
	}
}

func consumeCommittedItems(ctx context.Context, opts *Options, res *RunResult, dir string) {
	if !consumesInboxAtLanding(opts) {
		return
	}
	if v := workspaceACSVerdict(opts.WorkspacePath); v != "PASS" {
		if v == "" {
			res.Logs = append(res.Logs, "[ship] WARN: inbox consumption skipped: acs-verdict.json missing/unreadable in workspace — no verdict, no consuming")
		} else {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] inbox consumption skipped: acs verdict %s (only PASS consumes — partial work stays pickable)", v))
		}
		return
	}
	root := dir
	var prefix []string
	if dir == "" {
		root = opts.ProjectRoot
	} else {
		prefix = []string{"-C", dir}
	}
	body, note := triageDecisionBytes(opts.WorkspacePath, 0)
	if body == nil && note != "" {
		res.Logs = append(res.Logs, note)
	}
	ids := committedInboxIDs(opts.WorkspacePath, body, true)
	if len(ids) == 0 {
		return
	}
	c := &itemConsumer{ctx: ctx, opts: opts, res: res, prefix: prefix, root: root,
		inboxDir: filepath.Join(root, filepath.FromSlash(inboxRel)), cid: stateString(mustStateMap(opts), "cycle_id")}
	members := validatedUnifiedMembers(body)
	unified := make(map[string]bool, len(members))
	for _, id := range members {
		unified[id] = true
	}
	var unitIDs []string
	for _, id := range ids {
		if unified[id] {
			unitIDs = append(unitIDs, id)
			continue
		}
		if m, outcome := c.stage(id); outcome == moveStaged {
			c.record(m)
		}
	}
	if len(unitIDs) > 0 {
		c.consumeUnified(members, unitIDs)
	}
}

const (
	inboxRel    = ".evolve/inbox"
	consumedRel = inboxRel + "/consumed"
)

func consumptionPair(name string) (src, dst string) {
	return inboxRel + "/" + name, consumedRel + "/" + name
}

type itemConsumer struct {
	ctx      context.Context
	opts     *Options
	res      *RunResult
	prefix   []string
	root     string
	inboxDir string
	cid      string
}

type consumedMove struct {
	id, src, dstAbs, srcRel, dstRel string
	raw                             []byte
	bound                           continuation.Continuation
	boundOK                         bool
}

type moveOutcome int

const (
	moveStaged moveOutcome = iota
	moveAbsent
	moveFailed
)

func validatedUnifiedMembers(body []byte) []string {
	var doc struct {
		Commitment *struct {
			Members []struct {
				ID string `json:"id"`
			} `json:"members"`
		} `json:"unified_commitment"`
		Projection json.RawMessage `json:"unified_projection"`
	}
	if json.Unmarshal(body, &doc) != nil || doc.Commitment == nil || len(doc.Projection) == 0 || string(doc.Projection) == "null" {
		return nil
	}
	var ids []string
	for _, m := range doc.Commitment.Members {
		if id := strings.TrimSpace(m.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func (c *itemConsumer) consumeUnified(members, committed []string) {
	inSet := make(map[string]bool, len(committed))
	for _, id := range committed {
		inSet[id] = true
	}
	var staged []consumedMove
	for _, id := range members {
		reason := "is not in this ship's committed set"
		if inSet[id] {
			m, outcome := c.stage(id)
			if outcome == moveStaged {
				staged = append(staged, m)
				continue
			}
			if outcome == moveAbsent && c.alreadyConsumed(id) {
				continue
			}
			reason = "could not close (see the consume WARN above)"
			if outcome == moveAbsent {
				reason = "is unresolvable at the inbox root (absent or unparseable)"
			}
		}
		for _, m := range staged {
			c.rollback(m)
		}
		c.res.Logs = append(c.res.Logs, fmt.Sprintf("[ship] WARN: unified_commitment member %q %s — rolled back %d staged sibling(s); no member is consumed, all stay pickable", id, reason, len(staged)))
		return
	}
	for _, m := range staged {
		c.record(m)
	}
}

func (c *itemConsumer) alreadyConsumed(id string) bool {
	_, err := inboxmover.FindFileByTaskID(filepath.Join(c.inboxDir, "consumed"), id)
	return err == nil
}

func (c *itemConsumer) rollback(m consumedMove) {
	_ = os.Remove(m.dstAbs)
	if err := os.WriteFile(m.src, m.raw, 0o644); err != nil {
		c.res.Logs = append(c.res.Logs, fmt.Sprintf("[ship] WARN: unified_commitment rollback write %q: %v", m.id, err))
	}
	args := append(append([]string{}, c.prefix...), "add", "-A", "--", m.srcRel, m.dstRel)
	if exit, err := c.opts.run(c.ctx, "git", args, io.Discard, io.Discard); err != nil || exit != 0 {
		c.res.Logs = append(c.res.Logs, fmt.Sprintf("[ship] WARN: unified_commitment rollback restage %q rc=%d err=%v — the index may still carry the move", m.id, exit, err))
	}
}

func (c *itemConsumer) record(m consumedMove) {
	c.opts.internalConsumedPaths = append(c.opts.internalConsumedPaths, m.srcRel, m.dstRel)
	c.res.Logs = append(c.res.Logs, fmt.Sprintf("[ship] OK: consumed inbox item %q into %s (rides this ship commit)", m.id, m.dstRel))
	if m.boundOK {
		releaseBindingForConsume(c.opts, c.res, m.id, m.bound)
	}
}

var (
	readInboxItem    = os.ReadFile
	marshalInboxItem = json.MarshalIndent
)

func (c *itemConsumer) stage(id string) (consumedMove, moveOutcome) {
	opts, res := c.opts, c.res
	src, err := inboxmover.FindFileByTaskID(c.inboxDir, id)
	if err != nil {
		if !errors.Is(err, inboxmover.ErrNotFound) {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume lookup %q: %v", id, err))
			return consumedMove{}, moveFailed
		}
		return consumedMove{}, moveAbsent
	}
	raw, err := readInboxItem(src)
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume read %q: %v", id, err))
		return consumedMove{}, moveFailed
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume parse %q: %v — leaving item in place", id, err))
		return consumedMove{}, moveFailed
	}
	doc[inboxbatch.ConsumedField] = map[string]any{
		"at":    time.Now().UTC().Format(time.RFC3339),
		"via":   "ship",
		"cycle": c.cid,
	}
	m := consumedMove{id: id, src: src, raw: raw}
	m.bound, m.boundOK = readBindingForConsume(opts, res, id)
	if m.boundOK {
		doc[inboxbatch.ReleasedContinuationsField] = appendReleasedForConsume(doc, m.bound, c.cid)
	}
	out, err := marshalInboxItem(doc, "", "  ")
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume marshal %q: %v", id, err))
		return consumedMove{}, moveFailed
	}
	base := filepath.Base(src)
	m.dstAbs = filepath.Join(c.root, filepath.FromSlash(consumedRel), base)
	if err := os.MkdirAll(filepath.Dir(m.dstAbs), 0o755); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume mkdir for %q: %v", id, err))
		return consumedMove{}, moveFailed
	}
	if err := os.WriteFile(m.dstAbs, append(out, '\n'), 0o644); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume write %q: %v", id, err))
		return consumedMove{}, moveFailed
	}
	if err := os.Remove(src); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume remove root %q: %v", id, err))
		_ = os.Remove(m.dstAbs)
		return consumedMove{}, moveFailed
	}
	m.srcRel, m.dstRel = consumptionPair(base)
	if opts.internalAuditBoundTreeSHA != "" {
		boundArgs := append(append([]string{}, c.prefix...), "show", opts.internalAuditBoundTreeSHA+":"+m.srcRel)
		var boundOut strings.Builder
		exit, runErr := opts.run(c.ctx, "git", boundArgs, &boundOut, io.Discard)
		if runErr != nil || exit != 0 || boundOut.String() != string(raw) {
			_ = os.Remove(m.dstAbs)
			if werr := os.WriteFile(src, raw, 0o644); werr != nil {
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume provenance rollback write %q: %v", id, werr))
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q REFUSED sanctioning: item bytes are not what the audit-bound tree carries (absent or tampered post-binding) — move rolled back, item stays pickable", id))
			return consumedMove{}, moveFailed
		}
	}
	args := append(append([]string{}, c.prefix...), "add", "-A", "--", m.srcRel, m.dstRel)
	if exit, runErr := opts.run(c.ctx, "git", args, io.Discard, io.Discard); runErr != nil || exit != 0 {
		_ = os.Remove(m.dstAbs)
		if werr := os.WriteFile(src, raw, 0o644); werr != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume rollback write %q: %v", id, werr))
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume stage %q rc=%d err=%v — move rolled back, item stays pickable", id, exit, runErr))
		return consumedMove{}, moveFailed
	}
	return m, moveStaged
}

func readBindingForConsume(opts *Options, res *RunResult, id string) (continuation.Continuation, bool) {
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, id)
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation registry unreadable (%v) — binding NOT released", id, err))
		return continuation.Continuation{}, false
	}
	return c, ok
}

func appendReleasedForConsume(doc map[string]any, c continuation.Continuation, cycleID string) []any {
	return continuation.AppendReleased(doc, c, "ship-consume-"+cycleID)
}

func releaseBindingForConsume(opts *Options, res *RunResult, id string, c continuation.Continuation) {
	released, err := continuation.DeleteRegistryEntryIfCycle(opts.ProjectRoot, id, c.Cycle)
	switch {
	case err != nil:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation binding release failed: %v", id, err))
	case !released:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation binding was rebound by another lane (cycle %d no longer owns it) — left intact", id, c.Cycle))
	default:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: released continuation binding for %q (snapshot %s, cycle %d) — pointer preserved in the consumed item", id, c.SnapshotSHA, c.Cycle))
	}
}

func treeDriftExplainedByConsumption(ctx context.Context, opts *Options, gitDir, boundTree, actualTree string) (bool, string) {
	if len(opts.internalConsumedPaths) == 0 {
		return false, ""
	}
	sanctioned := make(map[string]bool, len(opts.internalConsumedPaths))
	for _, p := range opts.internalConsumedPaths {
		sanctioned[p] = true
	}
	args := []string{}
	if gitDir != "" {
		args = append(args, "-C", gitDir)
	}
	args = append(args, shipmanifest.RawPathRead("diff-tree", "-r", "--name-only", boundTree, actualTree)...)
	var out strings.Builder
	if exit, err := opts.run(ctx, "git", args, &out, io.Discard); err != nil || exit != 0 {
		return false, ""
	}
	var offenders []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		p := shipmanifest.UnquoteGitPath(strings.TrimSpace(line))
		if p == "" {
			continue
		}
		if !sanctioned[p] {
			offenders = append(offenders, p)
		}
	}
	if len(offenders) == 0 {
		return true, ""
	}
	const maxNamed = 8
	extra := ""
	if len(offenders) > maxNamed {
		extra = fmt.Sprintf(" and %d more", len(offenders)-maxNamed)
		offenders = offenders[:maxNamed]
	}
	return false, " (unsanctioned drift path(s): " + strings.Join(offenders, ", ") + extra + ")"
}

func adoptIntentConsumption(opts *Options, paths []string) error {
	if len(paths)%2 != 0 {
		return fmt.Errorf("%d consumed path(s) are not whole inbox consumption pairs", len(paths))
	}
	for i := 0; i < len(paths); i += 2 {
		name := filepath.Base(paths[i])
		if src, dst := consumptionPair(name); !strings.HasSuffix(name, ".json") || paths[i] != src || paths[i+1] != dst {
			return fmt.Errorf("%s -> %s is not an inbox consumption pair", paths[i], paths[i+1])
		}
	}
	opts.internalConsumedPaths = append([]string(nil), paths...)
	return nil
}

func workspaceACSVerdict(workspace string) string {
	raw, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Verdict string `json:"verdict"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return ""
	}
	return doc.Verdict
}

func mustStateMap(opts *Options) map[string]any {
	m, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return map[string]any{}
	}
	return m
}
