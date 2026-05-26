package phasestream

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProduceConfig configures a post-phase one-shot events render. The raw log
// paths are derived from Workspace+Phase (<phase>-stdout.log /
// <phase>-stderr.log), matching the bridge's naming and logfilter.Process.
type ProduceConfig struct {
	Workspace string
	Phase     string // file stem + envelope agent/phase
	CLI       string
	Cycle     int
	Now       func() time.Time // defaults to time.Now
}

// produceScanBufBytes caps a raw log line; a result envelope can embed a
// large payload. Generous, matching cyclecost's reader.
const produceScanBufBytes = 1 << 24 // 16MB

// Produce reads a finished phase's complete raw <phase>-stdout.log (and
// optional <phase>-stderr.log) and writes the unified <phase>-events.ndjson
// through the same Classifier the live Normalizer uses (ADR-0020). This is the
// post-phase path for the synchronous-bridge runner, where the whole log
// exists at once.
//
// Unlike the live Poll tail — which deliberately holds back an unterminated
// final line to avoid splitting a still-being-written record — Produce reads
// each log in full, so the billing-critical trailing `result` event is never
// dropped. The write is atomic (temp + rename) so the post-cycle readers
// (cyclecost, cycleclassify) never observe a half-written events file, and the
// temp never pollutes their *-events.ndjson glob.
//
// A missing stdout log is a no-op (returns nil): the phase produced no output.
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

	// Coalesced liveness tick for any stream_event burst (parity with Poll).
	if env, ok := clf.FlushProgress(); ok {
		envs = append(envs, env)
	}

	return writeEventsFile(cfg.Workspace, cfg.Phase, envs)
}

// classifyLines reads logPath in full and runs each line (including a final
// unterminated one) through classify, accumulating the emitted envelopes.
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

// writeEventsFile atomically writes the envelope stream to
// <workspace>/<phase>-events.ndjson via a temp file + rename. The temp name is
// dot-prefixed so a crash mid-write cannot leave a file matching the consumers'
// *-events.ndjson glob.
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
