package main

import (
	"fmt"
	"io"
	"os"
	"slices"
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

type dispatchRule struct {
	class   string
	withSub bool
	gates   func(sub string, rest []string) bool
}

func anySub(string, []string) bool { return true }

func onlySubs(subs ...string) func(string, []string) bool {
	return func(sub string, _ []string) bool { return slices.Contains(subs, sub) }
}

func phaseDispatch(sub string, _ []string) bool {
	lower := strings.ToLower(sub)
	return sub != "" && lower != "verify" && lower != "lint"
}

func loopDispatch(_ string, rest []string) bool {
	_, status := loopStatusArgs(rest)
	return !status
}

func campaignDispatch(sub string, _ []string) bool { return sub != "status" }

var dispatchRules = map[string]dispatchRule{
	"agent":              {classRequest, true, onlySubs("run")},
	"phase":              {classRequest, true, phaseDispatch},
	"subagent":           {classDenied, true, onlySubs("run", "dispatch-parallel")},
	"loop":               {classDenied, false, loopDispatch},
	"wave":               {classDenied, true, onlySubs("next")},
	"cycle":              {classDenied, true, onlySubs("run")},
	"boundary":           {classDenied, true, onlySubs("run")},
	"bridge":             {classDenied, true, onlySubs("launch")},
	"routing":            {classDenied, true, onlySubs("plan")},
	"campaign":           {classDenied, false, campaignDispatch},
	"compose":            {classDenied, false, anySub},
	"plan-and-execute":   {classDenied, false, anySub},
	"serve-phase":        {classDenied, false, anySub},
	"fleet":              {classDenied, false, anySub},
	"consensus-dispatch": {classDenied, false, anySub},
	"fanout-dispatch":    {classDenied, false, anySub},
}

func dispatchVerbOf(args []string) (gatedVerb, bool) {
	rule, ok := dispatchRules[args[0]]
	if !ok {
		return gatedVerb{}, false
	}
	sub := ""
	if len(args) > 1 {
		sub = args[1]
	}
	if !rule.gates(sub, args[1:]) {
		return gatedVerb{}, false
	}
	name := args[0]
	if rule.withSub {
		name += " " + sub
	}
	return gatedVerb{name, rule.class}, true
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
