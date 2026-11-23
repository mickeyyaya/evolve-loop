package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/workflowlint"
)

const yamlUsage = "usage: evolve yaml check [--root DIR] [PATH...]"

func runYAML(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runYAMLWith(yamlEnv{dir: ".", run: sysexec.DefaultRunner}, args, stdout, stderr)
}

type yamlEnv struct {
	dir string
	run sysexec.RunFunc
}

func runYAMLWith(env yamlEnv, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, yamlUsage)
		return exitUsage
	}
	fs := flag.NewFlagSet("yaml check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repository root (default: the top level of the working directory's repository)")
	paths, err := parseInterspersed(fs, args[1:])
	if err != nil {
		fmt.Fprintln(stderr, yamlUsage)
		return exitUsage
	}
	ctx := context.Background()
	if *root == "" {
		top, err := gitexec.Git{Dir: env.dir, Exec: env.run}.Output(ctx, "rev-parse", "--show-toplevel")
		if err != nil {
			fmt.Fprintf(stderr, "evolve yaml: %v\n", err)
			return exitIO
		}
		*root = top
	}
	if len(paths) == 0 {
		present, err := repostate.PresentFiles(ctx, gitexec.Git{Dir: *root, Exec: env.run})
		if err != nil {
			fmt.Fprintf(stderr, "evolve yaml: %v\n", err)
			return exitIO
		}
		paths = yamlPaths(present)
	}
	files, unreadable := readYAMLFiles(*root, paths)
	findings := append(unreadable, workflowlint.Check(files)...)
	for _, f := range findings {
		fmt.Fprintln(stdout, f)
	}
	if len(findings) > 0 {
		return exitRefused
	}
	fmt.Fprintf(stdout, "yaml check: %d files clean\n", len(paths))
	return 0
}

func yamlPaths(all []string) []string {
	var paths []string
	for _, path := range all {
		if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			paths = append(paths, path)
		}
	}
	return paths
}

func readYAMLFiles(root string, paths []string) ([]workflowlint.File, []workflowlint.Finding) {
	files := make([]workflowlint.File, 0, len(paths))
	var unreadable []workflowlint.Finding
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			unreadable = append(unreadable, workflowlint.Finding{Path: path, Msg: err.Error()})
			continue
		}
		files = append(files, workflowlint.File{Path: path, Data: data})
	}
	return files, unreadable
}
