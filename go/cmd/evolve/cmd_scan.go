package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/secretleakscan"
)

const (
	scanUsage         = "usage: evolve scan secrets [--staged | --diff <ref>] [--project-root P]"
	scanSecretsPrefix = "evolve scan secrets: "
	exitSecretFound   = 1
	maskKeepRunes     = 4
)

type scanSecretsArgs struct {
	staged, help bool
	diffRefs     []string
	projectRoot  string
}

func runScan(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, scanUsage)
		return 0
	}
	if len(args) == 0 || args[0] != "secrets" {
		fmt.Fprintf(stderr, "evolve scan: want the subcommand secrets\n%s\n", scanUsage)
		return exitUsage
	}
	a, err := parseScanSecretsArgs(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", scanSecretsPrefix, err, scanUsage)
		return exitUsage
	}
	if a.help {
		fmt.Fprintln(stdout, scanUsage)
		return 0
	}
	return scanSecrets(a, stdout, stderr)
}

func parseScanSecretsArgs(args []string) (scanSecretsArgs, error) {
	var a scanSecretsArgs
	operands, err := cliFlags{
		bools:  map[string]*bool{"--staged": &a.staged, "--help": &a.help, "-h": &a.help},
		values: map[string]*string{"--project-root": &a.projectRoot},
		lists:  map[string]*[]string{"--diff": &a.diffRefs},
	}.parse(args)
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected operand %q", operands[0])
	case len(a.diffRefs) > 1:
		return a, errors.New("--diff given more than once")
	case len(a.diffRefs) == 1 && a.staged:
		return a, errors.New("--staged and --diff are mutually exclusive")
	case len(a.diffRefs) == 1 && (a.diffRefs[0] == "" || strings.HasPrefix(a.diffRefs[0], "-")):
		return a, fmt.Errorf("--diff needs a revision, got %q", a.diffRefs[0])
	}
	return a, nil
}

func scanSecrets(a scanSecretsArgs, stdout, stderr io.Writer) int {
	diff, err := scanGitDiff(a)
	if err != nil {
		fmt.Fprintf(stderr, "%sgit diff: %v\n", scanSecretsPrefix, err)
		return exitIO
	}
	findings := secretleakscan.ScanDiff(diff)
	for _, f := range findings {
		fmt.Fprintf(stdout, "%s:%d: %s: %s\n", f.File, f.Line, f.Rule, maskSecret(f.Match))
	}
	verdict := secretleakscan.Verdict(findings)
	fmt.Fprintf(stdout, "scan secrets: %s: %d finding(s) in %s\n", verdict, len(findings), scanSource(a))
	if verdict == "FAIL" {
		return exitSecretFound
	}
	return 0
}

func scanGitDiff(a scanSecretsArgs) (string, error) {
	args := []string{"-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/"}
	if len(a.diffRefs) == 1 {
		args = append(args, a.diffRefs[0], "--")
	} else {
		args = append(args, "--cached")
	}
	out, errOut, code, err := gitexec.Default(a.projectRoot).Capture(context.Background(), args...)
	switch {
	case err != nil:
		return "", err
	case code != 0:
		return "", fmt.Errorf("exit %d: %s", code, strings.TrimSpace(errOut))
	}
	return out, nil
}

func scanSource(a scanSecretsArgs) string {
	if len(a.diffRefs) == 1 {
		return "diff " + a.diffRefs[0]
	}
	return "the staged diff"
}

func maskSecret(match string) string {
	runes := []rune(match)
	if len(runes) <= maskKeepRunes {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:maskKeepRunes]) + strings.Repeat("*", len(runes)-maskKeepRunes)
}
