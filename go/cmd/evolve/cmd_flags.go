// cmd_flags.go — `evolve flags generate|check` (L2.2, concurrency-factory
// plan): projects the internal/flagregistry SSOT into a marker-delimited
// region of docs/architecture/control-flags.md, exactly like `evolve skills
// generate|check` projects phase facts (ADR-0040). Hand-written cluster
// prose outside the markers is preserved byte-for-byte; the generated region
// is the complete flat flag index. `check` exits 2 on drift so CI can gate
// undocumented flags.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/internal/skillcheck"
)

const (
	flagIndexBegin = "<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->"
	flagIndexEnd   = "<!-- GENERATED:flag-index END -->"
)

func runFlags(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve flags <generate|check>")
		return 10
	}
	// control-flags.md is a generated SOURCE doc, part of a cycle's committed
	// deliverable — resolve it from the worktree under the ACS suite, not main's
	// stale working copy (cycle-355 fix; see sourceRoot).
	project := sourceRoot()
	docPath := filepath.Join(project, "docs", "architecture", "control-flags.md")
	switch args[0] {
	case "generate":
		return flagsRun(docPath, true, stdout, stderr)
	case "check":
		return flagsRun(docPath, false, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want generate|check)\n", args[0])
		return 10
	}
}

// flagsRun renders the expected doc and either writes it (generate) or
// compares against disk (check; exit 2 on drift — the L2.3 CI contract).
func flagsRun(docPath string, write bool, stdout, stderr io.Writer) int {
	doc, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", docPath, err)
		return 1
	}
	block := flagIndexBegin + "\n\n## Generated Flag Index\n\n" + flagregistry.RenderIndex() + "\n" + flagIndexEnd
	next, err := skillcheck.SpliceMarkedRegion(string(doc), block, flagIndexBegin, flagIndexEnd, "")
	if err != nil {
		fmt.Fprintf(stderr, "splice %s: %v\n", docPath, err)
		return 1
	}
	if write {
		if next != string(doc) {
			if werr := os.WriteFile(docPath, []byte(next), 0o644); werr != nil {
				fmt.Fprintf(stderr, "write %s: %v\n", docPath, werr)
				return 1
			}
			fmt.Fprintf(stdout, "flags: regenerated index (%d flags) in %s\n", len(flagregistry.All), docPath)
		} else {
			fmt.Fprintf(stdout, "flags: index up to date (%d flags)\n", len(flagregistry.All))
		}
		return 0
	}
	if next != string(doc) {
		fmt.Fprintf(stderr, "flags: %s is stale vs flagregistry — run `evolve flags generate`\n", docPath)
		return 2
	}
	fmt.Fprintf(stdout, "flags: index in sync (%d flags)\n", len(flagregistry.All))
	return 0
}
