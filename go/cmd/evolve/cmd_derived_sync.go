package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
)

const (
	docsIndexUsage = "usage: evolve docs index <generate|check> [--project-root P]"
	darwinSetUsage = "usage: evolve ci darwin-set <generate|check> [--project-root P]"
)

type syncVerb struct {
	usage, prefix, output string
	stale                 error
	sync                  func(root string, write bool) (bool, error)
}

var (
	docsIndexVerb = syncVerb{
		usage: docsIndexUsage, prefix: "evolve docs index: ", output: researchindex.IndexPath, stale: researchindex.ErrStale,
		sync: func(root string, write bool) (bool, error) {
			if write {
				return researchindex.Generate(root)
			}
			return false, researchindex.Check(root)
		},
	}
	darwinSetVerb = syncVerb{
		usage: darwinSetUsage, prefix: "evolve ci darwin-set: ", output: "go/" + ciparity.DarwinSetFile, stale: ciparity.ErrDarwinSetStale,
		sync: func(root string, write bool) (bool, error) {
			return ciparity.SyncDarwinSet(filepath.Join(root, ciModuleDir), write)
		},
	}
)

func (v syncVerb) run(args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	operands, err := cliFlags{values: map[string]*string{"--project-root": &projectRoot}}.parse(args)
	if err != nil || len(operands) != 1 || (operands[0] != "generate" && operands[0] != "check") {
		fmt.Fprintln(stderr, v.usage)
		return exitUsage
	}
	if projectRoot == "" {
		projectRoot = sourceRoot()
	}
	write := operands[0] == "generate"
	changed, err := v.sync(projectRoot, write)
	switch {
	case errors.Is(err, v.stale):
		fmt.Fprintf(stderr, "%s%v\n", v.prefix, err)
		return exitRefused
	case err != nil:
		fmt.Fprintf(stderr, "%s%v\n", v.prefix, err)
		return exitIO
	case changed:
		fmt.Fprintf(stdout, "%sregenerated %s\n", v.prefix, v.output)
	case write:
		fmt.Fprintf(stdout, "%s%s is up to date\n", v.prefix, v.output)
	default:
		fmt.Fprintf(stdout, "%s%s is in sync\n", v.prefix, v.output)
	}
	return 0
}
