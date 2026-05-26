package phasestream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// NormalizerConfig wires a live Normalizer. StdoutPath/StderrPath point at
// the phase's raw stream-json (or tmux-scrollback) logs; Sink receives the
// unified envelope stream (the caller owns it — typically an O_APPEND file
// handle for <agent>-events.ndjson, so writes from this single goroutine
// preserve the single-writer invariant).
type NormalizerConfig struct {
	Source     Source
	TraceID    string
	StdoutPath string
	StderrPath string // optional — empty disables stderr tailing
	Sink       io.Writer
	StallS     int // 0 disables the stall rule
	Enforce    bool
	PGID       int
	Now        func() time.Time
	KillPgrp   func(pgid int, sig syscall.Signal) error
}

// Normalizer tails one phase's raw logs, classifies each line into the
// unified envelope stream, coalesces stream_event liveness into a single
// progress tick per poll, runs the stall rule, and writes every envelope
// to a single-writer sink. Not safe for concurrent use — one Normalizer
// per phase, driven from a single goroutine (it owns one Classifier).
type Normalizer struct {
	cfg          NormalizerConfig
	classifier   *Classifier
	stdoutOff    int64
	stderrOff    int64
	lastActivity time.Time
	stallFired   bool
}

// NewNormalizer constructs a Normalizer. Now defaults to time.Now; KillPgrp
// defaults to a real process-group signal.
func NewNormalizer(cfg NormalizerConfig) *Normalizer {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.KillPgrp == nil {
		cfg.KillPgrp = func(pgid int, sig syscall.Signal) error {
			return syscall.Kill(-pgid, sig)
		}
	}
	return &Normalizer{
		cfg:          cfg,
		classifier:   NewClassifier(cfg.Source, cfg.TraceID, cfg.Now),
		lastActivity: cfg.Now(),
	}
}

// Poll performs one tail+classify+flush+stall iteration: it reads bytes
// appended to stdout/stderr since the last poll, classifies them, coalesces
// any stream_event burst into one progress tick, applies the stall rule,
// writes every resulting envelope to the sink, and returns them.
func (n *Normalizer) Poll() ([]Envelope, error) {
	var out []Envelope

	var outLines []string
	outLines, n.stdoutOff = tailFile(n.cfg.StdoutPath, n.stdoutOff)
	for _, ln := range outLines {
		out = append(out, n.classifier.Line([]byte(ln))...)
	}

	if n.cfg.StderrPath != "" {
		var errLines []string
		errLines, n.stderrOff = tailFile(n.cfg.StderrPath, n.stderrOff)
		for _, ln := range errLines {
			out = append(out, n.classifier.Stderr([]byte(ln))...)
		}
	}

	// One coalesced progress tick per poll if stream_event deltas accrued.
	if env, ok := n.classifier.FlushProgress(); ok {
		out = append(out, env)
	}

	// Any real activity this poll resets the stall window and re-arms the rule.
	if len(out) > 0 {
		n.lastActivity = n.cfg.Now()
		n.stallFired = false
	}

	// Stall rule — fires at most once per stall window.
	if n.cfg.StallS > 0 && !n.stallFired {
		idle := n.cfg.Now().Sub(n.lastActivity)
		if int(idle.Seconds()) >= n.cfg.StallS {
			data := map[string]any{
				"idle_s":      int64(idle.Seconds()),
				"threshold_s": int64(n.cfg.StallS),
			}
			// Surface the kill outcome on the envelope rather than swallow it,
			// so an enforce failure (bad pgid, permissions) is visible.
			if n.cfg.Enforce && n.cfg.PGID > 0 && n.cfg.KillPgrp != nil {
				if err := n.cfg.KillPgrp(n.cfg.PGID, syscall.SIGTERM); err != nil {
					data["kill_result"] = err.Error()
				} else {
					data["kill_result"] = "ok"
				}
			}
			out = append(out, n.classifier.Emit(KindStall, SeverityIncident, data))
			n.stallFired = true
		}
	}

	if n.cfg.Sink != nil {
		for _, e := range out {
			if err := writeEnvelope(n.cfg.Sink, e); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

// tailFile reads complete lines appended since off and returns them with the
// new offset. It advances the offset only past the last newline, so a partial
// (still-being-written) final line is held back and re-read once completed —
// improving on phaseobserver.tail, which advanced to EOF and so split partial
// lines across polls. A missing/unreadable file is a no-op (returns the
// unchanged offset); a shrunk file (truncation/rotation) restarts from zero.
func tailFile(path string, off int64) ([]string, int64) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, off
	}
	if info.Size() < off {
		off = 0
	}
	if info.Size() == off {
		return nil, off
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, off
	}
	defer f.Close()
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, off
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, off
	}
	lastNL := bytes.LastIndexByte(data, '\n')
	if lastNL < 0 {
		return nil, off // no complete line yet — hold the partial bytes
	}
	var lines []string
	for _, ln := range bytes.Split(data[:lastNL], []byte{'\n'}) {
		lines = append(lines, string(ln))
	}
	return lines, off + int64(lastNL) + 1
}

// writeEnvelope serializes e as one NDJSON line (JSON + '\n') to w. The newline
// terminator is appended here so the sink stays a valid append-only NDJSON
// stream regardless of caller.
func writeEnvelope(w io.Writer, e Envelope) error {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("phasestream: marshal envelope seq %d: %w", e.Seq, err)
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("phasestream: write envelope seq %d: %w", e.Seq, err)
	}
	return nil
}
