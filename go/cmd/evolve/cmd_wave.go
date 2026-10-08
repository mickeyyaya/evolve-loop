package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	waveNextSchema      = "evolve.wave.next/1"
	wavePrefix          = "evolve wave: "
	waveBuildVerb       = "wave-build"
	waveWatchPoll       = 5 * time.Second
	waveStatusMaxCycles = 20
	psStartLayout       = "Mon Jan 2 15:04:05 2006"
	waveUsage           = `usage: evolve wave next [--max-cycles N] [--merge n,...] [--note TEXT]... [--number N] [--dry-run] [--json] [--project-root P]
       evolve wave status [--json] [--project-root P]
       evolve wave watch [--json] [--project-root P]
       evolve wave note add TEXT | list [--json] | clear [--project-root P]
one cycle in the foreground: evolve cycle run --goal-text G; resume a paused cycle: evolve loop --resume; stop at the next wave boundary: evolve loop-stop --wait`
)

var mergedPRSubject = regexp.MustCompile(`^Merge pull request #(\d+)`)

type waveEnv struct {
	resolveRoot func(projectRoot string, stderr io.Writer) (string, error)
	executable  func() (string, error)
	pidStarted  func(int) (time.Time, bool)
	dispatch    boundaryDispatch
	now         func() time.Time
	sleep       func(time.Duration)
	pidAlive    func(int) bool
	mainHead    func(root string) (string, error)
	mergedPRs   func(root, since string) ([]string, error)
}

type wavePlaneDirs struct {
	root      string
	evolveDir string
	store     wave.Store
}

func runWave(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runWaveWith(defaultWaveEnv(), args, stdout, stderr)
}

func defaultWaveEnv() waveEnv {
	return waveEnv{
		resolveRoot: loopStopRoot, executable: os.Executable, pidStarted: processStartTime,
		dispatch: dispatchWaveVerb(sysexec.DefaultRunner), now: time.Now, sleep: time.Sleep,
		pidAlive: runlease.PIDAlive, mainHead: gitMainHead, mergedPRs: gitMergedPRs,
	}
}

func runWaveWith(env waveEnv, args []string, stdout, stderr io.Writer) int {
	handlers := map[string]func(waveEnv, []string, io.Writer, io.Writer) int{
		"next": runWaveNext, "status": runWaveStatus, "watch": runWaveWatch, "note": runWaveNote,
	}
	if len(args) == 0 {
		return waveUsageError(stderr, errors.New("a sub-verb is required"))
	}
	run, ok := handlers[args[0]]
	if !ok {
		return waveUsageError(stderr, fmt.Errorf("unknown sub-verb %q", args[0]))
	}
	return run(env, args[1:], stdout, stderr)
}

func waveUsageError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", wavePrefix, err, waveUsage)
	return exitUsage
}

func resolveWavePlane(env waveEnv, projectRoot string, stderr io.Writer) (wavePlaneDirs, error) {
	root, err := env.resolveRoot(projectRoot, stderr)
	if err != nil {
		return wavePlaneDirs{}, err
	}
	evolveDir := paths.EvolveDirOf(root)
	return wavePlaneDirs{root: root, evolveDir: evolveDir, store: wave.NewStore(evolveDir)}, nil
}

func waveLiveRuns(evolveDir string, now time.Time) []runlease.LiveRun {
	return runlease.LiveRuns(filepath.Join(evolveDir, "runs"), now)
}

func waveLoopLive(env waveEnv, evolveDir string, rec wave.Record) bool {
	return len(waveLiveRuns(evolveDir, env.now())) > 0 || waveOwnsPID(env, rec)
}

func waveOwnsPID(env waveEnv, rec wave.Record) bool {
	if rec.PID <= 0 || !env.pidAlive(rec.PID) {
		return false
	}
	if writer, err := readWriterPID(rec.LogPath); err != nil || writer != rec.PID {
		return false
	}
	started, known := env.pidStarted(rec.PID)
	return !known || !started.After(rec.StartedAt)
}

