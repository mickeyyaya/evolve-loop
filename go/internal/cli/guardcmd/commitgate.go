package guardcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func RunCommitGate(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "evolve commit-gate: usage: commit-gate run --reviewers \"<csv>\" [--files <%s>] [--no-install] [--project-root P]\n", commitgate.FilesFormat)
		return commitgate.ExitBadArgs
	}
	sub, rest := args[0], args[1:]
	if sub != "run" {
		fmt.Fprintf(stderr, "evolve commit-gate: unknown subcommand %q (want: run)\n", sub)
		return commitgate.ExitBadArgs
	}

	fs := flag.NewFlagSet("commit-gate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		reviewers   = fs.String("reviewers", "", "CSV of reviewers that ran (simplify + one review capability)")
		files       = fs.String("files", "", "override change detection: "+commitgate.FilesFormat+" from the change")
		noInstall   = fs.Bool("no-install", false, "do not auto-install missing tools (hard-fail instead)")
		projectRoot = fs.String("project-root", "", "git working tree root (default: git rev-parse --show-toplevel)")
	)
	if err := fs.Parse(rest); err != nil {
		return commitgate.ExitBadArgs
	}

	opts, ok := withReviewStore(commitgate.Options{
		RepoRoot:  *projectRoot,
		Reviewers: *reviewers,
		Files:     *files,
		NoInstall: *noInstall,
		AttestDir: os.Getenv("CG_ATTEST_DIR"),
		Env:       os.Environ(),
		Runner:    sysexec.DefaultRunner,
		Now:       time.Now,
	}, stderr)
	if !ok {
		return commitgate.ExitGitFatal
	}

	res := opts.Run(context.Background())
	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}
	if res.ExitCode == commitgate.ExitPass && res.Attestation != nil {
		fmt.Fprintf(stdout, "%s\n", res.Attestation.TreeStateSHA)
	}
	return res.ExitCode
}

func withReviewStore(opts commitgate.Options, stderr io.Writer) (commitgate.Options, bool) {
	root, ok := gateRoot(opts.RepoRoot, opts.Runner, stderr)
	if !ok {
		return commitgate.Options{}, false
	}
	reviewDir, err := commitgate.ReviewStoreDir(context.Background(), opts.Runner, root)
	if err != nil {
		fmt.Fprintf(stderr, "[commit-gate] cannot resolve the review store: %v\n", err)
		return commitgate.Options{}, false
	}
	opts.RepoRoot, opts.ReviewDir = root, reviewDir
	return opts, true
}

func gateRoot(flagRoot string, runner commitgate.Runner, stderr io.Writer) (string, bool) {
	if flagRoot != "" {
		return flagRoot, true
	}
	out, err := sysexec.Output(context.Background(), runner, "", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(stderr, "[commit-gate] not in a git repo")
	}
	return out, err == nil
}
