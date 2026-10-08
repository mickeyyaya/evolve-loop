// Package inboxstamps lands the loop's own writes to tracked inbox items in the plane, so they never block a sync with origin.
package inboxstamps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const (
	Pathspec       = ".evolve/inbox/"
	keptMessage    = "chore(inbox): the loop's inbox stamps from the plane"
	replayMessage  = "chore(inbox): the loop's inbox stamps replayed onto origin's edits"
	statusCodeSize = 3
)

type Git interface {
	Capture(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error)
}

type Stamp struct {
	Path    string
	Set     map[string]json.RawMessage
	Removed []string
}

type Partition struct {
	Stamps  []Stamp
	Claimed []Claimed
	Other   []string
}

type Plan struct {
	Kept       []Stamp
	Retired    []Stamp
	Replay     []Stamp
	Superseded []Stamp
	prepared   []Stamp
}

func Classify(ctx context.Context, g Git) (Partition, error) {
	out, err := output(ctx, g, "status", "--porcelain", "-z", "--no-renames", "--untracked-files=no")
	if err != nil {
		return Partition{}, err
	}
	root, err := output(ctx, g, "rev-parse", "--show-toplevel")
	if err != nil {
		return Partition{}, err
	}
	tree := worktree{root: strings.TrimSpace(root)}
	tree.claims = claimsByName(tree.root)
	var partition Partition
	for _, entry := range strings.Split(out, "\x00") {
		if len(entry) <= statusCodeSize {
			continue
		}
		path := entry[statusCodeSize:]
		stamp, ok, err := tree.lifecycleOnlyChange(ctx, g, entry[:2], path)
		if err != nil {
			return Partition{}, err
		}
		claimed, held := tree.claimOf(entry[:2], path)
		switch {
		case ok:
			partition.Stamps = append(partition.Stamps, stamp)
		case held:
			partition.Claimed = append(partition.Claimed, claimed)
		default:
			partition.Other = append(partition.Other, path)
		}
	}
	return partition, nil
}

type worktree struct {
	root   string
	claims map[string]string
}

func (w worktree) lifecycleOnlyChange(ctx context.Context, g Git, status, path string) (Stamp, bool, error) {
	if !strings.HasPrefix(path, Pathspec) || !isModification(status) {
		return Stamp{}, false, nil
	}
	head, headOK, err := itemAt(ctx, g, "HEAD", path)
	if err != nil || !headOK {
		return Stamp{}, false, err
	}
	now, ok := decodeFields(w.read(path))
	if !ok {
		return Stamp{}, false, nil
	}
	stamp := Stamp{Path: path, Set: map[string]json.RawMessage{}}
	for _, key := range changedKeys(head, now) {
		if !inboxmover.IsMoverWritten(key) {
			return Stamp{}, false, nil
		}
		if value, present := now[key]; present {
			stamp.Set[key] = value
		} else {
			stamp.Removed = append(stamp.Removed, key)
		}
	}
	return stamp, len(stamp.Set)+len(stamp.Removed) > 0, nil
}

func isModification(status string) bool {
	return strings.Contains(status, "M") && strings.Trim(status, "M ") == ""
}

func changedKeys(before, after map[string]json.RawMessage) []string {
	keys := slices.Concat(slices.Collect(maps.Keys(before)), slices.Collect(maps.Keys(after)))
	slices.Sort(keys)
	return slices.DeleteFunc(slices.Compact(keys), func(key string) bool {
		a, inBefore := before[key]
		b, inAfter := after[key]
		return inBefore == inAfter && jsonEqual(a, b)
	})
}

func PlanAgainst(ctx context.Context, g Git, stamps []Stamp, remoteRef string) (Plan, error) {
	if len(stamps) == 0 {
		return Plan{}, nil
	}
	changedOnRemote, err := ChangedOnRemote(ctx, g, remoteRef)
	if err != nil {
		return Plan{}, err
	}
	base, err := output(ctx, g, "merge-base", "HEAD", remoteRef)
	if err != nil {
		return Plan{}, err
	}
	remote := remoteSide{g: g, base: strings.TrimSpace(base), ref: remoteRef}
	var plan Plan
	for _, stamp := range stamps {
		if !changedOnRemote[stamp.Path] {
			plan.Kept = append(plan.Kept, stamp)
			continue
		}
		origin, err := remote.itemAt(ctx, stamp.Path)
		if err != nil {
			return Plan{}, err
		}
		plan.sortChanged(stamp, origin)
	}
	return plan, nil
}

type remoteSide struct {
	g         Git
	base, ref string
}

type remoteItem struct {
	atBase, now map[string]json.RawMessage
	present     bool
}