func processStartTime(pid int) (time.Time, bool) {
	var out strings.Builder
	env := append(os.Environ(), "LC_ALL=C")
	rc, err := sysexec.DefaultRunner(context.Background(), "ps", "", []string{"-o", "lstart=", "-p", strconv.Itoa(pid)}, env, nil, &out, io.Discard)
	if err != nil || rc != 0 {
		return time.Time{}, false
	}
	at, err := time.ParseInLocation(psStartLayout, strings.Join(strings.Fields(out.String()), " "), time.Local)
	return at, err == nil
}

func runWaveNote(env waveEnv, args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	var asJSON bool
	operands, err := cliFlags{bools: map[string]*bool{"--json": &asJSON}, values: map[string]*string{"--project-root": &projectRoot}}.parse(args)
	if err == nil && len(operands) == 0 {
		err = errors.New("note needs add, list or clear")
	}
	if err != nil {
		return waveUsageError(stderr, err)
	}
	plane, err := resolveWavePlane(env, projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	switch operands[0] {
	case "add":
		return addWaveNote(env, plane.store, strings.Join(operands[1:], " "), stdout, stderr)
	case "list":
		return listWaveNotes(plane.store, asJSON, stdout, stderr)
	case "clear":
		return clearWaveNotes(plane.store, stdout, stderr)
	default:
		return waveUsageError(stderr, fmt.Errorf("unknown note sub-verb %q", operands[0]))
	}
}

func addWaveNote(env waveEnv, store wave.Store, text string, stdout, stderr io.Writer) int {
	if strings.TrimSpace(text) == "" {
		return waveUsageError(stderr, errors.New("note add needs a text"))
	}
	n, err := store.AddNote(text, env.now())
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	fmt.Fprintf(stdout, "wave: queued note %s for the next wave\n", n.Name)
	return 0
}

func listWaveNotes(store wave.Store, asJSON bool, stdout, stderr io.Writer) int {
	notes, err := store.Notes()
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	if asJSON {
		return writeWaveJSON(append([]wave.Note{}, notes...), stdout, stderr)
	}
	for _, n := range notes {
		fmt.Fprintf(stdout, "%s: %s\n", n.Name, n.Text)
	}
	return 0
}

func clearWaveNotes(store wave.Store, stdout, stderr io.Writer) int {
	notes, err := store.Notes()
	if err == nil {
		err = store.RemoveNotes(notes)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	fmt.Fprintf(stdout, "wave: removed %d note(s)\n", len(notes))
	return 0
}

func dispatchWaveVerb(run sysexec.RunFunc) boundaryDispatch {
	handlers := map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
		"checkpoint": runCheckpoint, "reset-sha": runResetSHA, "doctor": opscmd.RunDoctor,
		waveBuildVerb: func(args []string, _ io.Reader, stdout, stderr io.Writer) int {
			return runWaveBuild(run, args, stdout, stderr)
		},
	}
	return func(verb string, args []string, stdout, stderr io.Writer) int {
		if h, ok := handlers[verb]; ok {
			return h(args, strings.NewReader(""), stdout, stderr)
		}
		return dispatchBoundaryVerb(verb, args, stdout, stderr)
	}
}

func runWaveBuild(run sysexec.RunFunc, args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	if _, err := (cliFlags{values: map[string]*string{"--project-root": &projectRoot}}).parse(args); err != nil || projectRoot == "" {
		fmt.Fprintf(stderr, "%s%s needs --project-root\n", wavePrefix, waveBuildVerb)
		return exitUsage
	}
	rc, err := run(context.Background(), "make", "", []string{"-C", filepath.Join(projectRoot, "go"), "build"}, nil, nil, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%sbuild: %v\n", wavePrefix, err)
		return exitIO
	}
	return rc
}

func gitMainHead(root string) (string, error) {
	return gitexec.Default(root).HEAD(context.Background())
}

func gitMergedPRs(root, since string) ([]string, error) {
	out, err := gitexec.Default(root).Output(context.Background(), "log", "--first-parent", "--merges", "--format=%s", since+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("list the merges since %s: %w", since, err)
	}
	return parseMergedPRs(out), nil
}

func parseMergedPRs(log string) []string {
	var prs []string
	for _, line := range strings.Split(log, "\n") {
		if m := mergedPRSubject.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			prs = append(prs, m[1])
		}
	}
	return prs
}
