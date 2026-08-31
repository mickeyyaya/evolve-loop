// Package explanationdocs verifies and binds Builder-authored engineering
// rationale documentation to one cycle's base-bound source diff.
package explanationdocs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const (
	manifestFilename      = "build-explanation.json"
	activationDirectory   = ".evolve/build-explanation-contracts"
	hostSchemaV1          = 1
	currentHostSchema     = hostSchemaV1
	artifactSchemaV1      = 1
	currentArtifactSchema = artifactSchemaV1
	contractV1            = 1
	maxArtifactBytes      = 2 << 20
	statusRequired        = "required"
	statusNA              = "not_applicable"
	materialReason        = "the base-bound Build diff contains material changes"
)

// CurrentContractVersion is the explanation-document contract activated for
// newly started cycles. Readers retain explicit dispatch for older versions.
const CurrentContractVersion = contractV1

var fullCommitSHA = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
var safeRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var cycleRecordPath = regexp.MustCompile(`^docs/explain/builds/cycle-[1-9][0-9]*-[a-z0-9][a-z0-9_-]{0,63}\.md$`)

type activation struct {
	SchemaVersion   int    `json:"schema_version"`
	ContractVersion int    `json:"contract_version"`
	Cycle           int    `json:"cycle"`
	RunID           string `json:"run_id"`
	Worktree        string `json:"worktree"`
	Workspace       string `json:"workspace"`
	BaseSHA         string `json:"base_sha"`
}

// CycleBinding carries host-owned facts. Callers populate it from orchestrator
// state, never from an agent-authored workspace artifact.
type CycleBinding struct {
	ProjectRoot     string
	Worktree        string
	Workspace       string
	BaseSHA         string
	Cycle           int
	RunID           string
	ContractVersion int
}

type reportDeclaration struct {
	Status   string
	Document string
	Reason   string
}

// Activate records a fresh cycle run's contract in host-owned runtime state.
// Reusing a non-durable local cycle number replaces an abandoned run marker;
// resume paths must use RequireActivation instead.
func Activate(binding CycleBinding) error {
	if binding.ProjectRoot == "" || binding.Cycle <= 0 || !safeRunID.MatchString(binding.RunID) || binding.Workspace == "" || !supportedContract(binding.ContractVersion) {
		return fmt.Errorf("project root, positive cycle, run ID, workspace, and a supported contract version are required")
	}
	if err := ensureActivationParent(binding.ProjectRoot); err != nil {
		return err
	}
	path := activationPath(binding.ProjectRoot, binding.Cycle)
	if _, err := os.Lstat(path); err == nil {
		existing, readErr := readActivation(binding.ProjectRoot, binding.Cycle)
		if readErr != nil {
			return readErr
		}
		if !supportedHostSchema(existing.SchemaVersion) || !supportedContract(existing.ContractVersion) || existing.Cycle != binding.Cycle {
			return fmt.Errorf("existing activation marker has invalid identity or version")
		}
		if existing.RunID == binding.RunID {
			_, active, resolveErr := resolveActivation(binding)
			if resolveErr != nil {
				return resolveErr
			}
			if !active {
				return fmt.Errorf("existing activation marker is inactive")
			}
			return nil
		}
		// Fresh-cycle activation is the host-authorized replacement boundary for
		// an abandoned run that reused a non-durable local cycle number.
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect activation marker: %w", err)
	}
	marker := activation{
		SchemaVersion:   currentHostSchema,
		ContractVersion: binding.ContractVersion,
		Cycle:           binding.Cycle,
		RunID:           binding.RunID,
		Workspace:       binding.Workspace,
	}
	if err := atomicwrite.JSON(path, marker); err != nil {
		return err
	}
	_, _, err := resolveActivation(binding)
	return err
}

// RequireActivation verifies an existing marker without creating or replacing
// it. Resume paths use this so corrupted checkpoint identity cannot rebind a
// cycle by calling the fresh-cycle activation transition.
func RequireActivation(binding CycleBinding) error {
	_, active, err := resolveActivation(binding)
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("build explanation contract is not active")
	}
	return nil
}

