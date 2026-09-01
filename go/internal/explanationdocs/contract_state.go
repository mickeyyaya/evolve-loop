package explanationdocs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

type resultSnapshot struct {
	SchemaVersion   int                     `json:"schema_version"`
	ContractVersion int                     `json:"contract_version"`
	Cycle           int                     `json:"cycle"`
	RunID           string                  `json:"run_id"`
	MaterialSHA256  string                  `json:"material_sha256"`
	View            phaseio.ExplanationView `json:"view"`
}

// SealBuild binds the final post-planning worktree and base SHA. It is a
// one-way, idempotent transition so continuation adoption may replace the
// provisional worktree before Build without weakening later provenance.
func SealBuild(binding CycleBinding) error {
	if binding.Worktree == "" || binding.BaseSHA == "" {
		return fmt.Errorf("worktree and base SHA are required to seal Build context")
	}
	identity := binding
	identity.Worktree = ""
	identity.BaseSHA = ""
	resolved, active, err := resolveActivation(identity)
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("cannot seal Build context without an active host contract")
	}
	marker, err := readActivation(resolved.ProjectRoot, resolved.Cycle)
	if err != nil {
		return err
	}
	if marker.Worktree != "" || marker.BaseSHA != "" {
		if marker.Worktree == binding.Worktree && marker.BaseSHA == binding.BaseSHA {
			return nil
		}
		return fmt.Errorf("build context already sealed to a different worktree or base SHA")
	}
	marker.Worktree = binding.Worktree
	marker.BaseSHA = binding.BaseSHA
	if err := ensureActivationParent(binding.ProjectRoot); err != nil {
		return err
	}
	return atomicwrite.JSON(activationPath(binding.ProjectRoot, binding.Cycle), marker)
}

// SealResult preserves the fully reviewed Builder handoff outside the mutable
// phase workspace. A legal same-cycle correction replaces the prior result
// atomically; downstream phases always receive the latest approved generation.
func SealResult(ctx context.Context, binding CycleBinding) error {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	view, err := revalidateResult(ctx, binding)
	if err != nil {
		return err
	}
	_, err = sealResultViewExpected(ctx, binding, view, "")
	return err
}

// RefreshResult advances the host snapshot after a legitimate post-Build
// worktree writer changes only non-material files (for example generated
// tests). It refuses to bless changed material scope or Builder-owned
// documentation; requiresBuild tells the orchestrator to route ownership back
// through Build.
func RefreshResult(ctx context.Context, binding CycleBinding) (requiresBuild bool, err error) {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil || !active {
		return false, err
	}
	prior, err := readResultSnapshot(binding)
	if err != nil {
		return false, err
	}
	paths, err := changedSince(ctx, binding.Worktree, binding.BaseSHA)
	if err != nil {
		return false, fmt.Errorf("derive post-Build diff: %w", err)
	}
	if !samePaths(prior.View.MaterialPaths, materialPaths(paths)) {
		return true, nil
	}
	materialSHA, err := materialSHA256(ctx, binding.Worktree, prior.View.MaterialPaths)
	if err != nil {
		return false, fmt.Errorf("hash post-Build material paths: %w", err)
	}
	if materialSHA != prior.MaterialSHA256 {
		return true, nil
	}
	current, err := revalidateResult(ctx, binding)
	if err != nil {
		return false, err
	}
	if prior.View.Status != current.Status || prior.View.Reason != current.Reason ||
		prior.View.DocumentPath != current.DocumentPath || prior.View.DocumentSHA256 != current.DocumentSHA256 ||
		!reflect.DeepEqual(prior.View.MaterialPaths, current.MaterialPaths) {
		return true, nil
	}
	return sealResultViewExpected(ctx, binding, current, prior.MaterialSHA256)
}

func revalidateResult(ctx context.Context, binding CycleBinding) (*phaseio.ExplanationView, error) {
	// The optional reviewer runs between the first Build check and this host
	// seal. Re-derive the complete view here so neither that reviewer nor a
	// lingering phase process can replace validated workspace fields in the
	// intervening window.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if failures := CheckBuild(ctx, binding); len(failures) != 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("revalidate builder explanation handoff before sealing: %s", strings.Join(failures, "; "))
	}
	view, err := Load(binding.Workspace)
	if err != nil {
		return nil, err
	}
	if !supportedArtifactSchema(view.SchemaVersion) || view.ContractVersion != binding.ContractVersion || view.Cycle != binding.Cycle || view.BaseSHA != binding.BaseSHA || view.DiffSHA256 == "" {
		return nil, fmt.Errorf("builder explanation handoff does not match the sealed host contract")
	}
	return view, nil
}

