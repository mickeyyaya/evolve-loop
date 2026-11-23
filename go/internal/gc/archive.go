package gc

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

const (
	archiveRefRoot   = "refs/archive/cycles/"
	manifestFile     = "manifest.json"
	manifestHashFile = "manifest.sha256"
)

var leafFileNames = []string{salvageHeadFile, salvagePatchFile, salvageUntrackedFile, salvageBundleFile}

type leafManifest struct {
	Cycle  int               `json:"cycle"`
	Branch string            `json:"branch"`
	Head   string            `json:"head"`
	Ref    string            `json:"ref"`
	Files  map[string]string `json:"files"`
}

type RestoreResult struct {
	Leaf     string
	Branch   string
	Head     string
	Worktree string
}

func archiveRef(leaf string) string { return archiveRefRoot + leaf }

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(body), nil
}

func writeLeafManifest(dir string) error {
	head, branch, err := readSalvageHead(filepath.Join(dir, salvageHeadFile))
	if err != nil {
		return err
	}
	leaf := filepath.Base(dir)
	cycle, _ := LeafCycleNumber(leaf)
	m := leafManifest{Cycle: cycle, Branch: branch, Head: head, Ref: archiveRef(leaf), Files: map[string]string{}}
	for _, name := range leafFileNames {
		sum, err := hashFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("hash %s: %w", name, err)
		}
		m.Files[name] = sum
	}
	body, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, manifestFile), body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", manifestFile, err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestHashFile), []byte(sha256Hex(body)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", manifestHashFile, err)
	}
	return nil
}

func (o WorktreeOptions) archiveBranch(leaf string, it WorktreeItem) error {
	if it.Head == "" || it.Branch == "" {
		return nil
	}
	if _, err := o.git(o.ProjectRoot, "update-ref", archiveRef(leaf), it.Head); err != nil {
		return fmt.Errorf("%w: %w", ErrBranchKept, err)
	}
	if _, err := o.git(o.ProjectRoot, "update-ref", "-d", "refs/heads/"+it.Branch, it.Head); err != nil {
		return fmt.Errorf("%w: branch %s moved since the plan: %w", ErrBranchKept, it.Branch, err)
	}
	o.logDeleted(it.Branch, it.Head)
	return nil
}

func (o WorktreeOptions) findLeaf(cycle int) (string, error) {
	root := OperatorSalvageDir(o.EvolveDir)
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("gc: restore: read %s: %w", root, err)
	}
	var found []string
	for _, e := range entries {
		if n, ok := LeafCycleNumber(e.Name()); e.IsDir() && ok && n == cycle && strings.HasPrefix(e.Name(), "cycle-") {
			found = append(found, e.Name())
		}
	}
	sort.Strings(found)
	switch len(found) {
	case 0:
		return "", fmt.Errorf("gc: restore: no salvage leaf for cycle %d under %s", cycle, root)
	case 1:
		return filepath.Join(root, found[0]), nil
	}
	return "", fmt.Errorf("gc: restore: cycle %d has %d salvage leaves (%s), none restored", cycle, len(found), strings.Join(found, ", "))
}

func verifyLeaf(dir string) (leafManifest, error) {
	var m leafManifest
	body, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return m, fmt.Errorf("gc: restore: read %s: %w", manifestFile, err)
	}
	recorded, err := os.ReadFile(filepath.Join(dir, manifestHashFile))
	if err != nil {
		return m, fmt.Errorf("gc: restore: read %s: %w", manifestHashFile, err)
	}
	if got := sha256Hex(body); strings.TrimSpace(string(recorded)) != got {
		return m, fmt.Errorf("gc: restore: %s does not match %s (it hashes to %s)", manifestFile, manifestHashFile, got)
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, fmt.Errorf("gc: restore: parse %s: %w", manifestFile, err)
	}
	if !salvageHeadSHARe.MatchString(m.Head) || m.Branch == "" || m.Ref != archiveRef(filepath.Base(dir)) {
		return m, fmt.Errorf("gc: restore: %s names head %q, branch %q, ref %q: not a leaf this tool wrote", manifestFile, m.Head, m.Branch, m.Ref)
	}
	for name, want := range m.Files {
		if filepath.Base(name) != name {
			return m, fmt.Errorf("gc: restore: %s lists %q: not a file of the leaf", manifestFile, name)
		}
		got, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return m, fmt.Errorf("gc: restore: %s listed in %s: %w", name, manifestFile, err)
		}
		if got != want {
			return m, fmt.Errorf("gc: restore: %s does not match %s: manifest %s, file %s", name, manifestFile, want, got)
		}
	}
	return m, nil
}

