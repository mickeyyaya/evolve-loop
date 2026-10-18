package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const ratchetUsage = "usage: evolve ratchet check [size|rawgit] [--root DIR] | evolve ratchet tighten [--check] [--root DIR] [--project-root P]"

var ratchetScans = map[string]func(root string, stderr io.Writer) bool{
	"size":   sizeRatchetClean,
	"rawgit": rawGitRatchetClean,
}

var ratchetVerbs = map[string]func(args []string, stdout, stderr io.Writer) int{
	"check":   runRatchetCheck,
	"tighten": runRatchetTighten,
}

func runRatchet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || ratchetVerbs[args[0]] == nil {
		fmt.Fprintln(stderr, ratchetUsage)
		return 2
	}
	return ratchetVerbs[args[0]](args[1:], stdout, stderr)
}

func runRatchetCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ratchet check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "Go module root to scan")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return 2
	}
	selected, err := selectRatchets(positional)
	if err != nil {
		fmt.Fprintf(stderr, "evolve ratchet: %v\n%s\n", err, ratchetUsage)
		return 2
	}
	clean := true
	for _, name := range selected {
		clean = ratchetScans[name](*root, stderr) && clean
	}
	if !clean {
		return 1
	}
	fmt.Fprintln(stdout, "ratchet check: clean")
	return 0
}

func runRatchetTighten(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ratchet tighten", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "Go module root to tighten")
	projectRoot := fs.String("project-root", "", "project root whose run leases guard the write (default: EVOLVE_PROJECT_ROOT or the cwd)")
	check := fs.Bool("check", false, "report the slack and exit 1 instead of writing the list")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return 2
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "evolve ratchet: unexpected arguments %q\n%s\n", positional, ratchetUsage)
		return 2
	}
	if !*check {
		if err := refuseTightenWrite(*projectRoot); err != nil {
			fmt.Fprintf(stderr, "evolve ratchet tighten: %v\n", err)
			return 1
		}
	}
	changes, encoded, err := sizeratchet.TightenModule(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(changes) == 0 {
		fmt.Fprintln(stdout, "ratchet tighten: tight")
		return 0
	}
	fmt.Fprintln(stdout, strings.Join(changes, "\n"))
	if *check {
		return 1
	}
	if err := os.WriteFile(filepath.Join(*root, filepath.FromSlash(sizeratchet.OffendersRelPath)), encoded, 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func refuseTightenWrite(flagged string) error {
	root, err := routingProjectRoot(flagged, processGetwd)
	if err != nil {
		return err
	}
	return refuseBoundaryWrite(root, os.Getenv, time.Now(), "the offender list")
}

func selectRatchets(positional []string) ([]string, error) {
	switch {
	case len(positional) == 0:
		return []string{"size", "rawgit"}, nil
	case len(positional) > 1:
		return nil, fmt.Errorf("unexpected arguments %q", positional[1:])
	case ratchetScans[positional[0]] == nil:
		return nil, fmt.Errorf("unknown ratchet %q", positional[0])
	}
	return positional, nil
}

func sizeRatchetClean(root string, stderr io.Writer) bool {
	if err := sizeratchet.Scan(root); err != nil {
		fmt.Fprintln(stderr, err)
		return false
	}
	return true
}

func rawGitRatchetClean(root string, stderr io.Writer) bool {
	note, err := rawgitratchet.Scan(root)
	if note != "" {
		fmt.Fprintln(stderr, "evolve ratchet: "+note)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return false
	}
	return true
}
