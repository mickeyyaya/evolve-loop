package cloudcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	exitFailed = 1
	exitUsage  = 10
)

const usage = `Usage: evolve cloud <subcommand>

  evolve cloud list [--json] [--project-root P]
      Print each cloud session that a cloud seat launched: the session id, the URL, the title and the time.
      It reads the launch records under .evolve/agent-runs and starts nothing.
      A cloud seat starts only through the bridge: evolve agent run <profile> on a profile or route that names claude-cloud.
      Exit 0 listed, 1 a launch record that does not read, 10 usage.
`

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "list":
		return list(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "evolve cloud: unknown subcommand %q\n\n%s", args[0], usage)
	return exitUsage
}

func list(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cloud list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print one JSON array of the sessions")
	root := fs.String("project-root", "", "project root (default EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve cloud list: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	sessions, err := cloudSessions(projectRoot(*root))
	if err != nil {
		fmt.Fprintf(stderr, "evolve cloud list: %v\n", err)
		return exitFailed
	}
	if *asJSON {
		buf, _ := json.MarshalIndent(sessions, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		return 0
	}
	for _, s := range sessions {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", s.ID, s.URL, s.Title, s.TS)
	}
	return 0
}

func projectRoot(flagValue string) string {
	if flagValue == "" {
		flagValue = cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	return paths.AbsoluteRoot("--project-root", flagValue, nil)
}

func cloudSessions(root string) ([]cloudlaunch.Session, error) {
	records, err := launchrecord.List(paths.EvolveDirOf(root))
	if err != nil {
		return nil, err
	}
	sessions := []cloudlaunch.Session{}
	for _, r := range records {
		if r.Cloud != nil {
			sessions = append(sessions, *r.Cloud)
		}
	}
	return sessions, nil
}
