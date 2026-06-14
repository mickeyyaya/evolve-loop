// Package fanoutdispatch ports legacy/scripts/dispatch/fanout-dispatch.sh.
//
// Bounded-concurrency parallel worker dispatcher (Sprint 1).
// Reads a TSV commands file (`<worker_name>\t<command>\n`) and runs each
// command concurrently, bounded by EVOLVE_FANOUT_CONCURRENCY (default 2).
// Each worker is wrapped in a per-worker timeout (default 600s). WAIT-ALL
// semantics: every worker runs to completion or timeout regardless of
// others' failures.
//
// Optional consensus-cancel (EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1) polls
// workers' stdout for FAIL verdicts and SIGTERMs survivors once K workers
// have voted FAIL.
package fanoutdispatch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Exit codes:
//
//	0 — every worker exited 0
//	1 — one or more workers had non-zero exit codes
//	2 — bad arguments / setup failure
const (
	ExitOK         = 0
	ExitWorkerFail = 1
	ExitSetupErr   = 2
)

// Config controls the dispatcher behavior.
type Config struct {
	CommandsFile        string // path to TSV (<name>\t<command>\n)
	ResultsFile         string // path to write merged results TSV
	Concurrency         int    // default 2 ≥1
	TimeoutSecs         int    // default 600 ≥1
	CancelOnConsensus   bool   // EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1
	ConsensusK          int    // ≥1; how many FAILs trigger cancel
	ConsensusPollSecs   int    // default 1
	PerWorkerBudgetUSD  string // default "0.20"; injected as EVOLVE_MAX_BUDGET_USD unless set
	CachePrefixFile     string // optional; exported as EVOLVE_FANOUT_CACHE_PREFIX_FILE
	CycleStateHelperBin string // optional; if set + TrackWorkers, shell out for status writes
	TrackWorkers        bool   // EVOLVE_FANOUT_TRACK_WORKERS=1
	// Testing seams:
	Now func() time.Time
}

// Run executes the dispatcher. stderr receives [fanout-dispatch] lines.
func Run(cfg Config, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[fanout-dispatch] "+format+"\n", args...)
	}

	if cfg.CommandsFile == "" || cfg.ResultsFile == "" {
		logf("usage: fanout-dispatch [--cache-prefix-file=PATH] <commands.tsv> <results.tsv>")
		return ExitSetupErr
	}
	if cfg.CachePrefixFile != "" {
		if _, err := os.Stat(cfg.CachePrefixFile); err != nil {
			logf("cache-prefix-file not found: %s", cfg.CachePrefixFile)
			return ExitSetupErr
		}
	}
	if _, err := os.Stat(cfg.CommandsFile); err != nil {
		logf("commands file not found: %s", cfg.CommandsFile)
		return ExitSetupErr
	}

	// defaults
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 2
	}
	if cfg.TimeoutSecs < 1 {
		cfg.TimeoutSecs = 600
	}
	if cfg.ConsensusK < 1 {
		cfg.ConsensusK = 2
	}
	if cfg.ConsensusPollSecs < 1 {
		cfg.ConsensusPollSecs = 1
	}
	if cfg.PerWorkerBudgetUSD == "" {
		cfg.PerWorkerBudgetUSD = "0.20"
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	resultsDir := filepath.Dir(cfg.ResultsFile)
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		logf("mkdir results dir: %v", err)
		return ExitSetupErr
	}

	// Read commands.
	commands, err := ReadCommands(cfg.CommandsFile)
	if err != nil {
		logf("read commands: %v", err)
		return ExitSetupErr
	}
	if len(commands) == 0 {
		// empty input → empty output, exit 0
		_ = os.WriteFile(cfg.ResultsFile, []byte{}, 0o644)
		return ExitOK
	}

	// Per-worker budget + cache-prefix are injected into each worker's OWN
	// env in runWorker (cfg.workerEnv), NOT via process-global os.Setenv:
	// the process environment is a single MT-Unsafe global, so two concurrent
	// DispatchParallel in one process would clobber each other (ADR-0049 S1 /
	// gap G8). See workerEnv for the "inject-unless-inherited" semantics.

	// Per-worker state.
	type result struct {
		name     string
		exitCode int
		duration int64
	}
	results := make(map[string]result, len(commands))
	resultsMu := sync.Mutex{}

	// Shared cancel signal for consensus-cancel.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Semaphore for bounded concurrency.
	sema := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, c := range commands {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			sema <- struct{}{}
			defer func() { <-sema }()

			cfg.setWorkerStatus(c.Name, "running", 0)
			start := cfg.Now()
			rc := cfg.runWorker(rootCtx, c.Name, c.Command, resultsDir)
			dur := int64(cfg.Now().Sub(start).Seconds())

			term := "done"
			if rc != 0 {
				term = "failed"
			}
			cfg.setWorkerStatus(c.Name, term, rc)

			// write meta file
			metaPath := filepath.Join(resultsDir, c.Name+".meta")
			_ = os.WriteFile(metaPath,
				[]byte(fmt.Sprintf("%s\t%d\t%d\n", c.Name, rc, dur)),
				0o644)

			resultsMu.Lock()
			results[c.Name] = result{name: c.Name, exitCode: rc, duration: dur}
			resultsMu.Unlock()
		}()
	}

	// Optional consensus-cancel polling.
	if cfg.CancelOnConsensus {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.ConsensusPollSecs) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-rootCtx.Done():
					return
				case <-ticker.C:
					if checkFailConsensus(resultsDir, cfg.ConsensusK) {
						logf("consensus reached: %d workers FAIL — SIGTERM remaining", cfg.ConsensusK)
						rootCancel()
						return
					}
					// also exit poll if all are done
					resultsMu.Lock()
					done := len(results) >= len(commands)
					resultsMu.Unlock()
					if done {
						return
					}
				}
			}
		}()
	}

	wg.Wait()

	// Merge meta files into RESULTS_FILE in input order.
	anyFail := false
	var b strings.Builder
	for _, c := range commands {
		meta := filepath.Join(resultsDir, c.Name+".meta")
		if data, err := os.ReadFile(meta); err == nil {
			b.Write(data)
			r := results[c.Name]
			if r.exitCode != 0 {
				anyFail = true
			}
		} else {
			// missing meta = SIGTERM'd worker; sentinel row
			fmt.Fprintf(&b, "%s\t-1\t0\n", c.Name)
			anyFail = true
		}
	}
	if err := os.WriteFile(cfg.ResultsFile, []byte(b.String()), 0o644); err != nil {
		logf("write results: %v", err)
		return ExitSetupErr
	}

	if anyFail {
		return ExitWorkerFail
	}
	return ExitOK
}

