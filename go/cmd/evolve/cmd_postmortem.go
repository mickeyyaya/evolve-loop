package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	postmortemPrefix     = "evolve postmortem: "
	postmortemDefaultCLI = "claude-tmux"
	postmortemUsage      = `usage: evolve postmortem collect --cycle N --phase P --attempt K (--session UUID | --transcript PATH) --cause CODE [--exit-code E] [--cli C] [--started RFC3339 --ended RFC3339] [--project-root P]
       evolve postmortem show --cycle N --phase P [--project-root P]`
)

var claudeSessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type postmortemEnv struct {
	home        func() string
	resolveRoot func(projectRoot string, stderr io.Writer) (string, error)
}

type postmortemArgs struct {
	projectRoot, cycle, phase, attempt, session, transcript, cause, exitCode, cli, started, ended string
}

type postmortemTarget struct {
	root  string
	cycle int
	phase string
	cfg   attemptpostmortem.Config
}

func runPostmortem(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runPostmortemWith(defaultPostmortemEnv(), args, stdout, stderr)
}

func defaultPostmortemEnv() postmortemEnv {
	return postmortemEnv{home: func() string { return os.Getenv("HOME") }, resolveRoot: loopStopRoot}
}

func runPostmortemWith(env postmortemEnv, args []string, stdout, stderr io.Writer) int {
	handlers := map[string]func(postmortemEnv, []string, io.Writer, io.Writer) int{
		"collect": runPostmortemCollect, "show": runPostmortemShow,
	}
	if len(args) == 0 {
		return postmortemUsageError(stderr, errors.New("a sub-verb is required"))
	}
	run, ok := handlers[args[0]]
	if !ok {
		return postmortemUsageError(stderr, fmt.Errorf("unknown sub-verb %q", args[0]))
	}
	return run(env, args[1:], stdout, stderr)
}

func postmortemUsageError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", postmortemPrefix, err, postmortemUsage)
	return exitUsage
}

func postmortemFailure(stderr io.Writer, code int, err error) int {
	fmt.Fprintf(stderr, "%s%v\n", postmortemPrefix, err)
	return code
}

func parsePostmortemArgs(args []string, collect bool) (postmortemArgs, error) {
	var a postmortemArgs
	values := map[string]*string{"--project-root": &a.projectRoot, "--cycle": &a.cycle, "--phase": &a.phase}
	if collect {
		for name, into := range map[string]*string{
			"--attempt": &a.attempt, "--session": &a.session, "--transcript": &a.transcript, "--cause": &a.cause,
			"--exit-code": &a.exitCode, "--cli": &a.cli, "--started": &a.started, "--ended": &a.ended,
		} {
			values[name] = into
		}
	}
	operands, err := cliFlags{values: values}.parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected operand %q", operands[0])
	}
	return a, err
}

func (a postmortemArgs) target() (int, error) {
	cycle, err := positiveInt("--cycle", a.cycle)
	if err == nil && !attemptpostmortem.ValidPhase(a.phase) {
		err = fmt.Errorf("--phase %q is not a bare phase name", a.phase)
	}
	return cycle, err
}

func positiveInt(name, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s needs a positive integer, got %q", name, value)
	}
	return n, nil
}

func resolvePostmortemTarget(env postmortemEnv, a postmortemArgs, cycle int, stderr io.Writer) (postmortemTarget, error) {
	root, err := env.resolveRoot(a.projectRoot, stderr)
	if err != nil {
		return postmortemTarget{}, err
	}
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return postmortemTarget{}, err
	}
	return postmortemTarget{root: root, cycle: cycle, phase: a.phase, cfg: pol.AttemptPostmortemConfig()}, nil
}

func (t postmortemTarget) workspace() string { return paths.RunWorkspace(t.root, t.cycle) }

func runPostmortemShow(env postmortemEnv, args []string, stdout, stderr io.Writer) int {
	a, err := parsePostmortemArgs(args, false)
	cycle := 0
	if err == nil {
		cycle, err = a.target()
	}
	if err != nil {
		return postmortemUsageError(stderr, err)
	}
	t, err := resolvePostmortemTarget(env, a, cycle, stderr)
	if err != nil {
		return postmortemFailure(stderr, exitIO, err)
	}
	records, err := attemptpostmortem.ReadAll(t.workspace(), t.phase)
	if err != nil {
		return postmortemFailure(stderr, exitIO, err)
	}
	section := attemptpostmortem.Render(records, t.cfg)
	if section == "" {
		fmt.Fprintf(stderr, "%sno attempt postmortem of an abnormal end for phase %s in cycle %d\n", postmortemPrefix, t.phase, t.cycle)
		return 0
	}
	fmt.Fprint(stdout, section)
	return 0
}

