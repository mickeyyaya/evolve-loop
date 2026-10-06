package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	backupsUsage      = "usage: evolve backups verify [--dir D]"
	backupsGitTimeout = 5 * time.Minute
)

func runBackups(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		fmt.Fprintln(stderr, backupsUsage)
		return 2
	}
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	dir := filepath.Join(filepath.Dir(root), "backups")
	rest := args[1:]
	for len(rest) > 0 {
		if rest[0] != "--dir" || len(rest) < 2 {
			fmt.Fprintf(stderr, "backups verify: bad argument %q\n%s\n", rest[0], backupsUsage)
			return 2
		}
		dir, rest = rest[1], rest[2:]
	}
	ctx, cancel := context.WithTimeout(context.Background(), backupsGitTimeout)
	defer cancel()
	return verifyBackups(ctx, gitexec.Default(root), dir, stdout, stderr)
}

func verifyBackups(ctx context.Context, g gitexec.Git, dir string, stdout, stderr io.Writer) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(stderr, "backups verify: %v\n", err)
		return 2
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tFILE\tHEAD\tSTATUS")
	var unsafe []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		var found []string
		switch {
		case e.IsDir():
		case strings.HasSuffix(e.Name(), ".bundle"):
			found, err = verifyBundle(ctx, g, tw, e.Name(), path)
		case strings.HasSuffix(e.Name(), ".patch"):
			found, err = verifyPatch(ctx, g, tw, e.Name(), path)
		}
		if err != nil {
			_ = tw.Flush()
			fmt.Fprintf(stderr, "backups verify: %v\n", err)
			return 2
		}
		unsafe = append(unsafe, found...)
	}
	_ = tw.Flush()
	for _, u := range unsafe {
		fmt.Fprintf(stderr, "backups verify: refusing: %s\n", u)
	}
	if len(unsafe) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "backups verify: every head and patch exists elsewhere")
	return 0
}

func verifyBundle(ctx context.Context, g gitexec.Git, w io.Writer, name, path string) ([]string, error) {
	heads, err := g.BundleHeads(ctx, path)
	if err != nil {
		return nil, err
	}
	var unsafe []string
	for _, h := range heads {
		place, err := g.ClassifyHead(ctx, h.SHA)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "BUNDLE\t%s\t%s\t%s\n", name, h.SHA, place)
		if place == gitexec.HeadOnlyInBundle {
			unsafe = append(unsafe, fmt.Sprintf("head %s (%s %s) exists only in the backup", h.SHA, name, h.Ref))
		}
	}
	return unsafe, nil
}

func verifyPatch(ctx context.Context, g gitexec.Git, w io.Writer, name, path string) ([]string, error) {
	applied, err := g.PatchApplied(ctx, path)
	if err != nil {
		return nil, err
	}
	if applied {
		fmt.Fprintf(w, "PATCH\t%s\t-\tAPPLIED\n", name)
		return nil, nil
	}
	fmt.Fprintf(w, "PATCH\t%s\t-\tUNAPPLIED\n", name)
	return []string{fmt.Sprintf("patch %s is not applied to origin/main", name)}, nil
}