// CheckBuild validates the cycle-owned explanation and writes the derived
// workspace handoff on success. The host marker is the sole activation
// authority; legacy cycles without one remain exempt.
func CheckBuild(ctx context.Context, binding CycleBinding) []string {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil {
		return explanationFailure(err)
	}
	if !active {
		return nil
	}
	switch binding.ContractVersion {
	case contractV1:
		return checkBuildV1(ctx, binding)
	default:
		return explanationFailure(fmt.Errorf("unsupported explanation contract version %d", binding.ContractVersion))
	}
}

func checkBuildV1(ctx context.Context, binding CycleBinding) []string {
	if binding.Worktree == "" || binding.Workspace == "" || binding.BaseSHA == "" {
		return explanationFailure(errors.New("active contract requires worktree, workspace, and base SHA"))
	}
	paths, err := changedSince(ctx, binding.Worktree, binding.BaseSHA)
	if err != nil {
		return explanationFailure(fmt.Errorf("derive base-bound diff: %w", err))
	}
	diffSHA, err := diffSHA256(ctx, binding.Worktree, binding.BaseSHA)
	if err != nil {
		return explanationFailure(fmt.Errorf("hash base-bound diff: %w", err))
	}
	report, err := readBuildReport(binding.Workspace)
	if err != nil {
		return explanationFailure(err)
	}
	declaration, present, err := parseDeclaration(report)
	if err != nil {
		return explanationFailure(err)
	}
	if !present {
		return explanationFailure(errors.New("build-report.md is missing the required ## Explanation Documentation section"))
	}
	material := materialPaths(paths)
	if len(material) == 0 {
		if records := changedCycleRecords(paths); len(records) != 0 {
			return explanationFailure(fmt.Errorf("not-applicable Build changed immutable cycle explanation record(s): %s", strings.Join(records, ", ")))
		}
		return checkNotApplicable(binding, declaration, material, diffSHA)
	}
	if failures := foreignHistoryFailures(paths, cycleDocumentPath(binding.Cycle, binding.RunID)); len(failures) != 0 {
		return failures
	}
	return checkRequired(ctx, binding, declaration, paths, material, diffSHA)
}

// Load reads the derived explanation handoff. Audit and Ship call Verify before
// trusting its referenced document.
func Load(workspace string) (*phaseio.ExplanationView, error) {
	body, _, err := readRegularWithin(workspace, manifestFilename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestFilename, err)
	}
	var view phaseio.ExplanationView
	if err := json.Unmarshal([]byte(body), &view); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestFilename, err)
	}
	return &view, nil
}

// Verify re-checks the Build handoff against the host-owned base, current diff,
// permanent document, and sealed result snapshot. active is false only for a
// legacy cycle with no activated contract.
func Verify(ctx context.Context, binding CycleBinding) (*phaseio.ExplanationView, bool, error) {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil || !active {
		return nil, active, err
	}
	return verifyResolved(ctx, binding, binding.Worktree)
}

// VerifyLanded re-checks a shipped snapshot against ProjectRoot after the
// cycle worktree has been cleaned up. landedCommit must still be ProjectRoot's
// exact HEAD; callers establish it from the host ship binding, not agent data.
func VerifyLanded(ctx context.Context, binding CycleBinding, landedCommit string) (*phaseio.ExplanationView, bool, error) {
	binding, active, err := resolveBuildActivation(binding)
	if err != nil || !active {
		return nil, active, err
	}
	if err := requireLandedCommit(ctx, binding.ProjectRoot, landedCommit); err != nil {
		return nil, true, err
	}
	verificationRoot, err := os.MkdirTemp("", "evolve-verify-landed-")
	if err != nil {
		return nil, true, fmt.Errorf("create landed verification path: %w", err)
	}
	if err := os.Remove(verificationRoot); err != nil {
		return nil, true, fmt.Errorf("prepare landed verification path: %w", err)
	}
	addOut, err := exec.CommandContext(ctx, "git", "-C", binding.ProjectRoot, "worktree", "add", "--quiet", "--detach", verificationRoot, landedCommit).CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(verificationRoot)
		return nil, true, fmt.Errorf("create landed verification worktree: %w: %s", err, strings.TrimSpace(string(addOut)))
	}
	view, active, verifyErr := verifyResolved(ctx, binding, verificationRoot)
	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCleanup()
	removeOut, removeErr := exec.CommandContext(cleanupCtx, "git", "-C", binding.ProjectRoot, "worktree", "remove", "--force", verificationRoot).CombinedOutput()
	if removeErr != nil {
		return nil, true, fmt.Errorf("remove landed verification worktree: %w: %s", removeErr, strings.TrimSpace(string(removeOut)))
	}
	return view, active, verifyErr
}

