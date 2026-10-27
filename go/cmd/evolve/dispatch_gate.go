package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const exitDispatchRefused = cliExitRefused

type dispatchGate struct {
	open func() (dispatchcred.Store, error)
}

var defaultDispatchGate = dispatchGate{open: dispatchcred.Default}

func agentClassOf(args []string) agentclass.Match {
	if args[0] == "loop" {
		if _, status := loopStatusArgs(args[1:]); status {
			return agentclass.Of([]string{"loop", "status"})
		}
	}
	return agentclass.Of(args)
}

func (g dispatchGate) refuse(args []string, stderr io.Writer) bool {
	verb := agentClassOf(args)
	if verb.Class == agentclass.Allowed {
		return false
	}
	v := dispatchcred.Admit(g.open)
	if !v.Violation() {
		return false
	}
	emitDispatchViolation(verb, v, stderr)
	if !v.Refused() {
		return false
	}
	fmt.Fprintf(stderr, "evolve %s: %s. This process does not hold the pipeline dispatch credential (%v), and a phase cell never holds it.\n", verb.Name, verb.Refusal(""), v.Cause)
	return true
}

func emitDispatchViolation(verb agentclass.Match, v dispatchcred.Verdict, stderr io.Writer) {
	action := "logged"
	if v.Refused() {
		action = "refused"
	}
	layout := paths.ResolveFromEnv()
	signals, closeSignals := newRoleSignalCenter(roleDispatch, layout.ProjectRoot, layout.EvolveDir, stderr)
	defer closeSignals()
	signals.Emit(signalcenter.Event{
		Phase: os.Getenv(ipcenv.PhaseKey), Module: signalcenter.ModuleIsolation, Origin: "evolve.dispatchGate",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(3),
		Reason: fmt.Sprintf("evolve %s ran without the dispatch credential: %v", verb.Name, v.Cause),
		Fields: map[string]string{
			"invariant": "I3", "stage": v.Stage, "action": action, "target": verb.Name,
			"rule": "verb.class." + strings.ToLower(verb.Class), "source": "dispatch-credential",
		},
	})
}

func dispatchWith(g dispatchGate, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd := lookupCommand(args[0])
	if cmd == nil {
		fmt.Fprintf(stderr, "evolve: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	run := func(stdout, stderr io.Writer) int {
		if g.refuse(append([]string{cmd.Name}, args[1:]...), stderr) {
			return exitDispatchRefused
		}
		return cmd.Run(args[1:], stdin, stdout, stderr)
	}
	if jsonRequested(args[1:]) {
		if v, ok := coreVerbOf(cmd.Name, args[1:]); ok {
			return runEnveloped(v, run, stdout, stderr)
		}
	}
	return run(stdout, stderr)
}