func (r remoteSide) itemAt(ctx context.Context, path string) (remoteItem, error) {
	now, present, err := itemAt(ctx, r.g, r.ref, path)
	if err != nil {
		return remoteItem{}, err
	}
	atBase, _, err := itemAt(ctx, r.g, r.base, path)
	if err != nil {
		return remoteItem{}, err
	}
	return remoteItem{atBase: atBase, now: now, present: present}, nil
}

func (p *Plan) sortChanged(stamp Stamp, origin remoteItem) {
	p.prepared = append(p.prepared, stamp)
	replay := withoutFieldsOriginChanged(stamp, origin)
	switch {
	case !origin.present:
		p.Retired = append(p.Retired, stamp)
	case len(replay.Set)+len(replay.Removed) == 0:
		p.Superseded = append(p.Superseded, stamp)
	default:
		p.Replay = append(p.Replay, replay)
	}
}

func withoutFieldsOriginChanged(stamp Stamp, origin remoteItem) Stamp {
	originChanged := changedKeys(origin.atBase, origin.now)
	kept := Stamp{Path: stamp.Path, Set: map[string]json.RawMessage{}}
	for key, value := range stamp.Set {
		if !slices.Contains(originChanged, key) {
			kept.Set[key] = value
		}
	}
	for _, key := range stamp.Removed {
		if !slices.Contains(originChanged, key) {
			kept.Removed = append(kept.Removed, key)
		}
	}
	return kept
}

func (p Plan) Prepare(ctx context.Context, g Git) error {
	paths := Paths(p.prepared)
	if len(paths) == 0 {
		return nil
	}
	return run(ctx, g, append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)...)
}

func (p Plan) CommitKept(ctx context.Context, g Git) error {
	return commitPaths(ctx, g, keptMessage, Paths(p.Kept))
}

func (p Plan) ApplyReplay(root string) error {
	return applyStamps(root, p.Replay)
}

func (p Plan) Restore(root string) error {
	return applyStamps(root, p.prepared)
}

func applyStamps(root string, stamps []Stamp) error {
	var failures []error
	for _, stamp := range stamps {
		err := inboxmover.UpdateItemJSON(filepath.Join(root, stamp.Path), func(item map[string]json.RawMessage) {
			maps.Copy(item, stamp.Set)
			for _, key := range stamp.Removed {
				delete(item, key)
			}
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("apply the stamp on %s: %w", stamp.Path, err))
		}
	}
	return errors.Join(failures...)
}

func (p Plan) CommitReplay(ctx context.Context, g Git) error {
	return commitPaths(ctx, g, replayMessage, Paths(p.Replay))
}

func ChangedOnRemote(ctx context.Context, g Git, remoteRef string) (map[string]bool, error) {
	out, err := output(ctx, g, "diff", "-z", "--no-renames", "--name-only", "HEAD..."+remoteRef, "--", Pathspec)
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			changed[path] = true
		}
	}
	return changed, nil
}

func Paths(stamps []Stamp) []string {
	paths := make([]string, len(stamps))
	for i, stamp := range stamps {
		paths[i] = stamp.Path
	}
	return paths
}

func commitPaths(ctx context.Context, g Git, message string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	return run(ctx, g, append([]string{"commit", "-q", "-m", message, "--"}, paths...)...)
}

func itemAt(ctx context.Context, g Git, rev, path string) (map[string]json.RawMessage, bool, error) {
	if _, _, code, err := g.Capture(ctx, "cat-file", "-e", rev+":"+path); err != nil || code != 0 {
		return nil, false, err
	}
	body, err := output(ctx, g, "show", rev+":"+path)
	if err != nil {
		return nil, false, err
	}
	fields, ok := decodeFields([]byte(body))
	if !ok {
		return nil, false, fmt.Errorf("%s at %s is not a JSON object", path, rev)
	}
	return fields, true, nil
}

func (w worktree) read(path string) []byte {
	body, err := os.ReadFile(filepath.Join(w.root, path))
	if err != nil {
		return nil
	}
	return body
}

func decodeFields(body []byte) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return nil, false
	}
	return fields, true
}

func jsonEqual(a, b json.RawMessage) bool {
	var valueA, valueB any
	if json.Unmarshal(a, &valueA) != nil || json.Unmarshal(b, &valueB) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(valueA, valueB)
}

func output(ctx context.Context, g Git, args ...string) (string, error) {
	stdout, stderr, code, err := g.Capture(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	if code != 0 {
		return "", fmt.Errorf("git %s: rc=%d: %s", args[0], code, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

func run(ctx context.Context, g Git, args ...string) error {
	_, err := output(ctx, g, args...)
	return err
}
