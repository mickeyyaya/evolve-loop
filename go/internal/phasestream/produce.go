package phasestream

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProduceConfig struct {
	Workspace      string
	Phase          string
	CLI            string
	Cycle          int
	InjectedPrompt string
	Now            func() time.Time
}

const produceScanBufBytes = 1 << 24

func Produce(cfg ProduceConfig) error {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	stdoutPath := filepath.Join(cfg.Workspace, cfg.Phase+"-stdout.log")
	if _, err := os.Stat(stdoutPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("phasestream: stat %s: %w", stdoutPath, err)
	}

	clf := NewClassifier(Source{
		Producer: "normalizer",
		CLI:      cfg.CLI,
		Cycle:    cfg.Cycle,
		Phase:    cfg.Phase,
		Agent:    cfg.Phase,
	}, fmt.Sprintf("cycle-%d-%s", cfg.Cycle, cfg.Phase), now)
	clf.SetInjectedPrompt(cfg.InjectedPrompt)

	var envs []Envelope
	stdoutEnvs, err := classifyLines(stdoutPath, clf.Line)
	if err != nil {
		return err
	}
	envs = append(envs, stdoutEnvs...)

	stderrPath := filepath.Join(cfg.Workspace, cfg.Phase+"-stderr.log")
	if _, err := os.Stat(stderrPath); err == nil {
		stderrEnvs, err := classifyLines(stderrPath, clf.Stderr)
		if err != nil {
			return err
		}
		envs = append(envs, stderrEnvs...)
	}

	if env, ok := clf.FlushProgress(); ok {
		envs = append(envs, env)
	}

	return writeEventsFile(cfg.Workspace, cfg.Phase, envs)
}

func classifyLines(logPath string, classify func([]byte) []Envelope) ([]Envelope, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("phasestream: open %s: %w", logPath, err)
	}
	defer func() { _ = f.Close() }()

	var out []Envelope
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<10), produceScanBufBytes)
	for sc.Scan() {
		out = append(out, classify(sc.Bytes())...)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("phasestream: scan %s: %w", logPath, err)
	}
	return out, nil
}

func writeEventsFile(workspace, phase string, envs []Envelope) error {
	tmp, err := os.CreateTemp(workspace, "."+phase+"-events.ndjson.tmp.*")
	if err != nil {
		return fmt.Errorf("phasestream: create temp events: %w", err)
	}
	tmpPath := tmp.Name()
	w := bufio.NewWriter(tmp)
	for _, e := range envs {
		if err := writeEnvelope(w, e); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("phasestream: flush events: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("phasestream: close events: %w", err)
	}
	dst := filepath.Join(workspace, phase+"-events.ndjson")
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("phasestream: rename events: %w", err)
	}
	return nil
}
