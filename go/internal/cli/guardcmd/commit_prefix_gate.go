package guardcmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolveloop/go/internal/commitprefixgate"
)

// runCommitPrefixGate is `evolve commit-prefix-gate --msg "<msg>" [flags]`.
// Mirrors legacy/scripts/guards/commit-prefix-gate.sh exit codes:
//
//	0 = prefix matches scope (or unknown prefix / pass-through cases)
//	2 = scope violation
//	3 = bad arguments
//	4 = manifest missing or malformed
func RunCommitPrefixGate(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		commitMsg    string
		repoDir      string
		mode         = commitprefixgate.ModeStaged
		diffRef      string
		manifestPath string
		bypass       bool
	)

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve commit-prefix-gate --msg \"<commit-msg>\" [--repo-dir <path>] [--staged | --diff-ref <ref>] [--manifest <path>] [--bypass]")
			fmt.Fprintln(stdout, "Emergency bypass requires --bypass + SHIP_CLASS=manual.")
			return 0
		case a == "--msg":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[commit-prefix-gate] --msg missing value")
				return 3
			}
			commitMsg = args[i]
		case len(a) > 6 && a[:6] == "--msg=":
			commitMsg = a[6:]
		case a == "--repo-dir":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[commit-prefix-gate] --repo-dir missing value")
				return 3
			}
			repoDir = args[i]
		case len(a) > 11 && a[:11] == "--repo-dir=":
			repoDir = a[11:]
		case a == "--staged":
			mode = commitprefixgate.ModeStaged
		case a == "--bypass":
			bypass = true
		case a == "--diff-ref":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[commit-prefix-gate] --diff-ref missing value")
				return 3
			}
			mode = commitprefixgate.ModeRef
			diffRef = args[i]
		case len(a) > 11 && a[:11] == "--diff-ref=":
			mode = commitprefixgate.ModeRef
			diffRef = a[11:]
		case a == "--manifest":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[commit-prefix-gate] --manifest missing value")
				return 3
			}
			manifestPath = args[i]
		default:
			fmt.Fprintf(stderr, "[commit-prefix-gate] unknown arg: %s\n", a)
			return 3
		}
		i++
	}

	if commitMsg == "" {
		fmt.Fprintln(stderr, "[commit-prefix-gate] usage: --msg \"<commit-message>\" [...]")
		return 3
	}
	if repoDir == "" {
		cwd, _ := os.Getwd()
		repoDir = cwd
	}

	opts := commitprefixgate.Options{
		CommitMsg:    commitMsg,
		RepoDir:      repoDir,
		Mode:         mode,
		DiffRef:      diffRef,
		ManifestPath: manifestPath,
		Stderr:       stderr,
		Bypass:       bypass,
		ShipClass:    os.Getenv("SHIP_CLASS"),
	}
	_, err := commitprefixgate.Run(opts)
	if err == nil {
		return 0
	}
	if errors.Is(err, commitprefixgate.ErrScopeViolation) {
		return 2
	}
	if errors.Is(err, commitprefixgate.ErrBadArgs) {
		return 3
	}
	if errors.Is(err, commitprefixgate.ErrBadManifest) {
		return 4
	}
	fmt.Fprintf(stderr, "[commit-prefix-gate] FAIL: %v\n", err)
	return 1
}