func verifyResolved(ctx context.Context, binding CycleBinding, verificationRoot string) (*phaseio.ExplanationView, bool, error) {
	switch binding.ContractVersion {
	case contractV1:
		return verifyResolvedV1(ctx, binding, verificationRoot)
	default:
		return nil, true, fmt.Errorf("unsupported explanation contract version %d", binding.ContractVersion)
	}
}

func verifyResolvedV1(ctx context.Context, binding CycleBinding, verificationRoot string) (*phaseio.ExplanationView, bool, error) {
	view, err := Load(binding.Workspace)
	if err != nil {
		return nil, true, err
	}
	snapshot, err := LoadSnapshot(binding)
	if err != nil {
		return nil, true, err
	}
	if !SameView(view, snapshot) {
		return nil, true, fmt.Errorf("workspace %s does not match the host snapshot", manifestFilename)
	}
	if !supportedArtifactSchema(view.SchemaVersion) || !supportedContract(view.ContractVersion) || view.ContractVersion != binding.ContractVersion {
		return nil, true, fmt.Errorf("%s schema/contract version does not match the active host contract", manifestFilename)
	}
	if view.Cycle != binding.Cycle || view.BaseSHA != binding.BaseSHA {
		return nil, true, fmt.Errorf("%s cycle/base does not match the active host contract", manifestFilename)
	}
	paths, err := changedSince(ctx, verificationRoot, binding.BaseSHA)
	if err != nil {
		return nil, true, fmt.Errorf("derive base-bound diff: %w", err)
	}
	material := materialPaths(paths)
	if !samePaths(view.MaterialPaths, material) {
		return nil, true, fmt.Errorf("%s material_paths does not match the base-bound diff", manifestFilename)
	}
	diffSHA, err := diffSHA256(ctx, verificationRoot, binding.BaseSHA)
	if err != nil {
		return nil, true, fmt.Errorf("hash base-bound diff: %w", err)
	}
	if view.DiffSHA256 == "" || view.DiffSHA256 != diffSHA {
		return nil, true, fmt.Errorf("%s diff SHA256 does not match the base-bound Build content", manifestFilename)
	}
	report, err := readBuildReport(binding.Workspace)
	if err != nil {
		return nil, true, err
	}
	declaration, present, err := parseDeclaration(report)
	if err != nil || !present {
		if err == nil {
			err = errors.New("build-report.md is missing the required ## Explanation Documentation section")
		}
		return nil, true, err
	}
	if len(material) == 0 {
		if records := changedCycleRecords(paths); len(records) != 0 {
			return nil, true, fmt.Errorf("not-applicable Build changed immutable cycle explanation record(s): %s", strings.Join(records, ", "))
		}
		if view.Status != statusNA || view.DocumentPath != "" || view.DocumentSHA256 != "" || declaration.Status != "NOT_APPLICABLE" || declaration.Document != "" || declaration.Reason != view.Reason {
			return nil, true, fmt.Errorf("not-applicable explanation handoff does not match the Build declaration")
		}
		return view, true, nil
	}
	if view.Status != statusRequired || declaration.Status != "REQUIRED" || declaration.Document != view.DocumentPath {
		return nil, true, fmt.Errorf("required explanation handoff does not match the Build declaration")
	}
	verificationBinding := binding
	verificationBinding.Worktree = verificationRoot
	failures := validateRequired(ctx, verificationBinding, paths, material, view)
	if len(failures) != 0 {
		return nil, true, errors.New(strings.Join(failures, "; "))
	}
	return view, true, nil
}

func checkNotApplicable(binding CycleBinding, declaration reportDeclaration, material []string, diffSHA string) []string {
	if declaration.Status != "NOT_APPLICABLE" || strings.TrimSpace(declaration.Reason) == "" || declaration.Document != "" {
		return explanationFailure(errors.New("build-report.md must declare Status: NOT_APPLICABLE and a Reason, with no Document"))
	}
	if err := validateMetadataText("Reason", declaration.Reason, 1000); err != nil {
		return explanationFailure(err)
	}
	view := &phaseio.ExplanationView{
		SchemaVersion:   currentArtifactSchema,
		ContractVersion: binding.ContractVersion,
		Status:          statusNA,
		Cycle:           binding.Cycle,
		Reason:          strings.TrimSpace(declaration.Reason),
		BaseSHA:         binding.BaseSHA,
		DiffSHA256:      diffSHA,
		MaterialPaths:   material,
	}
	if err := writeManifest(binding, view); err != nil {
		return explanationFailure(err)
	}
	return nil
}

