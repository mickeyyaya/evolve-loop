package explanationdocs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

// ErrRebindIncomplete marks an error after the rebind started writing. It may leave a split that
// RecoverRebaseSplit recognises; any other error was raised before a write, so the caller may fall back.
var ErrRebindIncomplete = errors.New("identity-preserving rebind incomplete")

// RebindIdenticalRebase moves an approved Build contract to newBaseSHA without a new Build when the host
// proves the change it explains is byte-identical on that base (ADR-0105). It returns (false, nil) and
// writes nothing when the change is not provably identical, and (true, nil) once the result snapshot,
// the commit point, is written.
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

// proveIdenticalRebase is read-only. It returns the rebound snapshot only when every identity check holds.
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
	// One read of the changed paths backs both the proof and the digest the rebind binds.
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

// sameChange holds when the pending change on the new base has the exact paths, modes and bytes the
// approved Build sealed, the same material digest, and the Build report still declares the same handoff.
// The rebase itself needs no separate clean-replay check: a path the replay regenerated or resolved is
// one the peer delta also touched, which lineageHolds declines, and any other byte change fails the digest.
func sameChange(ctx context.Context, binding CycleBinding, prior *resultSnapshot, changed []string, states []pathState, newBaseSHA string) (bool, error) {
	if foldPathStates(diffDomain(prior.View.BaseSHA), states) != prior.View.DiffSHA256 {
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
	// Defence in depth: disjointness and the digest decline a document present at the new base first.
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

// authoredBase is the base the Builder's document names: the authored base a rebind recorded, else the
// view's own base.
func authoredBase(view *phaseio.ExplanationView) string {
	if view.AuthoredBaseSHA != "" {
		return view.AuthoredBaseSHA
	}
	return view.BaseSHA
}

// lineageHolds proves base descends from authored and that nothing landed between them touches the
// change: no peer path equals a changed path even case-folded, and no peer delta alters how git reads
// content (.gitattributes, .gitignore).
func lineageHolds(ctx context.Context, worktree, authored, base string, changed []string) (bool, error) {
	if authored == base {
		return false, nil
	}
	if ok, err := isAncestor(ctx, worktree, authored, base); err != nil || !ok {
		return false, err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "diff", "--no-renames", "--name-only", "-z", authored, base, "--")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git diff %s %s --name-only: %w: %s", authored, base, err, strings.TrimSpace(stderr.String()))
	}
	return !peerTouchesLane(splitNUL(out), changed), nil
}

// peerTouchesLane reports whether a peer path may name a lane file or changes how git reads content.
// Plain paths compare case-folded, for case-insensitive filesystems. Any other path counts as touching:
// some filesystem resolves it to a differently spelled file (APFS normalization, NTFS/exFAT/SMB trailing
// dots and spaces, 8.3 short names, stream suffixes), so equality of its bytes proves nothing.
func peerTouchesLane(peer, lane []string) bool {
	folded := make(map[string]bool, len(lane))
	for _, p := range lane {
		if !isPlainPath(p) {
			return true
		}
		folded[strings.ToLower(p)] = true
	}
	for _, p := range peer {
		lower := strings.ToLower(p)
		if name := path.Base(lower); !isPlainPath(p) || name == ".gitattributes" || name == ".gitignore" || folded[lower] {
			return true
		}
	}
	return false
}

// isPlainPath holds for an ASCII path with no ':', '\' or '~' whose components never end in a dot or a
// space (which also rejects "." and ".." components) and stay under a filesystem's name limit: one that
// every supported filesystem resolves to itself alone. Windows device names are out of scope here: they
// redirect I/O rather than alias another path, so they fail the digest instead.
func isPlainPath(p string) bool {
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

// maxPlainComponentBytes stays below the 255-byte name limit, so a truncating filesystem cannot fold two
// long names into one.
const maxPlainComponentBytes = 250

func isAncestor(ctx context.Context, worktree, ancestor, descendant string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "merge-base", "--is-ancestor", ancestor, descendant)
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
