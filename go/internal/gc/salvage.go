package gc

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrTreeKept = errors.New("tree kept")

var salvageReadlink = os.Readlink

func (o WorktreeOptions) salvageRemoveWorktrees(items []WorktreeItem, refused map[string]bool) (bool, []error) {
	var errs []error
	did := false
	for _, it := range items {
		if it.Action != WorktreeActionSalvageRemove || refused[it.Path] {
			continue
		}
		if got := o.branchHead(it.Branch); it.Head != "" && got != it.Head {
			errs = append(errs, fmt.Errorf("gc: refuse %s: branch %s moved from %s to %s since the plan, %w", it.Path, it.Branch, it.Head, got, ErrTreeKept))
			continue
		}
		if err := o.salvageWorktree(it.Path, it.Branch); err != nil {
			errs = append(errs, fmt.Errorf("gc: refuse %s: salvage failed, %w: %w", it.Path, ErrTreeKept, err))
			continue
		}
		if o.isLive(it.Path) {
			errs = append(errs, fmt.Errorf("gc: refuse %s: became live during its salvage, %w", it.Path, ErrTreeKept))
			continue
		}
		if _, err := o.git(o.ProjectRoot, "worktree", "remove", "--force", it.Path); err != nil {
			errs = append(errs, err)
			continue
		}
		did = true
		if err := o.archiveBranch(filepath.Base(it.Path), it); err != nil {
			errs = append(errs, fmt.Errorf("gc: archive %s: %w", it.Branch, err))
		}
	}
	return did, errs
}

type gitRun func(dir string, args ...string) (string, error)

func (o WorktreeOptions) salvageWorktree(path, branch string) error {
	if err := writeSalvageLeaf(o.git, o.EvolveDir, path, branch); err != nil {
		return err
	}
	return writeLeafManifest(filepath.Join(OperatorSalvageDir(o.EvolveDir), filepath.Base(path)))
}

func writeSalvageLeaf(git gitRun, evolveDir, path, branch string) error {
	dir := filepath.Join(OperatorSalvageDir(evolveDir), filepath.Base(path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create salvage dir: %w", err)
	}
	head, err := git(path, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	tip := strings.TrimSpace(head)
	if err := os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(tip+" "+branch+"\n"), 0o644); err != nil {
		return fmt.Errorf("write salvage HEAD: %w", err)
	}
	patch, err := git(path, "diff", "HEAD", "--binary")
	if err != nil {
		return err
	}
	if patch != "" {
		if err := os.WriteFile(filepath.Join(dir, salvagePatchFile), []byte(patch), 0o644); err != nil {
			return fmt.Errorf("write salvage patch: %w", err)
		}
	}
	untracked, err := git(path, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	if err := archiveUntracked(path, untracked, filepath.Join(dir, salvageUntrackedFile)); err != nil {
		return err
	}
	return bundleVerified(git, path, filepath.Join(dir, salvageBundleFile), salvageRef(branch), tip)
}

func salvageRef(branch string) string {
	if branch == "" {
		return "HEAD"
	}
	return "refs/heads/" + branch
}

const salvageOriginMainRef = "refs/remotes/origin/main"

func bundleVerified(git gitRun, dir, file, ref, tip string) error {
	args := []string{"bundle", "create", file, ref}
	if _, err := originMainSHA(git, dir); err == nil {
		if count, err := commitsAhead(git, dir, salvageOriginMainRef, tip); err == nil {
			if count == 0 {
				return nil
			}
			args = append(args, "^"+salvageOriginMainRef)
		}
	}
	if _, err := git(dir, args...); err != nil {
		return err
	}
	_, heads, err := verifyBundle(git, dir, file)
	if err != nil {
		return err
	}
	if !strings.Contains(heads, tip+" "+ref) {
		return fmt.Errorf("bundle %s does not hold %s@%s", file, ref, tip)
	}
	return nil
}

func verifyBundle(git gitRun, dir, file string) (verified, heads string, err error) {
	if verified, err = git(dir, "bundle", "verify", file); err != nil {
		return "", "", err
	}
	heads, err = git(dir, "bundle", "list-heads", file)
	return verified, heads, err
}

func originMainSHA(git gitRun, dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--verify", "--quiet", salvageOriginMainRef)
	sha := strings.TrimSpace(out)
	if err == nil && !salvageHeadSHARe.MatchString(sha) {
		err = fmt.Errorf("%s resolves to %q", salvageOriginMainRef, sha)
	}
	return sha, err
}

func commitsAhead(git gitRun, dir, base, tip string) (int, error) {
	out, err := git(dir, "rev-list", "--count", base+".."+tip)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err == nil && n < 0 {
		err = fmt.Errorf("rev-list counted %d commits", n)
	}
	return n, err
}

func archiveUntracked(root, nulList, dest string) (err error) {
	names := strings.FieldsFunc(nulList, func(r rune) bool { return r == 0 })
	if len(names) == 0 {
		return nil
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create untracked archive: %w", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	defer func() { err = errors.Join(err, tw.Close(), gz.Close(), f.Close()) }()
	for _, name := range names {
		if err := addTarEntry(tw, root, name); err != nil {
			return fmt.Errorf("archive %s: %w", name, err)
		}
	}
	return nil
}

func addTarEntry(tw *tar.Writer, root, name string) error {
	p := filepath.Join(root, name)
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = salvageReadlink(p); err != nil {
			return err
		}
	}
	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(name)
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	src, err := os.Open(p)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	_, err = io.Copy(tw, src)
	return err
}

const (
	salvageHeadFile      = "HEAD"
	salvagePatchFile     = "uncommitted.patch"
	salvageUntrackedFile = "untracked.tgz"
)

func OperatorSalvageDir(evolveDir string) string { return filepath.Join(evolveDir, "operator-salvage") }