func (o WorktreeOptions) refValue(ref string) (string, bool) {
	out, err := o.git(o.ProjectRoot, "rev-parse", "--verify", "--quiet", ref)
	return strings.TrimSpace(out), err == nil
}

func (o WorktreeOptions) ensureArchiveRef(dir string, m leafManifest) error {
	if cur, ok := o.refValue(m.Ref); ok {
		if cur != m.Head {
			return fmt.Errorf("gc: restore: %s is %s, the manifest says %s", m.Ref, cur, m.Head)
		}
		return nil
	}
	if _, ok := o.refValue(m.Head + "^{commit}"); ok {
		_, err := o.git(o.ProjectRoot, "update-ref", m.Ref, m.Head)
		return err
	}
	if _, ok := m.Files[salvageBundleFile]; !ok {
		return fmt.Errorf("gc: restore: %s is gone and the leaf holds no %s", m.Ref, salvageBundleFile)
	}
	bundle := filepath.Join(dir, salvageBundleFile)
	if _, err := o.git(o.ProjectRoot, "fetch", "--quiet", bundle, "refs/heads/"+m.Branch+":"+m.Ref); err != nil {
		return err
	}
	if cur, ok := o.refValue(m.Ref); !ok || cur != m.Head {
		return fmt.Errorf("gc: restore: the bundle gave %s = %q, the manifest says %s", m.Ref, cur, m.Head)
	}
	return nil
}

func RestoreArchive(o WorktreeOptions, cycle int, worktree string) (RestoreResult, error) {
	var res RestoreResult
	if o.Exec == nil {
		return res, errors.New("gc: RestoreArchive requires Exec")
	}
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return res, fmt.Errorf("gc: acquire ship.lock: %w", err)
	}
	defer release()
	dir, err := o.findLeaf(cycle)
	if err != nil {
		return res, err
	}
	m, err := verifyLeaf(dir)
	if err != nil {
		return res, err
	}
	if worktree == "" {
		worktree = filepath.Join(o.WorktreeBase, filepath.Base(dir))
	}
	if _, ok := o.refValue("refs/heads/" + m.Branch); ok {
		return res, fmt.Errorf("gc: restore: branch %s already exists, nothing restored", m.Branch)
	}
	if _, err := os.Lstat(worktree); err == nil {
		return res, fmt.Errorf("gc: restore: %s already exists, nothing restored", worktree)
	}
	if err := o.ensureArchiveRef(dir, m); err != nil {
		return res, err
	}
	if _, err := o.git(o.ProjectRoot, "worktree", "add", "--quiet", "-b", m.Branch, worktree, m.Head); err != nil {
		return res, err
	}
	if err := o.restoreUncommitted(dir, worktree, m); err != nil {
		return res, errors.Join(err, o.undoRestore(worktree, m))
	}
	return RestoreResult{Leaf: filepath.Base(dir), Branch: m.Branch, Head: m.Head, Worktree: worktree}, nil
}

func (o WorktreeOptions) restoreUncommitted(dir, worktree string, m leafManifest) error {
	if _, ok := m.Files[salvagePatchFile]; ok {
		if _, err := o.git(worktree, "apply", "--binary", filepath.Join(dir, salvagePatchFile)); err != nil {
			return err
		}
	}
	if _, ok := m.Files[salvageUntrackedFile]; ok {
		return extractUntracked(filepath.Join(dir, salvageUntrackedFile), worktree)
	}
	return nil
}

func (o WorktreeOptions) undoRestore(worktree string, m leafManifest) error {
	var errs []error
	if _, err := o.git(o.ProjectRoot, "worktree", "remove", "--force", worktree); err != nil {
		errs = append(errs, err)
	}
	if _, err := o.git(o.ProjectRoot, "update-ref", "-d", "refs/heads/"+m.Branch, m.Head); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func extractUntracked(archive, root string) (err error) {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractEntry(tr, hdr, root); err != nil {
			return fmt.Errorf("restore %s: %w", hdr.Name, err)
		}
	}
}

func extractEntry(tr *tar.Reader, hdr *tar.Header, root string) error {
	if !filepath.IsLocal(hdr.Name) {
		return errors.New("path leaves the worktree")
	}
	dest := filepath.Join(root, hdr.Name)
	if err := requirePlainParents(root, filepath.Dir(dest)); err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(dest, 0o755)
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.Symlink(hdr.Linkname, dest)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, hdr.FileInfo().Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		return errors.Join(err, out.Close())
	}
	return fmt.Errorf("entry type %q is not restorable", hdr.Typeflag)
}

func requirePlainParents(root, dir string) error {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	cur := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("parent %s is a symlink", cur)
		}
	}
	return nil
}
