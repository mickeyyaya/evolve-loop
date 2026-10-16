package explanationdocs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var ErrRebindIncomplete = errors.New("identity-preserving rebind incomplete")

func RebindIdenticalRebase(ctx context.Context, binding CycleBinding, newBaseSHA string, persist func() error) (bool, error) {
	if persist == nil {
		return false, errors.New("rebind requires a checkpoint persist step: the snapshot moves, so the cycle state must move with it")
	}
	binding, active, err := resolveBuildActivation(binding)
	if err != nil {
		return false, err
	}
	if !active {
		return false, errors.New("cannot rebind a Build contract without an active host contract")
	}
	next, identical, err := proveIdenticalRebase(ctx, binding, newBaseSHA)
	if err != nil || !identical {
		return false, err
	}
	if err := applyRebind(binding, next, persist); err != nil {
		return false, fmt.Errorf("%w: %v", ErrRebindIncomplete, err)
	}
	return true, nil
}

func proveIdenticalRebase(ctx context.Context, binding CycleBinding, newBaseSHA string) (*resultSnapshot, bool, error) {
	if err := validateBaseCommit(ctx, binding.Worktree, newBaseSHA); err != nil {
		return nil, false, err
	}
	if newBaseSHA == binding.BaseSHA {
		return nil, false, nil
	}
	prior, err := readResultSnapshot(binding)
	if err != nil {
		return nil, false, err
	}
	authored := authoredBase(&prior.View)
	changed, err := changedSince(ctx, binding.Worktree, newBaseSHA)
	if err != nil {
		return nil, false, err
	}
	if ok, err := lineageHolds(ctx, binding.Worktree, authored, newBaseSHA, changed); err != nil || !ok {
		return nil, false, err
	}
	if ok, err := isAncestor(ctx, binding.Worktree, binding.BaseSHA, newBaseSHA); err != nil || !ok {
		return nil, false, err
	}
	states, err := readPathStates(ctx, binding.Worktree, changed)
	if err != nil {
		return nil, false, err
	}
	if ok, err := sameChange(ctx, binding, prior, changed, states, newBaseSHA); err != nil || !ok {
		return nil, false, err
	}
	next := *prior
	next.View.BaseSHA, next.View.DiffSHA256, next.View.AuthoredBaseSHA = newBaseSHA, foldPathStates(diffDomain(newBaseSHA), states), authored
	return &next, true, nil
}

func sameChange(ctx context.Context, binding CycleBinding, prior *resultSnapshot, changed []string, states []pathState, newBaseSHA string) (bool, error) {
	ships, err := shipConsumption(ctx, binding.Worktree, newBaseSHA, states)
	if err != nil {
		return false, err
	}
	if foldPathStates(diffDomain(prior.View.BaseSHA), withoutShipConsumption(states, ships)) != prior.View.DiffSHA256 {
		return false, nil
	}
	material := materialPaths(changed)
	if !samePaths(material, prior.View.MaterialPaths) {
		return false, nil
	}
	if foldPathStates(materialDomain, statesOf(states, material)) != prior.MaterialSHA256 {
		return false, nil
	}
	report, err := readBuildReport(binding.Workspace)
	if err != nil {
		return false, err
	}
	declaration, present, err := parseDeclaration(report)
	if err != nil || !present || !declares(declaration, &prior.View) {
		return false, err
	}
	if prior.View.Status != statusRequired {
		return true, nil
	}
	exists, err := existsAtBase(ctx, binding.Worktree, newBaseSHA, prior.View.DocumentPath)
	return err == nil && !exists, err
}

func applyRebind(binding CycleBinding, next *resultSnapshot, persist func() error) error {
	if err := advanceMarker(binding, next.View.BaseSHA, persist); err != nil {
		return err
	}
	rebound := binding
	rebound.BaseSHA = next.View.BaseSHA
	if err := writeManifest(rebound, &next.View); err != nil {
		return err
	}
	return atomicwrite.JSON(resultSnapshotPath(binding.ProjectRoot, binding.Cycle), next)
}

