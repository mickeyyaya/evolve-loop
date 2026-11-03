package main

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

const (
	reviewCheckStage = "shadow"
	reviewCheckHead  = "HEAD"
)

type reviewCheckReport struct {
	Check    string                `json:"check"`
	Stage    string                `json:"stage"`
	Base     string                `json:"base,omitempty"`
	Refusals []reviewcheck.Refusal `json:"refusals"`
}

func parseCheckArgs(args []string, values map[string]*string) (asJSON, ok bool) {
	operands, err := cliFlags{bools: map[string]*bool{"--json": &asJSON}, values: values}.parse(args)
	return asJSON, err == nil && len(operands) == 0
}

func parseBaseCheckArgs(args []string) (base string, asJSON, ok bool) {
	asJSON, ok = parseCheckArgs(args, map[string]*string{"--base": &base})
	return base, asJSON, ok && base != ""
}

func printReviewCheck(verb string, asJSON bool, report reviewCheckReport, stdout, stderr io.Writer) int {
	report.Stage = reviewCheckStage
	if report.Refusals == nil {
		report.Refusals = []reviewcheck.Refusal{}
	}
	if asJSON {
		return printVerbJSON(verb, report, stdout, stderr)
	}
	for _, r := range report.Refusals {
		fmt.Fprintf(stdout, "%s: refuse %s: %s\n", verb, r.Subject, r.Reason)
	}
	fmt.Fprintf(stdout, "%s: %d refusal(s); %s stage, report only\n", verb, len(report.Refusals), reviewCheckStage)
	return cliExitOK
}

type baseCheck struct {
	verb  string
	check string
	usage string
	at    func(ctx context.Context, git gitexec.Git, r reviewcheck.Range) ([]reviewcheck.Refusal, error)
}

func (c baseCheck) run(root string, args []string, stdout, stderr io.Writer) int {
	base, asJSON, ok := parseBaseCheckArgs(args)
	if !ok {
		fmt.Fprintln(stderr, c.usage)
		return cliExitUsage
	}
	refusals, err := c.at(context.Background(), gitexec.Default(root), reviewcheck.Range{Base: base, Head: reviewCheckHead})
	if err != nil {
		fmt.Fprintf(stderr, "evolve %s: %v\n", c.verb, err)
		return cliExitIO
	}
	return printReviewCheck(c.verb, asJSON, reviewCheckReport{Check: c.check, Base: base, Refusals: refusals}, stdout, stderr)
}

var (
	inboxIntegrityCheck = baseCheck{
		verb:  "inbox integrity",
		check: reviewcheck.CheckInboxIntegrity,
		usage: "usage: evolve inbox integrity --base B [--json] [--project-root P]",
		at:    reviewcheck.InboxIntegrityAt,
	}
	permissionsTraceCheck = baseCheck{
		verb:  "permissions trace",
		check: reviewcheck.CheckPermissionWidening,
		usage: "usage: evolve permissions trace --base B [--json] [--project-root P]",
		at:    reviewcheck.PermissionWideningAt,
	}
)

func runInboxIntegrity(root string, args []string, stdout, stderr io.Writer) int {
	return inboxIntegrityCheck.run(root, args, stdout, stderr)
}

type singleVerb struct {
	name  string
	usage string
	run   func(root string, args []string, stdout, stderr io.Writer) int
}

func (v singleVerb) dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != v.name {
		fmt.Fprintln(stderr, v.usage)
		return cliExitUsage
	}
	root, rest, ok := takeProjectRoot(args[1:])
	if !ok {
		fmt.Fprintln(stderr, v.usage)
		return cliExitUsage
	}
	return v.run(root, rest, stdout, stderr)
}

const personaStripUsage = "usage: evolve persona strip-check [--json] [--project-root P]"

func runPersona(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return singleVerb{name: "strip-check", usage: personaStripUsage, run: runPersonaStripCheck}.dispatch(args, stdout, stderr)
}

func runPermissions(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return singleVerb{name: "trace", usage: permissionsTraceCheck.usage, run: permissionsTraceCheck.run}.dispatch(args, stdout, stderr)
}

func runPersonaStripCheck(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, ok := parseCheckArgs(args, nil)
	if !ok {
		fmt.Fprintln(stderr, personaStripUsage)
		return cliExitUsage
	}
	personas, err := loadPersonas(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve persona strip-check: %v\n", err)
		return cliExitIO
	}
	report := reviewCheckReport{Check: reviewcheck.CheckPersonaStrip, Refusals: reviewcheck.PersonaStrip(personas)}
	return printReviewCheck("persona strip-check", asJSON, report, stdout, stderr)
}

func loadPersonas(root string) ([]reviewcheck.Persona, error) {
	loader := prompts.NewForProject(root)
	names, err := loader.Agents()
	if err != nil {
		return nil, err
	}
	personas := make([]reviewcheck.Persona, 0, len(names))
	for _, name := range names {
		p, err := loader.Agent(name)
		if err != nil {
			return nil, err
		}
		personas = append(personas, reviewcheck.Persona{Path: "agents/" + name + ".md", Text: p.Raw})
	}
	return personas, nil
}
