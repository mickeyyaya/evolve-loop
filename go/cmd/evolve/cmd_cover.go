package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const coverUsage = "usage: evolve cover changed --base <ref> [--profile <path>] [--packages <file>] [--json] [--root DIR]\n       evolve cover strict [--profile <path>] [--floors <path>] [--packages <file>] [--json] [--root DIR]"

var coverChangedRunner sysexec.RunFunc = sysexec.DefaultRunner

type coverChangedArgs struct {
	root, base, profile, packages string
	asJSON                        bool
}

func runCover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "strict" {
		return runCoverStrict(args[1:], stdout, stderr)
	}
	return runCoverChanged(args, stdout, stderr)
}

func runCoverChanged(args []string, stdout, stderr io.Writer) int {
	in, ok := parseCoverChanged(args, stderr)
	if !ok {
		return exitUsage
	}
	moduleDir, err := coverModuleDir(in.root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover changed: %v\n", err)
		return exitIO
	}
	if in.profile == "" {
		in.profile = filepath.Join(moduleDir, "coverage.txt")
		if _, err := os.Stat(in.profile); errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "evolve cover changed: no coverage profile at %s; run `make -C go test-integration` first, then run this check again\n", in.profile)
			return exitIO
		}
	}
	packages, err := readPackageSet(in.packages)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover changed: %v\n", err)
		return exitIO
	}
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: coverChangedRunner}
	v, err := ws.Check(context.Background(), covchanged.Options{Base: in.base, Profile: in.profile, Tags: covchanged.DefaultTags, Packages: packages})
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover changed: %v\n", err)
		return exitIO
	}
	if err := printCoverVerdict(stdout, v, in); err != nil {
		fmt.Fprintf(stderr, "evolve cover changed: write the verdict: %v\n", err)
		return exitIO
	}
	if !v.Clean() {
		return exitRefused
	}
	return 0
}

func parseCoverChanged(args []string, stderr io.Writer) (coverChangedArgs, bool) {
	var in coverChangedArgs
	if len(args) == 0 || args[0] != "changed" {
		fmt.Fprintln(stderr, coverUsage)
		return in, false
	}
	flags := flag.NewFlagSet("cover changed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&in.root, "root", ".", "repository root or Go module root")
	flags.StringVar(&in.base, "base", "", "the git ref the change is measured against")
	flags.StringVar(&in.profile, "profile", "", "coverage profile (default: <module>/coverage.txt)")
	flags.StringVar(&in.packages, "packages", "", packagesFlagUsage)
	flags.BoolVar(&in.asJSON, "json", false, "print the verdict as JSON")
	positional, err := parseInterspersed(flags, args[1:])
	switch {
	case err != nil:
		fmt.Fprintln(stderr, coverUsage)
		return in, false
	case len(positional) > 0:
		fmt.Fprintf(stderr, "evolve cover changed: unexpected arguments %q\n%s\n", positional, coverUsage)
		return in, false
	case in.base == "":
		fmt.Fprintf(stderr, "evolve cover changed: --base is required\n%s\n", coverUsage)
		return in, false
	}
	return in, true
}

const packagesFlagUsage = "file of package directories, one per line, relative to the working directory; judge only those packages (default: every package)"

func readPackageSet(file string) (covchanged.PackageSet, error) {
	if file == "" {
		return nil, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read the package set: %w", err)
	}
	return covchanged.ParsePackageSet(data)
}

func coverModuleDir(root string) (string, error) {
	for _, dir := range []string{root, filepath.Join(root, "go")} {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no go.mod at %s or %s", root, filepath.Join(root, "go"))
}

func printCoverVerdict(w io.Writer, v covchanged.Verdict, in coverChangedArgs) error {
	if in.asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Clean bool `json:"clean"`
			covchanged.Verdict
		}{v.Clean(), v})
	}
	for _, problem := range v.Problems() {
		fmt.Fprintln(w, "cover changed: "+problem)
	}
	_, err := fmt.Fprintf(w, "cover changed: %d changed, %d full, %d refused, %d exempt\n", v.Changed, v.Full, len(v.Refused), len(v.Exempt))
	return err
}
