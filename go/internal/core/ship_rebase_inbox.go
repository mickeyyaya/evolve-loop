package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeFleetRebaseInboxResolved signalcenter.Code = "ORCHESTRATOR_FLEET_REBASE_INBOX_RESOLVED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeFleetRebaseInboxResolved,
		"the fleet rebase of the ship recovery met a conflict on the inbox item of the cycle itself, which the plane also stamped on main, and resolved it by a stated rule: main's stamps stay, the cycle's field changes and its consumption land, and the code tree is not touched; WARN; fields.paths (the resolved inbox paths)")
}

type inboxStamps struct{ own map[string]bool }

func newInboxStamps(ids []string) inboxStamps {
	own := make(map[string]bool, len(ids))
	for _, id := range ids {
		own[id] = true
	}
	return inboxStamps{own: own}
}

func (s inboxStamps) resolve(ctx context.Context, worktree string, git gitFn, unmerged []string) (left, settled []string, err error) {
	r := stageReader{worktree: worktree, git: git}
	for _, p := range unmerged {
		done := false
		if path.Dir(p)+"/" == inboxRoot {
			if done, err = s.settle(ctx, r, p); err != nil {
				return nil, nil, fmt.Errorf("settle %s: %w", p, err)
			}
		}
		if done {
			settled = append(settled, p)
			continue
		}
		left = append(left, p)
	}
	return left, settled, nil
}

func (s inboxStamps) settle(ctx context.Context, r stageReader, p string) (bool, error) {
	stages, err := r.stages(ctx, p)
	if err != nil || !stages[2] {
		return false, err
	}
	ours, err := r.blob(ctx, 2, p)
	if err != nil || !s.owns(ours) {
		return false, err
	}
	if !stages[3] {
		return r.consume(ctx, p)
	}
	base := ""
	if stages[1] {
		if base, err = r.blob(ctx, 1, p); err != nil {
			return false, err
		}
	}
	return r.merge(ctx, p, itemSides{base: base, ours: ours})
}

type itemSides struct{ base, ours string }

func (s inboxStamps) owns(item string) bool {
	var head struct {
		ID string `json:"id"`
	}
	return json.Unmarshal([]byte(item), &head) == nil && head.ID != "" && s.own[head.ID]
}

type stageReader struct {
	worktree string
	git      gitFn
}

func (r stageReader) out(ctx context.Context, args ...string) (string, error) {
	out, code, err := r.git(ctx, r.worktree, args...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git %s: exit %d", strings.Join(args, " "), code)
	}
	return out, nil
}

func (r stageReader) stages(ctx context.Context, p string) (map[int]bool, error) {
	out, err := r.out(ctx, "ls-files", "-u", "-z", "--", p)
	if err != nil {
		return nil, err
	}
	found := map[int]bool{}
	for _, entry := range nulPaths(out) {
		meta, _, _ := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if len(fields) == 0 {
			continue
		}
		if n, convErr := strconv.Atoi(fields[len(fields)-1]); convErr == nil {
			found[n] = true
		}
	}
	return found, nil
}

func (r stageReader) blob(ctx context.Context, stage int, p string) (string, error) {
	return r.out(ctx, "show", fmt.Sprintf(":%d:%s", stage, p))
}

func (r stageReader) consume(ctx context.Context, p string) (bool, error) {
	held, err := r.out(ctx, "ls-files", "-z", "--", consumedInbox+path.Base(p))
	if err != nil || held == "" {
		return false, err
	}
	_, err = r.out(ctx, "rm", "-q", "--", p)
	return err == nil, err
}

func (r stageReader) merge(ctx context.Context, p string, sides itemSides) (bool, error) {
	theirs, err := r.blob(ctx, 3, p)
	if err != nil {
		return false, err
	}
	merged, ok := mergeInboxItem(sides.base, sides.ours, theirs)
	if !ok {
		return false, nil
	}
	if err := os.WriteFile(filepath.Join(r.worktree, filepath.FromSlash(p)), merged, 0o644); err != nil {
		return false, err
	}
	_, err = r.out(ctx, "add", "--", p)
	return err == nil, err
}

func mergeInboxItem(base, ours, theirs string) ([]byte, bool) {
	var b, o, t map[string]json.RawMessage
	if base == "" {
		base = "{}"
	}
	for i, into := range []*map[string]json.RawMessage{&b, &o, &t} {
		if json.Unmarshal([]byte([]string{base, ours, theirs}[i]), into) != nil {
			return nil, false
		}
	}
	if o == nil || t == nil {
		return nil, false
	}
	id := o["id"]
	for k, tv := range t {
		if !bytes.Equal(tv, b[k]) && bytes.Equal(o[k], b[k]) {
			o[k] = tv
		}
	}
	for k := range b {
		if _, kept := t[k]; !kept && bytes.Equal(o[k], b[k]) {
			delete(o, k)
		}
	}
	if len(id) == 0 || !bytes.Equal(o["id"], id) {
		return nil, false
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(o)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), true
}

type rebaseReport struct {
	signals *signalcenter.Center
	cycle   int
	origin  string
}

func (r rebaseReport) run(ctx context.Context, projectRoot string, cs *CycleState) (ok, conflict bool) {
	ok, conflict = reauditRebase(ctx, projectRoot, cs)
	if len(cs.ShipRecoveryStamped) > 0 {
		r.signals.Emit(signalcenter.Event{
			Cycle: r.cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator, Origin: r.origin,
			Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeFleetRebaseInboxResolved,
			Reason: "fleet rebase resolved the inbox stamps of the cycle's own item", Fields: map[string]string{"paths": strings.Join(cs.ShipRecoveryStamped, ",")},
		})
	}
	return ok, conflict
}