func checkRequired(ctx context.Context, binding CycleBinding, declaration reportDeclaration, changed, material []string, diffSHA string) []string {
	want := cycleDocumentPath(binding.Cycle, binding.RunID)
	var failures []string
	if declaration.Status != "REQUIRED" {
		failures = append(failures, "Explanation Documentation: Status must be REQUIRED for a material Build diff")
	}
	if declaration.Document != want || declaration.Reason != "" {
		failures = append(failures, "Explanation Documentation: Document must be the canonical cycle-owned path and Reason must be omitted when required")
	}
	view := &phaseio.ExplanationView{
		SchemaVersion:   currentArtifactSchema,
		ContractVersion: binding.ContractVersion,
		Status:          statusRequired,
		Cycle:           binding.Cycle,
		Reason:          materialReason,
		BaseSHA:         binding.BaseSHA,
		DocumentPath:    want,
		DiffSHA256:      diffSHA,
		MaterialPaths:   material,
	}
	failures = append(failures, validateRequired(ctx, binding, changed, material, view)...)
	if len(failures) != 0 {
		return failures
	}
	if err := writeManifest(binding, view); err != nil {
		return explanationFailure(err)
	}
	return nil
}

func validateRequired(ctx context.Context, binding CycleBinding, changed, material []string, view *phaseio.ExplanationView) []string {
	want := cycleDocumentPath(binding.Cycle, binding.RunID)
	var failures []string
	if view.DocumentPath != want {
		return []string{"Explanation Documentation: manifest Document path is not canonical for the cycle"}
	}
	if view.BaseSHA == "" {
		failures = append(failures, "Explanation Documentation: base_sha is required for downstream verification")
	}
	if !samePaths(view.MaterialPaths, material) {
		failures = append(failures, "Explanation Documentation: manifest material_paths does not match the base-bound diff")
	}
	if !contains(changed, want) {
		failures = append(failures, "Explanation Documentation: cycle document is not part of the Build diff")
	}
	if view.BaseSHA != "" {
		exists, err := existsAtBase(ctx, binding.Worktree, view.BaseSHA, want)
		if err != nil {
			failures = append(failures, "Explanation Documentation: cannot verify cycle document immutability: "+err.Error())
		} else if exists {
			failures = append(failures, "Explanation Documentation: cycle document existed at the cycle base; published Build explanations are immutable")
		}
	}
	failures = append(failures, immutableHistoryFailures(changed, want)...)
	body, sha, err := readRegularWithin(binding.Worktree, want)
	if err != nil {
		return append(failures, "Explanation Documentation: "+err.Error())
	}
	failures = append(failures, validateDocument(body, binding.Cycle, binding.BaseSHA, changed, material)...)
	if view.DocumentSHA256 != "" && view.DocumentSHA256 != sha {
		failures = append(failures, "Explanation Documentation: document SHA256 mismatch")
	}
	view.DocumentSHA256 = sha
	return failures
}

