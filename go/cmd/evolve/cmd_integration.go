package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	integrationUsage      = "usage: evolve integration watch --tip SHA\n       evolve integration status [--pool N] [--wave N] [--project-root P] [--json]\n       evolve integration record-pr --pr N (--reviewed | --class NAME)\n       evolve integration batch [--dry-run] [PR...]\n  watch: exit 0 = CI green, 1 = CI red, 2 = no run to watch or gh failed, 10 = usage\n  status: exit 0 = clean, 1 = not clean, no evidence or an unreadable record; 2 = an absent root, an unreadable wave store, or a wave number that the store does not know; 10 = usage\n  record-pr and batch: exit 0 = done, 1 = refused or a PR was ejected, 2 = git or gh failed, 10 = usage"
	integrationWatchUsage = "usage: evolve integration watch --tip SHA"
	integrationPrefix     = "evolve integration: "
)

var integrationTip = regexp.MustCompile(`^[0-9a-f]{40}$`)

type integrationEnv struct {
	Gh             integration.GhFunc
	Git            integration.GitFunc
	Root           string
	ProjectRootEnv string
	Wave           string

	Settle func(ctx context.Context) error
}

type integrationVerb func(env integrationEnv, args []string, stdout, stderr io.Writer) int

var integrationVerbs = map[string]integrationVerb{
	"watch":     watchVerb,
	"batch":     batchVerb,
	"record-pr": recordPRVerb,
	"status":    statusVerb,
}

func chooseRoot(flag, envValue, fallback string) string {
	return cmp.Or(flag, envValue, fallback)
}

const registrationWait = 5 * time.Second

func runIntegration(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runIntegrationAt(os.Getwd, args, stdout, stderr)
}

func runIntegrationAt(getwd func() (string, error), args []string, stdout, stderr io.Writer) int {
	root, err := getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%sread the working directory: %v\n", integrationPrefix, err)
		return exitIO
	}
	return runIntegrationEnv(productionIntegrationEnv(root, time.Now()), args, stdout, stderr)
}

func productionIntegrationEnv(root string, now time.Time) integrationEnv {
	return integrationEnv{
		Gh:             integration.SysGh(),
		Git:            integration.SysGit(),
		Root:           root,
		ProjectRootEnv: os.Getenv("EVOLVE_PROJECT_ROOT"),
		Wave:           "pr" + now.UTC().Format("20060102t150405"),
		Settle:         pauseFor(registrationWait),
	}
}

func pauseFor(d time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			return nil
		}
	}
}

func runIntegrationEnv(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || integrationVerbs[args[0]] == nil {
		fmt.Fprintf(stderr, "%sname the sub-verb watch, status, record-pr or batch\n%s\n", integrationPrefix, integrationUsage)
		return exitUsage
	}
	return integrationVerbs[args[0]](env, args[1:], stdout, stderr)
}

func watchVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	gh := env.Gh
	var tip string
	var help bool
	operands, err := cliFlags{bools: map[string]*bool{"--help": &help}, values: map[string]*string{"--tip": &tip}}.parse(args)
	if help && err == nil {
		fmt.Fprintln(stdout, integrationWatchUsage)
		return 0
	}
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected operand %q", operands[0])
	}
	if err == nil && !integrationTip.MatchString(tip) {
		err = fmt.Errorf("--tip %q must be a 40-character commit id", tip)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", integrationPrefix, err, integrationUsage)
		return exitUsage
	}
	return watchIntegrationTip(gh, env.Settle, tip, stdout, stderr)
}

func watchIntegrationTip(gh integration.GhFunc, settle func(context.Context) error, tip string, stdout, stderr io.Writer) int {
	verdict, err := integration.GhCI{Gh: gh, Settle: settle}.Watch(context.Background(), integration.PR{Tip: tip})
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", integrationPrefix, err)
		return exitIO
	}
	fmt.Fprintln(stdout, verdict)
	if verdict == integration.Red {
		return exitRefused
	}
	return 0
}