func sealResultViewExpected(ctx context.Context, binding CycleBinding, view *phaseio.ExplanationView, expectedMaterialSHA string) (requiresBuild bool, err error) {
	materialSHA, err := materialSHA256(ctx, binding.Worktree, view.MaterialPaths)
	if err != nil {
		return false, fmt.Errorf("hash Build material paths: %w", err)
	}
	if expectedMaterialSHA != "" && materialSHA != expectedMaterialSHA {
		return true, nil
	}
	snapshot := resultSnapshot{
		SchemaVersion:   currentHostSchema,
		ContractVersion: binding.ContractVersion,
		Cycle:           binding.Cycle,
		RunID:           binding.RunID,
		MaterialSHA256:  materialSHA,
		View:            *view,
	}
	if err := ensureActivationParent(binding.ProjectRoot); err != nil {
		return false, err
	}
	return false, atomicwrite.JSON(resultSnapshotPath(binding.ProjectRoot, binding.Cycle), snapshot)
}

// RebaseBuild moves an approved Build contract to a new, verified git base and
// invalidates its result snapshot. The orchestrator must dispatch Build again
// so only Builder-authored documentation can bind and explain the rebased diff.
func RebaseBuild(ctx context.Context, binding CycleBinding, newBaseSHA string) error {
	return RebaseBuildAndPersist(ctx, binding, newBaseSHA, nil)
}

// RebaseBuildAndPersist writes the host marker before persisting its matching
// checkpoint. The marker is the write-ahead authority: an ambiguous persistence
// error must not roll it back because the checkpoint write may already have
// committed before a secondary mirror failed.
func RebaseBuildAndPersist(ctx context.Context, binding CycleBinding, newBaseSHA string, persist func() error) error {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("cannot rebase Build context without an active host contract")
	}
	if err := validateBaseCommit(ctx, binding.Worktree, newBaseSHA); err != nil {
		return err
	}
	if newBaseSHA == binding.BaseSHA {
		return nil
	}
	if _, err := LoadSnapshot(binding); err != nil {
		return fmt.Errorf("rebase requires an approved Build snapshot: %w", err)
	}
	marker, err := readActivation(binding.ProjectRoot, binding.Cycle)
	if err != nil {
		return err
	}
	marker.BaseSHA = newBaseSHA
	if err := atomicwrite.JSON(activationPath(binding.ProjectRoot, binding.Cycle), marker); err != nil {
		return err
	}
	if persist != nil {
		if persistErr := persist(); persistErr != nil {
			return fmt.Errorf("persist rebased Build checkpoint: %w", persistErr)
		}
	}
	return nil
}

// RecoverRebaseSplit identifies the single crash window where the host marker
// reached a verified new base but the cycle checkpoint still names the old
// base. The approved old-base snapshot is the journal witness; callers must
// persist the returned base and restart at Build before continuing.
func RecoverRebaseSplit(ctx context.Context, binding CycleBinding) (newBaseSHA string, recoverable bool, err error) {
	if binding.ContractVersion == 0 || binding.ProjectRoot == "" || binding.Cycle <= 0 || binding.Worktree == "" || binding.BaseSHA == "" {
		return "", false, nil
	}
	marker, err := readActivation(binding.ProjectRoot, binding.Cycle)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if marker.ContractVersion != binding.ContractVersion || marker.Cycle != binding.Cycle || marker.RunID != binding.RunID ||
		filepath.Clean(marker.Workspace) != filepath.Clean(binding.Workspace) || filepath.Clean(marker.Worktree) != filepath.Clean(binding.Worktree) {
		return "", false, nil
	}
	if err := validateBaseCommit(ctx, binding.Worktree, marker.BaseSHA); err != nil {
		return "", false, err
	}
	snapshot, err := readResultSnapshotIdentity(binding)
	if err != nil {
		return "", false, nil
	}
	if marker.BaseSHA == binding.BaseSHA {
		if snapshot.View.BaseSHA == marker.BaseSHA {
			return "", false, nil
		}
		if err := validateBaseCommit(ctx, binding.Worktree, snapshot.View.BaseSHA); err != nil {
			return "", false, nil
		}
		return marker.BaseSHA, true, nil
	}
	if snapshot.View.BaseSHA != binding.BaseSHA {
		return "", false, nil
	}
	return marker.BaseSHA, true, nil
}

// LoadSnapshot returns the host-owned Builder handoff used for downstream
// PhaseRequests. It never falls back to the mutable workspace manifest.
func LoadSnapshot(binding CycleBinding) (*phaseio.ExplanationView, error) {
	snapshot, err := loadResultSnapshot(binding)
	if err != nil {
		return nil, err
	}
	view := snapshot.View
	return &view, nil
}

func loadResultSnapshot(binding CycleBinding) (*resultSnapshot, error) {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, fs.ErrNotExist
	}
	return readResultSnapshot(binding)
}

func readResultSnapshot(binding CycleBinding) (*resultSnapshot, error) {
	snapshot, err := readResultSnapshotIdentity(binding)
	if err != nil {
		return nil, err
	}
	if snapshot.View.BaseSHA != binding.BaseSHA {
		return nil, fmt.Errorf("host explanation snapshot does not match the active cycle/run contract")
	}
	return snapshot, nil
}