func authoredBase(view *phaseio.ExplanationView) string {
	if view.AuthoredBaseSHA != "" {
		return view.AuthoredBaseSHA
	}
	return view.BaseSHA
}

func lineageHolds(ctx context.Context, worktree, authored, base string, changed []string) (bool, error) {
	if authored == base {
		return false, nil
	}
	if ok, err := isAncestor(ctx, worktree, authored, base); err != nil || !ok {
		return false, err
	}
	cmd := sysexec.Command(ctx, "git", "-C", worktree, "diff", "--no-renames", "--name-only", "-z", authored, base, "--")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git diff %s %s --name-only: %w: %s", authored, base, err, strings.TrimSpace(stderr.String()))
	}
	return !peerTouchesLane(splitNUL(out), changed), nil
}

func peerTouchesLane(peer, lane []string) bool {
	folded := make(map[string]bool, len(lane))
	for _, p := range lane {
		if !IsPlainPath(p) {
			return true
		}
		folded[strings.ToLower(p)] = true
	}
	for _, p := range peer {
		lower := strings.ToLower(p)
		if name := path.Base(lower); !IsPlainPath(p) || name == ".gitattributes" || name == ".gitignore" || folded[lower] {
			return true
		}
	}
	return false
}

func IsPlainPath(p string) bool {
	for i := 0; i < len(p); i++ {
		if c := p[i]; c >= 0x80 || c == ':' || c == '\\' || c == '~' {
			return false
		}
	}
	for _, component := range strings.Split(p, "/") {
		if len(component) > maxPlainComponentBytes || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return false
		}
	}
	return true
}

const maxPlainComponentBytes = 250

func isAncestor(ctx context.Context, worktree, ancestor, descendant string) (bool, error) {
	cmd := sysexec.Command(ctx, "git", "-C", worktree, "merge-base", "--is-ancestor", ancestor, descendant)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return false, nil
	default:
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w: %s", ancestor, descendant, err, strings.TrimSpace(string(out)))
	}
}

const (
	inboxRoot     = ".evolve/inbox/"
	consumedInbox = ".evolve/inbox/consumed/"
)

func consumptionPairs(states []pathState) []string {
	removed, consumed := map[string]bool{}, map[string]bool{}
	for _, s := range states {
		switch name := path.Base(s.rel); {
		case s.mode == "absent" && s.rel == inboxRoot+name:
			removed[name] = true
		case s.mode != "absent" && s.rel == consumedInbox+name:
			consumed[name] = true
		}
	}
	var pairs []string
	for name := range removed {
		if consumed[name] {
			pairs = append(pairs, name)
		}
	}
	return pairs
}

func shipConsumption(ctx context.Context, worktree, baseSHA string, states []pathState) (map[string]bool, error) {
	ships := map[string]bool{}
	for _, name := range consumptionPairs(states) {
		before, err := sysexec.Command(ctx, "git", "-C", worktree, "show", baseSHA+":"+inboxRoot+name).Output()
		if err != nil {
			return nil, fmt.Errorf("read inbox item %s at %s: %w", name, baseSHA, err)
		}
		after, _, err := readRegularWithin(worktree, consumedInbox+name)
		if err != nil {
			return nil, fmt.Errorf("read consumed item %s: %w", name, err)
		}
		ships[name] = keepsTheItem(before, []byte(after))
	}
	return ships, nil
}

func keepsTheItem(before, after []byte) bool {
	var item, consumed map[string]any
	if json.Unmarshal(before, &item) != nil || json.Unmarshal(after, &consumed) != nil {
		return false
	}
	for _, stamp := range []string{inboxbatch.ConsumedField, inboxbatch.ReleasedContinuationsField} {
		delete(item, stamp)
		delete(consumed, stamp)
	}
	return reflect.DeepEqual(item, consumed)
}

func withoutShipConsumption(states []pathState, ships map[string]bool) []pathState {
	lane := make([]pathState, 0, len(states))
	for _, s := range states {
		name := path.Base(s.rel)
		onePairHalf := s.rel == inboxRoot+name || s.rel == consumedInbox+name
		if onePairHalf && ships[name] {
			continue
		}
		lane = append(lane, s)
	}
	return lane
}
