package explanationdocs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

const (
	maxDiffDigestBytes = int64(64 << 20)
	maxDiffDigestFiles = 10_000
)

var errDiffDigestLimit = errors.New("base-bound diff exceeds the 64 MiB digest limit")

func changedSince(ctx context.Context, worktree, baseSHA string) ([]string, error) {
	if worktree == "" || baseSHA == "" {
		return nil, fmt.Errorf("worktree and base SHA are required")
	}
	if err := validateBaseCommit(ctx, worktree, baseSHA); err != nil {
		return nil, err
	}
	diff, err := exec.CommandContext(ctx, "git", "-C", worktree, "diff", "--no-renames", "--name-only", "-z", baseSHA, "--").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff %s --name-only: %w: %s", baseSHA, err, strings.TrimSpace(string(diff)))
	}
	untracked, err := exec.CommandContext(ctx, "git", "-C", worktree, "ls-files", "--others", "--exclude-standard", "-z").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others: %w: %s", err, strings.TrimSpace(string(untracked)))
	}
	paths := append(splitNUL(diff), splitNUL(untracked)...)
	return uniqueSorted(paths), nil
}

func diffSHA256(ctx context.Context, worktree, baseSHA string) (string, error) {
	paths, err := changedSince(ctx, worktree, baseSHA)
	if err != nil {
		return "", err
	}
	return pathStateSHA256(ctx, worktree, fmt.Sprintf("evolve-build-diff-v3\nbase:%d:%s", len(baseSHA), baseSHA), paths)
}

func materialSHA256(ctx context.Context, worktree string, paths []string) (string, error) {
	return pathStateSHA256(ctx, worktree, "evolve-build-material-v1", paths)
}

func pathStateSHA256(ctx context.Context, worktree, domain string, paths []string) (string, error) {
	paths = uniqueSorted(paths)
	if len(paths) > maxDiffDigestFiles {
		return "", fmt.Errorf("base-bound diff exceeds the %d-file digest limit", maxDiffDigestFiles)
	}

	h := sha256.New()
	fmt.Fprintf(h, "%s\npath-count:%d\n", domain, len(paths))
	var totalBytes int64
	for _, rel := range paths {
		if !validRelative(rel) {
			return "", fmt.Errorf("invalid changed path %q", rel)
		}
		remaining := maxDiffDigestBytes - totalBytes
		mode, contentSHA, contentBytes, err := currentDiffState(ctx, worktree, rel, remaining)
		if err != nil {
			return "", fmt.Errorf("hash changed path %s: %w", rel, err)
		}
		totalBytes += contentBytes
		fmt.Fprintf(h, "path:%d:", len(rel))
		_, _ = io.WriteString(h, rel)
		fmt.Fprintf(h, "\nmode:%s\ncontent-bytes:%d\ncontent-sha256:%s\n", mode, contentBytes, contentSHA)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func splitNUL(body []byte) []string {
	raw := strings.TrimSuffix(string(body), "\x00")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\x00")
}

func currentDiffState(ctx context.Context, worktree, rel string, remaining int64) (string, string, int64, error) {
	if remaining < 0 {
		return "", "", 0, errDiffDigestLimit
	}
	root, err := filepath.Abs(worktree)
	if err != nil {
		return "", "", 0, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", 0, fmt.Errorf("resolve worktree: %w", err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if _, repeatErr := os.Lstat(path); !os.IsNotExist(repeatErr) {
			return "", "", 0, fmt.Errorf("deleted path changed while hashing")
		}
		return "absent", "-", 0, nil
	}
	if err != nil {
		return "", "", 0, err
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", "", 0, fmt.Errorf("resolve changed-path parent: %w", err)
	}
	if !within(realRoot, realParent) {
		return "", "", 0, fmt.Errorf("changed path escapes the worktree")
	}
	switch {
	case info.Mode().IsRegular():
		sha, size, err := untrackedContentSHA256(path, info, remaining)
		mode := "100644"
		if info.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		return mode, sha, size, err
	case info.Mode()&os.ModeSymlink != 0:
		sha, size, err := untrackedContentSHA256(path, info, remaining)
		return "120000", sha, size, err
	case info.IsDir():
		out, err := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--verify", "HEAD").CombinedOutput()
		if err != nil {
			return "", "", 0, fmt.Errorf("changed directory is not a readable gitlink: %w: %s", err, strings.TrimSpace(string(out)))
		}
		commit := strings.TrimSpace(string(out))
		if !fullCommitSHA.MatchString(commit) || int64(len(commit)) > remaining {
			return "", "", 0, fmt.Errorf("gitlink commit is invalid or exceeds the digest limit")
		}
		sum := sha256.Sum256([]byte(commit))
		return "160000", hex.EncodeToString(sum[:]), int64(len(commit)), nil
	default:
		return "", "", 0, fmt.Errorf("unsupported file type")
	}
}

func untrackedContentSHA256(path string, expected os.FileInfo, remaining int64) (string, int64, error) {
	if remaining < 0 {
		return "", 0, errDiffDigestLimit
	}
	h := sha256.New()
	switch {
	case expected.Mode().IsRegular():
		if expected.Size() > remaining {
			return "", 0, errDiffDigestLimit
		}
		file, err := openRegularNoFollow(path)
		if err != nil {
			return "", 0, err
		}
		defer func() { _ = file.Close() }()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) {
			return "", 0, fmt.Errorf("file changed type or identity while opening")
		}
		if opened.Size() > remaining {
			return "", 0, errDiffDigestLimit
		}
		n, err := io.CopyN(h, file, opened.Size())
		if err != nil {
			return "", 0, err
		}
		var extra [1]byte
		if count, readErr := file.Read(extra[:]); count != 0 || readErr != io.EOF {
			return "", 0, fmt.Errorf("file changed size while hashing")
		}
		after, err := file.Stat()
		if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
			return "", 0, fmt.Errorf("file changed while hashing")
		}
		return hex.EncodeToString(h.Sum(nil)), n, nil
	case expected.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return "", 0, err
		}
		if int64(len(target)) > remaining {
			return "", 0, errDiffDigestLimit
		}
		after, err := os.Lstat(path)
		if err != nil || after.Mode()&os.ModeSymlink == 0 || !os.SameFile(expected, after) {
			return "", 0, fmt.Errorf("symlink changed while hashing")
		}
		_, _ = io.WriteString(h, target)
		return hex.EncodeToString(h.Sum(nil)), int64(len(target)), nil
	default:
		return "", 0, fmt.Errorf("unsupported file type")
	}
}

