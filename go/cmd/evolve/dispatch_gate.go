package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	exitDispatchRefused = 3
	classRequest        = "R"
	classDenied         = "D"
	dispatchRequestPath = ".evolve/runs/cycle-<N>/requests/dispatch-<n>.json"
)

type gatedVerb struct{ name, class string }

type dispatchGate struct {
	open func() (dispatchcred.Store, error)
}

var defaultDispatchGate = dispatchGate{open: dispatchcred.Default}

func dispatchVerbOf(args []string) (gatedVerb, bool) {
	sub := ""
	if len(args) > 1 {
		sub = args[1]
	}
	switch args[0] {
	case "agent":
		return gatedVerb{"agent run", classRequest}, sub == "run"
	case "phase":
		lower := strings.ToLower(sub)
		return gatedVerb{"phase " + sub, classRequest}, sub != "" && lower != "verify" && lower != "lint"
	case "subagent":
		return gatedVerb{"subagent " + sub, classDenied}, sub == "run" || sub == "dispatch-parallel"
	case "loop":
		_, status := loopStatusArgs(args[1:])
		return gatedVerb{"loop", classDenied}, !status
	case "wave":
		return gatedVerb{"wave next", classDenied}, sub == "next"
	}
	return gatedVerb{}, false
}

func (g dispatchGate) refuse(args []string, stderr io.Writer) bool {
	verb, gated := dispatchVerbOf(args)
	if !gated {
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
	fmt.Fprintf(stderr, "evolve %s: refused: this process does not hold the pipeline dispatch credential (%v). A phase cell never holds it: %s\n", verb.name, v.Cause, verb.advice())
	return true
}

func (v gatedVerb) advice() string {
	if v.class == classRequest {
		return fmt.Sprintf("%s is class R; write a dispatch request to %s (schema evolve.request/1; item isolation-dispatch-request-channel builds the channel)", v.name, dispatchRequestPath)
	}
	return v.name + " is class D; it is an operator or boundary verb"
}

func emitDispatchViolation(verb gatedVerb, v dispatchcred.Verdict, stderr io.Writer) {
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
		Reason: fmt.Sprintf("evolve %s ran without the dispatch credential: %v", verb.name, v.Cause),
		Fields: map[string]string{
			"invariant": "I3", "stage": v.Stage, "action": action, "target": verb.name,
			"rule": "verb.class." + strings.ToLower(verb.class), "source": "dispatch-credential",
		},
	})
}

func dispatchWith(g dispatchGate, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if g.refuse(args, stderr) {
		return exitDispatchRefused
	}
	if cmd := lookupCommand(args[0]); cmd != nil {
		return cmd.Run(args[1:], stdin, stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve: unknown command %q\n\n%s", args[0], usage)
	return 2
}