func validateDocument(body string, cycle int, baseSHA string, changed, material []string) []string {
	required := []struct {
		heading  string
		minBytes int
	}{
		{heading: "Summary", minBytes: 20},
		{heading: "Rationale", minBytes: 40},
		{heading: "Changed Areas", minBytes: 20},
		{heading: "Design Decisions", minBytes: 20},
		{heading: "Verification", minBytes: 20},
		{heading: "Compatibility", minBytes: 4},
		{heading: "Limitations", minBytes: 4},
	}
	var failures []string
	bindingBody, ok, err := reportdoc.Section(body, "Build Binding")
	if err != nil || !ok {
		if err == nil {
			err = errors.New("missing ## Build Binding")
		}
		failures = append(failures, "Explanation Documentation: "+err.Error())
	} else {
		fields, fieldErr := reportdoc.Fields(bindingBody, "Cycle", "Base SHA")
		if fieldErr != nil {
			failures = append(failures, "Explanation Documentation: "+fieldErr.Error())
		} else if fields["cycle"] != strconv.Itoa(cycle) || fields["base sha"] != baseSHA {
			failures = append(failures, "Explanation Documentation: Build Binding must contain the exact Cycle and Base SHA")
		}
	}
	sections := make(map[string]string, len(required))
	for _, requirement := range required {
		section, present, sectionErr := reportdoc.Section(body, requirement.heading)
		switch {
		case sectionErr != nil:
			failures = append(failures, "Explanation Documentation: "+sectionErr.Error())
		case !present:
			failures = append(failures, "Explanation Documentation: missing ## "+requirement.heading)
		case len(strings.TrimSpace(section)) < requirement.minBytes:
			failures = append(failures, fmt.Sprintf("Explanation Documentation: ## %s must contain substantive content", requirement.heading))
		default:
			sections[requirement.heading] = section
		}
	}
	entries, entryFailures := changedAreaEntries(sections["Changed Areas"])
	failures = append(failures, entryFailures...)
	changedSet := stringSet(changed)
	for _, path := range material {
		if entries[path] == "" {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: Changed Areas does not explain material path %s", path))
		}
	}
	extraPaths := make([]string, 0, len(entries))
	for path := range entries {
		extraPaths = append(extraPaths, path)
	}
	sort.Strings(extraPaths)
	for _, path := range extraPaths {
		if !changedSet[path] {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: cited path %s is not in the Build diff", path))
		}
	}
	return failures
}

func changedAreaEntries(body string) (map[string]string, []string) {
	entries := map[string]string{}
	var failures []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		rest := strings.TrimPrefix(line, "- `")
		end := strings.Index(rest, "`")
		if end < 0 {
			continue
		}
		path := normalize(rest[:end])
		explanation := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(rest[end+1:]), "—-:"))
		if !validRelative(path) {
			failures = append(failures, "Explanation Documentation: Changed Areas contains an invalid repo-relative path")
			continue
		}
		if len(explanation) < 10 {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: Changed Areas path %s needs a what/why explanation", path))
			continue
		}
		entries[path] = explanation
	}
	return entries, failures
}

func readBuildReport(workspace string) (string, error) {
	for _, rel := range []string{"build-report.md", "deliverables/build-report.md"} {
		body, _, err := readRegularWithin(workspace, rel)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", errors.New("build-report.md is missing")
}

func parseDeclaration(report string) (reportDeclaration, bool, error) {
	body, present, err := reportdoc.Section(report, "Explanation Documentation")
	if err != nil || !present {
		return reportDeclaration{}, present, err
	}
	fields, err := reportdoc.Fields(body, "Status", "Document", "Reason")
	if err != nil {
		return reportDeclaration{}, true, fmt.Errorf("build-report.md Explanation Documentation: %w", err)
	}
	return reportDeclaration{
		Status:   strings.ToUpper(fields["status"]),
		Document: fields["document"],
		Reason:   fields["reason"],
	}, true, nil
}

func materialPaths(paths []string) []string {
	var out []string
	for _, raw := range paths {
		path := normalize(raw)
		lower := strings.ToLower(path)
		if path == "" || strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "knowledge-base/") ||
			strings.HasPrefix(path, ".evolve/evals/") || strings.HasPrefix(path, "go/acs/") ||
			strings.HasPrefix(path, "acs/") || pathInDirectory(path, "testdata") ||
			isUnambiguousTestFile(lower) {
			continue
		}
		out = append(out, path)
	}
	return uniqueSorted(out)
}

func isUnambiguousTestFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		(strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) {
		return true
	}
	for _, suffix := range []string{
		".spec.js", ".spec.jsx", ".spec.ts", ".spec.tsx", ".spec.mjs", ".spec.cjs",
		".test.js", ".test.jsx", ".test.ts", ".test.tsx", ".test.mjs", ".test.cjs",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func pathInDirectory(path, directory string) bool {
	return strings.HasPrefix(path, directory+"/") || strings.Contains(path, "/"+directory+"/")
}

func cycleDocumentPath(cycle int, runID string) string {
	path, _ := DocumentPath(cycle, runID)
	return path
}

// DocumentPath returns the collision-resistant permanent path bound to one
// cycle run. Run identity prevents a fresh clone's local cycle counter from
// colliding with a document already published by another installation.
func DocumentPath(cycle int, runID string) (string, error) {
	if cycle <= 0 || !safeRunID.MatchString(runID) {
		return "", fmt.Errorf("positive cycle and safe run ID are required for the explanation document path")
	}
	return fmt.Sprintf("docs/explain/builds/cycle-%d-%s.md", cycle, strings.ToLower(runID)), nil
}

func supportedContract(version int) bool {
	switch version {
	case contractV1:
		return true
	default:
		return false
	}
}

func supportedHostSchema(version int) bool {
	switch version {
	case hostSchemaV1:
		return true
	default:
		return false
	}
}

func supportedArtifactSchema(version int) bool {
	switch version {
	case artifactSchemaV1:
		return true
	default:
		return false
	}
}

func explanationFailure(err error) []string {
	return []string{"Explanation Documentation: " + err.Error()}
}

func validateMetadataText(label, value string, max int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds %d bytes", label, max)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains control characters", label)
		}
	}
	return nil
}

func activationPath(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, filepath.FromSlash(activationDirectory), fmt.Sprintf("cycle-%d.json", cycle))
}

func ensureActivationParent(projectRoot string) error {
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if info, err := os.Lstat(evolveDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect .evolve directory: %w", err)
		}
		if err := os.MkdirAll(evolveDir, 0o755); err != nil {
			return fmt.Errorf("create .evolve directory: %w", err)
		}
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf(".evolve must be a real directory")
	}
	parent := filepath.Join(projectRoot, filepath.FromSlash(activationDirectory))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create activation directory: %w", err)
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("activation path must be a real directory")
	}
	return nil
}

func resolveActivation(binding CycleBinding) (CycleBinding, bool, error) {
	if binding.ContractVersion != 0 && (binding.ProjectRoot == "" || binding.Cycle <= 0 || binding.RunID == "") {
		return binding, true, fmt.Errorf("active contract requires project root, positive cycle, and run ID")
	}
	if binding.ProjectRoot == "" || binding.Cycle <= 0 {
		return binding, false, nil
	}
	body, _, err := readRegularWithin(binding.ProjectRoot, activationRel(binding.Cycle))
	if errors.Is(err, fs.ErrNotExist) {
		if binding.ContractVersion != 0 {
			return binding, true, fmt.Errorf("activation marker is missing for contract version %d", binding.ContractVersion)
		}
		return binding, false, nil
	}
	if err != nil {
		return binding, false, fmt.Errorf("read activation marker: %w", err)
	}
	var marker activation
	if err := json.Unmarshal([]byte(body), &marker); err != nil {
		return binding, true, fmt.Errorf("parse activation marker: %w", err)
	}
	if !supportedHostSchema(marker.SchemaVersion) || !supportedContract(marker.ContractVersion) {
		return binding, true, fmt.Errorf("activation marker has unsupported schema/contract version")
	}
	if binding.ContractVersion != 0 && marker.ContractVersion != binding.ContractVersion {
		return binding, true, fmt.Errorf("activation marker does not match host contract version %d", binding.ContractVersion)
	}
	if marker.Cycle != binding.Cycle || binding.RunID == "" || marker.RunID != binding.RunID {
		return binding, true, fmt.Errorf("activation marker cycle/run binding does not match host state")
	}
	strict := binding.ContractVersion != 0
	if (strict || binding.Workspace != "") && binding.Workspace != marker.Workspace ||
		marker.Worktree != "" && binding.Worktree != "" && binding.Worktree != marker.Worktree ||
		marker.BaseSHA != "" && binding.BaseSHA != "" && binding.BaseSHA != marker.BaseSHA {
		return binding, true, fmt.Errorf("activation marker worktree/workspace/base binding does not match host state")
	}
	binding.Workspace = marker.Workspace
	if marker.Worktree != "" {
		binding.Worktree = marker.Worktree
	}
	if marker.BaseSHA != "" {
		binding.BaseSHA = marker.BaseSHA
	}
	binding.ContractVersion = marker.ContractVersion
	return binding, true, nil
}

func samePaths(a, b []string) bool {
	return strings.Join(uniqueSorted(a), "\x00") == strings.Join(uniqueSorted(b), "\x00")
}