func validateBaseCommit(ctx context.Context, worktree, baseSHA string) error {
	if !fullCommitSHA.MatchString(baseSHA) {
		return fmt.Errorf("base revision must be a full commit SHA")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", worktree, "cat-file", "-e", baseSHA+"^{commit}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("base SHA is not a commit: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func requireLandedCommit(ctx context.Context, projectRoot, landedCommit string) error {
	if !fullCommitSHA.MatchString(landedCommit) {
		return fmt.Errorf("landed commit must be a full commit SHA")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", projectRoot, "rev-parse", "--verify", "HEAD").CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolve landed HEAD: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) != landedCommit {
		return fmt.Errorf("landed commit does not match ProjectRoot HEAD")
	}
	return nil
}

// ArchiveUnpublishedContinuationRecords moves failed-build drafts out of the
// immutable canonical cycle namespace while preserving them under docs/.
func ArchiveUnpublishedContinuationRecords(ctx context.Context, worktree, baseSHA string) ([]string, error) {
	paths, err := changedSince(ctx, worktree, baseSHA)
	if err != nil {
		return nil, err
	}
	archiveDir := filepath.ToSlash(filepath.Join("docs", "private", "research", "archived-"+time.Now().UTC().Format("2006-01-02"), "unshipped-build-explanations"))
	archiveAbs, err := ensureRealSubdirectories(worktree, archiveDir)
	if err != nil {
		return nil, err
	}
	var archived []string
	var stagePaths []string
	for _, rel := range changedCycleRecords(paths) {
		exists, err := existsAtBase(ctx, worktree, baseSHA, rel)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		path := filepath.Join(worktree, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect unpublished explanation %s: %w", rel, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unpublished explanation %s must be a regular non-symlink file", rel)
		}
		if _, _, err := readRegularWithin(worktree, rel); err != nil {
			return nil, fmt.Errorf("verify unpublished explanation %s: %w", rel, err)
		}
		archiveRel := filepath.ToSlash(filepath.Join(archiveDir, filepath.Base(rel)))
		archivePath := filepath.Join(archiveAbs, filepath.Base(rel))
		if _, err := os.Lstat(archivePath); err == nil {
			return nil, fmt.Errorf("archive destination %s already exists", archiveRel)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect archive destination %s: %w", archiveRel, err)
		}
		if err := os.Rename(path, archivePath); err != nil {
			return nil, fmt.Errorf("archive unpublished explanation %s: %w", rel, err)
		}
		archived = append(archived, archiveRel)
		stagePaths = append(stagePaths, rel, archiveRel)
	}
	if len(archived) == 0 {
		return nil, nil
	}
	args := append([]string{"-C", worktree, "add", "-A", "--"}, stagePaths...)
	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("stage unpublished explanation archive: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return archived, nil
}

func ensureRealSubdirectories(root, rel string) (string, error) {
	if !validRelative(rel) {
		return "", fmt.Errorf("invalid archive directory %q", rel)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := requireRealDirectory(root, "worktree"); err != nil {
		return "", err
	}
	current := root
	for _, component := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("create archive directory: %w", err)
		}
		if err := requireRealDirectory(current, "archive directory"); err != nil {
			return "", err
		}
	}
	return current, nil
}

func immutableHistoryFailures(changed []string, current string) []string {
	failures := foreignHistoryFailures(changed, current)
	if !contains(changed, current) {
		failures = append(failures, "Explanation Documentation: current cycle change record must be newly added relative to the sealed base")
	}
	return failures
}

func foreignHistoryFailures(changed []string, current string) []string {
	var failures []string
	for _, path := range changedCycleRecords(changed) {
		if path != current {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: historical change record %s is a foreign cycle change record; published cycle records are immutable", path))
		}
	}
	return failures
}

func changedCycleRecords(changed []string) []string {
	var records []string
	for _, path := range uniqueSorted(changed) {
		if isCycleChangeRecord(path) {
			records = append(records, path)
		}
	}
	return records
}

func isCycleChangeRecord(path string) bool {
	return cycleRecordPath.MatchString(path)
}

func existsAtBase(ctx context.Context, worktree, baseSHA, path string) (bool, error) {
	if !validRelative(path) {
		return false, fmt.Errorf("invalid base path %q", path)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", worktree, "ls-tree", "-z", "--name-only", baseSHA, "--", path).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("inspect %s at base: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSuffix(string(out), "\x00") == path, nil
}

func readRegularWithin(root, rel string) (string, string, error) {
	if !validRelative(rel) {
		return "", "", fmt.Errorf("invalid relative path %q", rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	abs := filepath.Join(absRoot, filepath.FromSlash(rel))
	info, err := os.Lstat(abs)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("%s must be a regular non-symlink file", rel)
	}
	if info.Size() > maxArtifactBytes {
		return "", "", fmt.Errorf("%s exceeds %d bytes", rel, maxArtifactBytes)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve worktree: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil || !within(realRoot, real) {
		return "", "", fmt.Errorf("%s escapes the worktree", rel)
	}
	file, err := openRegularNoFollow(real)
	if err != nil {
		return "", "", fmt.Errorf("open %s without following links: %w", rel, err)
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", "", fmt.Errorf("%s changed type or identity while opening", rel)
	}
	body, err := io.ReadAll(io.LimitReader(file, maxArtifactBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", rel, err)
	}
	if len(body) > maxArtifactBytes {
		return "", "", fmt.Errorf("%s exceeds %d bytes", rel, maxArtifactBytes)
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return "", "", fmt.Errorf("%s changed while reading", rel)
	}
	sum := sha256.Sum256(body)
	return string(body), hex.EncodeToString(sum[:]), nil
}

func writeManifest(binding CycleBinding, view *phaseio.ExplanationView) error {
	path, err := safeManifestPath(binding.ProjectRoot, binding.Workspace)
	if err != nil {
		return err
	}
	return atomicwrite.JSON(path, view)
}

func safeManifestPath(projectRoot, workspace string) (string, error) {
	if projectRoot == "" || workspace == "" {
		return "", fmt.Errorf("project root and workspace are required")
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	if !within(absRoot, absWorkspace) {
		return "", fmt.Errorf("workspace must be inside the project root")
	}
	if err := requireRealDirectory(absRoot, "project root"); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absWorkspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace relative path: %w", err)
	}
	current := absRoot
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		if err := requireRealDirectory(current, "workspace"); err != nil {
			return "", err
		}
	}
	target := filepath.Join(absWorkspace, manifestFilename)
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("workspace manifest must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect workspace manifest: %w", err)
	}
	return target, nil
}

func requireRealDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must be a real directory", label)
	}
	return nil
}

func normalize(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return ""
	}
	return path
}

func validRelative(path string) bool {
	return path != "" && path != "." && !filepath.IsAbs(path) && path != ".." && !strings.HasPrefix(path, "../")
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func stringSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, path := range paths {
		if path = normalize(path); path != "" {
			out[path] = true
		}
	}
	return out
}

func uniqueSorted(paths []string) []string {
	set := stringSet(paths)
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
