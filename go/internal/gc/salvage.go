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

func (o WorktreeOptions) salvageRemoveWorktrees(items []WorktreeItem) (bool, []error) {
	var errs []error
	did := false
	for _, it := range items {
		if it.Action != WorktreeActionSalvageRemove {
			continue
		}
		if o.isLive(it.Path) {
			errs = append(errs, fmt.Errorf("gc: refuse %s: became live between plan and apply", it.Path))
			continue
		}
		if err := o.salvageWorktree(it.Path, it.Branch); err != nil {
			errs = append(errs, fmt.Errorf("gc: refuse %s: salvage failed, tree kept: %w", it.Path, err))
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

func (o WorktreeOptions) salvageWorktree(path, branch string) error {
	dir := filepath.Join(o.EvolveDir, "operator-salvage", filepath.Base(path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create salvage dir: %w", err)
	}
	head, err := o.git(path, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(strings.TrimSpace(head)+" "+branch+"\n"), 0o644); err != nil {
		return fmt.Errorf("write salvage HEAD: %w", err)
	}
	patch, err := o.git(path, "diff", "HEAD", "--binary")
	if err != nil {
		return err
	}
	if patch != "" {
		if err := os.WriteFile(filepath.Join(dir, "uncommitted.patch"), []byte(patch), 0o644); err != nil {
			return fmt.Errorf("write salvage patch: %w", err)
		}
	}
	untracked, err := o.git(path, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	return archiveUntracked(path, untracked, filepath.Join(dir, "untracked.tgz"))
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
		if link, err = os.Readlink(p); err != nil {
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
	defer src.Close()
	_, err = io.Copy(tw, src)
	return err
}
