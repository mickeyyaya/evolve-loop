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

type NormalizerConfig struct {
	Source     Source
	TraceID    string
	StdoutPath string
	StderrPath string
	Sink       io.Writer
	StallS     int
	Enforce    bool
	PGID       int
	Now        func() time.Time
	KillPgrp   func(pgid int, sig syscall.Signal) error
}

type Normalizer struct {
	cfg          NormalizerConfig
	classifier   *Classifier
	stdoutOff    int64
	stderrOff    int64
	lastActivity time.Time
	stallFired   bool
}

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

	if env, ok := n.classifier.FlushProgress(); ok {
		out = append(out, env)
	}

	if len(out) > 0 {
		n.lastActivity = n.cfg.Now()
		n.stallFired = false
	}

	if n.cfg.StallS > 0 && !n.stallFired {
		idle := n.cfg.Now().Sub(n.lastActivity)
		if int(idle.Seconds()) >= n.cfg.StallS {
			data := map[string]any{
				"idle_s":      int64(idle.Seconds()),
				"threshold_s": int64(n.cfg.StallS),
			}
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
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, off
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, off
	}
	lastNL := bytes.LastIndexByte(data, '\n')
	if lastNL < 0 {
		return nil, off
	}
	var lines []string
	for _, ln := range bytes.Split(data[:lastNL], []byte{'\n'}) {
		lines = append(lines, string(ln))
	}
	return lines, off + int64(lastNL) + 1
}

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
