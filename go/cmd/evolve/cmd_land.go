package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	landUsage  = "usage: evolve land --branch B (--patch F | --salvage <leaf>) [--project-root P]"
	landPrefix = "evolve land: "
)

type landArgs struct {
	branch, patch, salvage, projectRoot string
}

type landInput struct {
	patch, untracked string
}

func parseLandArgs(args []string) (landArgs, error) {
	var a landArgs
	operands, err := cliFlags{values: map[string]*string{
		"--branch": &a.branch, "--patch": &a.patch, "--salvage": &a.salvage, "--project-root": &a.projectRoot,
	}}.parse(args)
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected argument %q", operands[0])
	case a.branch == "":
		return a, errors.New("--branch is required")
	case (a.patch == "") == (a.salvage == ""):
		return a, errors.New("name exactly one of --patch or --salvage")
	}
	return a, nil
}

func runLand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	a, err := parseLandArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", landPrefix, err, landUsage)
		return exitUsage
	}
	root, err := loopStopRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", landPrefix, err)
		return exitIO
	}
	in, err := resolveLandInput(a, root)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", landPrefix, err)
		return exitIO
	}
	task := strings.ReplaceAll(a.branch, "/", "-")
	if rc := runWorktreeCreateDev(root, task, a.branch, stdout, stderr); rc != 0 {
		return rc
	}
	hub, err := resolveDevHub(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", landPrefix, err)
		return exitIO
	}
	return landInto(hub.taskDir(task), in, stdout, stderr)
}

func resolveLandInput(a landArgs, root string) (landInput, error) {
	if a.patch != "" {
		patch, err := regularFile(a.patch)
		if err != nil {
			return landInput{}, fmt.Errorf("patch %s: %w", a.patch, err)
		}
		return landInput{patch: patch}, nil
	}
	leaf, err := salvageLeafDir(a.salvage, root)
	if err != nil {
		return landInput{}, err
	}
	var in landInput
	if p, err := regularFile(filepath.Join(leaf, "uncommitted.patch")); err == nil {
		in.patch = p
	} else if !errors.Is(err, os.ErrNotExist) {
		return landInput{}, fmt.Errorf("salvage leaf %s: %w", a.salvage, err)
	}
	if p, err := regularFile(filepath.Join(leaf, "untracked.tgz")); err == nil {
		in.untracked = p
	} else if !errors.Is(err, os.ErrNotExist) {
		return landInput{}, fmt.Errorf("salvage leaf %s: %w", a.salvage, err)
	}
	if in == (landInput{}) {
		return landInput{}, fmt.Errorf("salvage leaf %s holds neither uncommitted.patch nor untracked.tgz", a.salvage)
	}
	return in, nil
}

func salvageLeafDir(leaf, root string) (string, error) {
	candidates := []string{leaf}
	if !strings.ContainsAny(leaf, `/\`) {
		candidates = append(candidates, filepath.Join(gc.OperatorSalvageDir(paths.EvolveDirOf(root)), leaf))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return filepath.Abs(c)
		}
	}
	return "", fmt.Errorf("salvage leaf %s: no such directory (looked in %s)", leaf, strings.Join(candidates, ", "))
}

func regularFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	return filepath.Abs(path)
}

func landInto(dir string, in landInput, stdout, stderr io.Writer) int {
	rc := 0
	if in.patch != "" {
		rc = applyLandPatch(dir, in.patch, stderr)
		if rc == exitIO {
			return rc
		}
	}
	if in.untracked != "" {
		if err := extractUntracked(in.untracked, dir); err != nil {
			fmt.Fprintf(stderr, "%srestore untracked files into %s: %v\n", landPrefix, dir, err)
			return exitIO
		}
	}
	if rc != 0 {
		return rc
	}
	fmt.Fprintf(stdout, "land: landed uncommitted on origin/main in %s; nothing was committed or pushed\n", dir)
	fmt.Fprintf(stdout, "land: next: cd %s, review the diff, commit through the commit gate (/commit), then evolve ship --class manual\n", dir)
	return 0
}

func applyLandPatch(dir, patch string, stderr io.Writer) int {
	ctx := context.Background()
	wt := gitexec.Default(dir)
	if info, err := os.Stat(patch); err == nil && info.Size() == 0 {
		return 0
	}
	_, applyErr, code, err := wt.Capture(ctx, "apply", "--3way", patch)
	if err != nil {
		fmt.Fprintf(stderr, "%sgit apply --3way: %v\n", landPrefix, err)
		return exitIO
	}
	if code == 0 {
		return 0
	}
	conflicted, _, diffCode, err := wt.Capture(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil || diffCode != 0 {
		fmt.Fprintf(stderr, "%sgit apply --3way rc=%d: %s\n", landPrefix, code, strings.TrimSpace(applyErr))
		return exitIO
	}
	if files := strings.Fields(conflicted); len(files) > 0 {
		fmt.Fprintf(stderr, "%srefused: the patch conflicts with origin/main in %s; resolve the conflict markers in %s\n", landPrefix, strings.Join(files, ", "), dir)
		return exitRefused
	}
	fmt.Fprintf(stderr, "%srefused: the patch does not apply to origin/main (worktree %s left for inspection): %s\n", landPrefix, dir, strings.TrimSpace(applyErr))
	return exitRefused
}

func extractUntracked(archive, dir string) (err error) {
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
		if err := restoreEntry(tr, hdr, dir); err != nil {
			return fmt.Errorf("%s: %w", hdr.Name, err)
		}
	}
}

func restoreEntry(r io.Reader, hdr *tar.Header, dir string) error {
	name := filepath.FromSlash(hdr.Name)
	if !filepath.IsLocal(name) {
		return errors.New("entry escapes the worktree")
	}
	dest := filepath.Join(dir, name)
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(dest, 0o755)
	case tar.TypeReg:
	default:
		return fmt.Errorf("unsupported entry type %q; restore it by hand", hdr.Typeflag)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, hdr.FileInfo().Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return errors.Join(err, out.Close())
}