// runWorker executes a single worker command with timeout. Stdout → name.out,
// stderr → name.err. Returns the exit code (124 for timeout, 143 for SIGTERM).
// workerEnv builds the explicit environment for a fan-out worker subprocess —
// the parent env plus per-worker budget/cache-prefix. Injecting per-child
// (exec.Cmd.Env) rather than mutating the process-global env (os.Setenv) keeps
// two concurrent DispatchParallel calls in one process from clobbering each
// other (ADR-0049 S1 / gap G8). Budget is added only when not already
// inherited (preserving the prior "unless set" semantics); cache-prefix is
// appended when configured (a later duplicate key wins in exec, as before).
func (cfg Config) workerEnv() []string {
	env := os.Environ()
	if os.Getenv("EVOLVE_MAX_BUDGET_USD") == "" {
		env = append(env, "EVOLVE_MAX_BUDGET_USD="+cfg.PerWorkerBudgetUSD)
	}
	if cfg.CachePrefixFile != "" {
		env = append(env, "EVOLVE_FANOUT_CACHE_PREFIX_FILE="+cfg.CachePrefixFile)
	}
	return env
}

func (cfg Config) runWorker(parent context.Context, name, command, resultsDir string) int {
	ctx, cancel := context.WithTimeout(parent, time.Duration(cfg.TimeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Env = cfg.workerEnv() // per-worker env snapshot (ADR-0049 S1 / G8)
	outF, _ := os.Create(filepath.Join(resultsDir, name+".out"))
	errF, _ := os.Create(filepath.Join(resultsDir, name+".err"))
	if outF != nil {
		defer outF.Close()
		cmd.Stdout = outF
	}
	if errF != nil {
		defer errF.Close()
		cmd.Stderr = errF
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 124 // matches coreutils timeout exit code
		}
		if ctx.Err() == context.Canceled {
			return 143 // SIGTERM-equivalent
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// setWorkerStatus optionally shells out to cycle-state.sh for worker status.
func (cfg Config) setWorkerStatus(name, status string, rc int) {
	if !cfg.TrackWorkers || cfg.CycleStateHelperBin == "" {
		return
	}
	args := []string{cfg.CycleStateHelperBin, "set-worker-status", name, status}
	if status != "running" {
		args = append(args, fmt.Sprintf("%d", rc))
	}
	cmd := exec.Command("bash", args...)
	cmd.Env = os.Environ()
	_ = cmd.Run()
}

// ── helpers ────────────────────────────────────────────────────────────────

// Command represents a row from the commands TSV.
type Command struct {
	Name    string
	Command string
}

// ReadCommands parses a TSV of (name\tcommand) pairs. Empty lines and lines
// without a tab are ignored.
func ReadCommands(path string) ([]Command, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Command
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		idx := strings.Index(line, "\t")
		if idx <= 0 {
			continue
		}
		out = append(out, Command{Name: line[:idx], Command: line[idx+1:]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// failVerdictRe matches inline "Verdict: FAIL" (case-insensitive, optional
// markdown bold/asterisks around FAIL).
var failVerdictRe = regexp.MustCompile(`(?i)^\s*verdict:\s*\**\s*FAIL\b`)

// checkFailConsensus scans .out files in resultsDir for FAIL verdicts and
// returns true if at least k workers have voted FAIL.
func checkFailConsensus(resultsDir string, k int) bool {
	matches, err := filepath.Glob(filepath.Join(resultsDir, "*.out"))
	if err != nil {
		return false
	}
	count := 0
	for _, p := range matches {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		matched := false
		for scanner.Scan() {
			if failVerdictRe.MatchString(scanner.Text()) {
				matched = true
				break
			}
		}
		f.Close()
		if matched {
			count++
		}
	}
	return count >= k
}

// ErrUsage is returned for argument-validation failures.
var ErrUsage = errors.New("fanoutdispatch: usage error")