type collectRequest struct {
	attempt        attemptpostmortem.Attempt
	started, ended time.Time
	hasWindow      bool
}

func (a postmortemArgs) collectRequest() (collectRequest, error) {
	cycle, err := a.target()
	if err != nil {
		return collectRequest{}, err
	}
	number, err := positiveInt("--attempt", a.attempt)
	if err != nil {
		return collectRequest{}, err
	}
	if err := a.checkSource(); err != nil {
		return collectRequest{}, err
	}
	exitCode, err := optionalInt("--exit-code", a.exitCode)
	if err != nil {
		return collectRequest{}, err
	}
	req := collectRequest{attempt: attemptpostmortem.Attempt{
		Phase: a.phase, Cycle: cycle, Number: number, CLI: orDefaultCLI(a.cli), CauseCode: a.cause, ExitCode: exitCode,
	}}
	req.started, req.ended, req.hasWindow, err = a.window()
	return req, err
}

func (a postmortemArgs) checkSource() error {
	switch {
	case a.cause == "":
		return errors.New("--cause is required")
	case (a.session == "") == (a.transcript == ""):
		return errors.New("give exactly one of --session and --transcript")
	case a.session != "" && !claudeSessionID.MatchString(a.session):
		return fmt.Errorf("--session %q is not a Claude session id", a.session)
	}
	return nil
}

func optionalInt(name, value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s needs an integer, got %q", name, value)
	}
	return n, nil
}

func orDefaultCLI(cli string) string {
	if cli == "" {
		return postmortemDefaultCLI
	}
	return cli
}

func (a postmortemArgs) window() (time.Time, time.Time, bool, error) {
	if a.started == "" && a.ended == "" {
		return time.Time{}, time.Time{}, false, nil
	}
	started, err := time.Parse(time.RFC3339Nano, a.started)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("--started and --ended need RFC 3339 times: %w", err)
	}
	ended, err := time.Parse(time.RFC3339Nano, a.ended)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("--started and --ended need RFC 3339 times: %w", err)
	}
	if ended.Before(started) {
		return time.Time{}, time.Time{}, false, errors.New("--ended is before --started")
	}
	return started, ended, true, nil
}

func runPostmortemCollect(env postmortemEnv, args []string, stdout, stderr io.Writer) int {
	a, err := parsePostmortemArgs(args, true)
	var req collectRequest
	if err == nil {
		req, err = a.collectRequest()
	}
	if err != nil {
		return postmortemUsageError(stderr, err)
	}
	t, err := resolvePostmortemTarget(env, a, req.attempt.Cycle, stderr)
	if err != nil {
		return postmortemFailure(stderr, exitIO, err)
	}
	path := a.transcript
	if a.session != "" {
		if path, err = findSessionTranscript(env.home(), a.session); err != nil {
			return postmortemFailure(stderr, exitRefused, err)
		}
	}
	trace, err := attemptpostmortem.ClaudeTranscript(path)()
	if err != nil {
		return postmortemFailure(stderr, exitIO, err)
	}
	rec, err := attemptpostmortem.Collect(attemptpostmortem.Input{
		Attempt:    req.withTrace(path, trace),
		Transcript: func() (attemptpostmortem.Trace, error) { return trace, nil },
	}, t.cfg)
	if err == nil {
		err = attemptpostmortem.Write(t.workspace(), rec)
	}
	if err != nil {
		return postmortemFailure(stderr, exitIO, err)
	}
	fmt.Fprintf(stdout, "%swrote %s\n", postmortemPrefix, attemptpostmortem.Path(t.workspace(), t.phase, rec.Number))
	return 0
}

func (r collectRequest) withTrace(path string, trace attemptpostmortem.Trace) attemptpostmortem.Attempt {
	a := r.attempt
	a.Session = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	a.StartedAt, a.EndedAt = r.started, r.ended
	if !r.hasWindow {
		a.StartedAt, a.EndedAt = trace.LastActivityAt, trace.LastActivityAt
		if len(trace.Commands) > 0 {
			a.StartedAt = trace.Commands[0].StartedAt
		}
	}
	return a
}

func findSessionTranscript(home, session string) (string, error) {
	projects := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projects)
	if err != nil {
		return "", fmt.Errorf("no transcript for session %s: %w", session, err)
	}
	for _, e := range entries {
		path := filepath.Join(projects, e.Name(), session+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no transcript for session %s under %s", session, projects)
}