func readResultSnapshotIdentity(binding CycleBinding) (*resultSnapshot, error) {
	body, _, err := readRegularWithin(binding.ProjectRoot, resultSnapshotRel(binding.Cycle))
	if err != nil {
		return nil, fmt.Errorf("read host explanation snapshot: %w", err)
	}
	var snapshot resultSnapshot
	if err := json.Unmarshal([]byte(body), &snapshot); err != nil {
		return nil, fmt.Errorf("parse host explanation snapshot: %w", err)
	}
	if !supportedHostSchema(snapshot.SchemaVersion) || snapshot.ContractVersion != binding.ContractVersion || snapshot.Cycle != binding.Cycle || snapshot.RunID != binding.RunID || snapshot.MaterialSHA256 == "" ||
		!supportedArtifactSchema(snapshot.View.SchemaVersion) || snapshot.View.ContractVersion != binding.ContractVersion || snapshot.View.Cycle != binding.Cycle || snapshot.View.BaseSHA == "" || snapshot.View.DiffSHA256 == "" {
		return nil, fmt.Errorf("host explanation snapshot does not match the active cycle/run contract")
	}
	return &snapshot, nil
}

// ActivationForCycle loads exactly one host-owned marker and reconciles its
// workspace. Callers obtain cycle from a host checkpoint path or native ship
// input, so unrelated stale/corrupt markers cannot block the lookup and lookup
// cost does not grow with cycle history.
func ActivationForCycle(projectRoot string, cycle int, workspace string) (CycleBinding, bool, error) {
	if projectRoot == "" || cycle <= 0 || workspace == "" {
		return CycleBinding{}, false, nil
	}
	path := activationPath(projectRoot, cycle)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return CycleBinding{}, false, nil
	}
	if err != nil {
		return CycleBinding{}, false, fmt.Errorf("inspect activation marker: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return CycleBinding{}, false, fmt.Errorf("activation marker must be a regular file")
	}
	marker, err := readActivation(projectRoot, cycle)
	if err != nil {
		return CycleBinding{}, false, err
	}
	if !supportedHostSchema(marker.SchemaVersion) || !supportedContract(marker.ContractVersion) || marker.Cycle != cycle || !safeRunID.MatchString(marker.RunID) || marker.Workspace == "" {
		return CycleBinding{}, false, fmt.Errorf("activation marker cycle-%d has invalid identity or version", cycle)
	}
	if filepath.Clean(marker.Workspace) != filepath.Clean(workspace) {
		return CycleBinding{}, false, nil
	}
	return CycleBinding{
		ProjectRoot: projectRoot, Worktree: marker.Worktree, Workspace: marker.Workspace,
		BaseSHA: marker.BaseSHA, Cycle: marker.Cycle, RunID: marker.RunID,
		ContractVersion: marker.ContractVersion,
	}, true, nil
}

func resolveBuildActivation(binding CycleBinding) (CycleBinding, bool, error) {
	resolved, active, err := resolveActivation(binding)
	if err != nil || !active {
		return resolved, active, err
	}
	marker, err := readActivation(resolved.ProjectRoot, resolved.Cycle)
	if err != nil {
		return resolved, true, err
	}
	if marker.Worktree == "" || marker.BaseSHA == "" {
		return resolved, true, fmt.Errorf("build context is not sealed")
	}
	if binding.Worktree != "" && binding.Worktree != marker.Worktree || binding.BaseSHA != "" && binding.BaseSHA != marker.BaseSHA {
		return resolved, true, fmt.Errorf("sealed Build context does not match host state")
	}
	resolved.Worktree = marker.Worktree
	resolved.BaseSHA = marker.BaseSHA
	return resolved, true, nil
}

func readActivation(projectRoot string, cycle int) (activation, error) {
	body, _, err := readRegularWithin(projectRoot, activationRel(cycle))
	if err != nil {
		return activation{}, fmt.Errorf("read activation marker: %w", err)
	}
	var marker activation
	if err := json.Unmarshal([]byte(body), &marker); err != nil {
		return activation{}, fmt.Errorf("parse activation marker: %w", err)
	}
	return marker, nil
}

func activationRel(cycle int) string {
	return filepath.ToSlash(filepath.Join(activationDirectory, fmt.Sprintf("cycle-%d.json", cycle)))
}

func resultSnapshotRel(cycle int) string {
	return filepath.ToSlash(filepath.Join(activationDirectory, fmt.Sprintf("cycle-%d-result.json", cycle)))
}

func resultSnapshotPath(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, filepath.FromSlash(resultSnapshotRel(cycle)))
}

// SameView compares the complete typed handoff. Audit and Ship use this exact
// comparison so neither can accept a weaker projection.
func SameView(a, b *phaseio.ExplanationView) bool {
	return a != nil && b != nil && reflect.DeepEqual(*a, *b)
}
