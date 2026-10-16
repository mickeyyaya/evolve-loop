// Package fanoutdispatch runs the commands of a TSV file as bounded parallel workers and merges their exit codes.
// See docs/architecture/packages/internal-fanoutdispatch.md.
package fanoutdispatch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	ExitOK         = 0
	ExitWorkerFail = 1
	ExitSetupErr   = 2
)

const (
	exitWorkerTimedOut   = 124
	exitWorkerTerminated = 143
)

type Config struct {
	CommandsFile        string
	ResultsFile         string
	Concurrency         int
	TimeoutSecs         int
	CancelOnConsensus   bool
	ConsensusK          int
	ConsensusPollSecs   int
	CachePrefixFile     string
	CycleStateHelperBin string
	TrackWorkers        bool
	Now                 func() time.Time
}

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
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	resultsDir := filepath.Dir(cfg.ResultsFile)
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		logf("mkdir results dir: %v", err)
		return ExitSetupErr
	}

	commands, err := ReadCommands(cfg.CommandsFile)
	if err != nil {
		logf("read commands: %v", err)
		return ExitSetupErr
	}
	if len(commands) == 0 {
		_ = os.WriteFile(cfg.ResultsFile, []byte{}, 0o644)
		return ExitOK
	}

	type result struct {
		name     string
		exitCode int
		duration int64
	}
	results := make(map[string]result, len(commands))
	resultsMu := sync.Mutex{}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

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

			metaPath := filepath.Join(resultsDir, c.Name+".meta")
			_ = os.WriteFile(metaPath,
				[]byte(fmt.Sprintf("%s\t%d\t%d\n", c.Name, rc, dur)),
				0o644)

			resultsMu.Lock()
			results[c.Name] = result{name: c.Name, exitCode: rc, duration: dur}
			resultsMu.Unlock()
		}()
	}

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

func (cfg Config) workerEnv() []string {
	env := os.Environ()
	if cfg.CachePrefixFile != "" {
		env = append(env, "EVOLVE_FANOUT_CACHE_PREFIX_FILE="+cfg.CachePrefixFile)
	}
	return env
}

func (cfg Config) runWorker(parent context.Context, name, command, resultsDir string) int {
	ctx, cancel := context.WithTimeout(parent, time.Duration(cfg.TimeoutSecs)*time.Second)
	defer cancel()

	cmd := sysexec.Command(ctx, "/bin/sh", "-c", command)
	cmd.Env = cfg.workerEnv()
	outF, _ := os.Create(filepath.Join(resultsDir, name+".out"))
	errF, _ := os.Create(filepath.Join(resultsDir, name+".err"))
	if outF != nil {
		defer func() { _ = outF.Close() }()
		cmd.Stdout = outF
	}
	if errF != nil {
		defer func() { _ = errF.Close() }()
		cmd.Stderr = errF
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return exitWorkerTimedOut
		}
		if ctx.Err() == context.Canceled {
			return exitWorkerTerminated
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

func (cfg Config) setWorkerStatus(name, status string, rc int) {
	if !cfg.TrackWorkers || cfg.CycleStateHelperBin == "" {
		return
	}
	args := []string{cfg.CycleStateHelperBin, "set-worker-status", name, status}
	if status != "running" {
		args = append(args, fmt.Sprintf("%d", rc))
	}
	cmd := sysexec.Command(context.Background(), "bash", args...)
	cmd.Env = os.Environ()
	_ = cmd.Run()
}

type Command struct {
	Name    string
	Command string
}

func ReadCommands(path string) ([]Command, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
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

var failVerdictRe = regexp.MustCompile(`(?i)^\s*verdict:\s*\**\s*FAIL\b`)

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
		_ = f.Close()
		if matched {
			count++
		}
	}
	return count >= k
}
