package agentcmd

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (c command) capabilities(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent capabilities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print one evolve.agent.capabilities/1 document")
	root := fs.String("project-root", "", "project root; it gives the caps of the request channel (the class table is the same for each root)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve agent capabilities: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	surface := agentclass.Surface(nil, c.deps.VerbSchema)
	pol := policy.LoadOrDefaults(filepath.Join(paths.EvolveDirOf(projectRoot(*root)), paths.PolicyFile), stderr)
	surface.Caps[agentclass.KindDispatch] = pol.IsolationDispatchRequests()
	if *asJSON {
		printJSON(stdout, surface)
		return 0
	}
	for _, v := range surface.Verbs {
		if v.Request != nil {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", v.Verb, v.Class, v.Request.Kind)
			continue
		}
		fmt.Fprintf(stdout, "%s\t%s\n", v.Verb, v.Class)
	}
	fmt.Fprintf(stdout, "default\t%s\n", surface.DefaultClass)
	return 0
}
