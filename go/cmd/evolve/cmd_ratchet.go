package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const ratchetUsage = "usage: evolve ratchet check [size|rawgit] [--root DIR]"

var ratchetScans = map[string]func(root string, stderr io.Writer) bool{
	"size":   sizeRatchetClean,
	"rawgit": rawGitRatchetClean,
}

func runRatchet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, ratchetUsage)
		return 2
	}
	fs := flag.NewFlagSet("ratchet check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "Go module root to scan")
	positional, err := parseInterspersed(fs, args[1:])
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
