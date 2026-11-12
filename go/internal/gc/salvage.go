package gc

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var salvageReadlink = os.Readlink

func (o WorktreeOptions) salvageRemoveWorktrees(items []WorktreeItem, refused map[string]bool) (bool, []error) {
	var errs []error
	did := false
	for _, it := range items {
		if it.Action != WorktreeActionSalvageRemove || refused[it.Path] {
			continue
		}
		if err := o.salvageWorktree(it.Path, it.Branch); err != nil {
			errs = append(errs, fmt.Errorf("gc: refuse %s: salvage failed, tree kept: %w", it.Path, err))
			continue
		}
		if o.isLive(it.Path) {
			errs = append(errs, fmt.Errorf("gc: refuse %s: became live during its salvage, tree kept", it.Path))
			continue
		}
		if _, err := o.git(o.ProjectRoot, "worktree", "remove", "--force", it.Path); err != nil {
			errs = append(errs, err)
			continue
		}
		did = true
	}
	return did, errs
}

type gitRun func(dir string, args ...string) (string, error)

func (o WorktreeOptions) salvageWorktree(path, branch string) error {
	return writeSalvageLeaf(o.git, o.EvolveDir, path, branch)
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

func bundleVerified(git gitRun, dir, file, ref, tip string) error {
	if _, err := git(dir, "bundle", "create", file, ref); err != nil {
		return err
	}
	if _, err := git(dir, "bundle", "verify", file); err != nil {
		return err
	}
	heads, err := git(dir, "bundle", "list-heads", file)
	if err != nil {
		return err
	}
	if !strings.Contains(heads, tip+" "+ref) {
		return fmt.Errorf("bundle %s does not hold %s@%s", file, ref, tip)
	}
	return nil
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
