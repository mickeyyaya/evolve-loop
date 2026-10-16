// Package phasewatchdog kills the process group of a phase that shows no file activity for too long.
// See docs/architecture/packages/internal-phasewatchdog.md.
package phasewatchdog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

type Config struct {
	Workspace      string
	TargetPGID     int
	Cycle          int
	CycleStatePath string
	ProjectRoot    string
	ThresholdS     int
	PollS          int
	WarnPct        int
	GraceS         int
	Disabled       bool
	Now            func() time.Time
	Sleep          func(d time.Duration)
	KillPgrp       func(pgid int, sig syscall.Signal) error
	StopAfter      int
}

const (
	ExitOK         = 0
	ExitInvalidArg = 1
)

func Run(cfg Config, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-watchdog] "+format+"\n", args...)
	}

	if cfg.ThresholdS == 0 {
		cfg.ThresholdS = 600
	}
	if cfg.PollS == 0 {
		cfg.PollS = 15
	}
	if cfg.WarnPct == 0 {
		cfg.WarnPct = 75
	}
	if cfg.GraceS == 0 {
		cfg.GraceS = 10
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = time.Sleep
	}
	if cfg.KillPgrp == nil {
		cfg.KillPgrp = defaultKillPgrp
	}

	if cfg.Workspace == "" {
		logf("ERROR: workspace required")
		return ExitInvalidArg
	}
	if info, err := os.Stat(cfg.Workspace); err != nil || !info.IsDir() {
		logf("ERROR: workspace directory does not exist: %s", cfg.Workspace)
		return ExitInvalidArg
	}
	if cfg.TargetPGID <= 0 {
		logf("ERROR: target_pgid must be a positive integer, got: %d", cfg.TargetPGID)
		return ExitInvalidArg
	}
	if cfg.Cycle <= 0 {
		logf("ERROR: cycle must be a positive integer, got: %d", cfg.Cycle)
		return ExitInvalidArg
	}

	if cfg.Disabled {
		logf("observer policy disables watchdog; exiting.")
		return ExitOK
	}

	logf("started: workspace=%s pgid=%d cycle=%d threshold=%ds poll=%ds warn_pct=%d%%",
		cfg.Workspace, cfg.TargetPGID, cfg.Cycle, cfg.ThresholdS, cfg.PollS, cfg.WarnPct)

	warnS := cfg.ThresholdS * cfg.WarnPct / 100
	startTime := cfg.Now().Unix()
	phaseStartTime := startTime
	lastPhase := ""
	idleClockStarted := false
	warnEmitted := false

	iter := 0
	for {
		iter++
		if cfg.StopAfter > 0 && iter > cfg.StopAfter {
			return ExitOK
		}

		now := cfg.Now().Unix()
		bestMtime := int64(0)
		bestPath := ""

		if currentPhase := readPhase(cfg.CycleStatePath); currentPhase != "" && currentPhase != lastPhase {
			if lastPhase != "" {
				logf("phase advance: '%s' → '%s' (resetting baseline to now)", lastPhase, currentPhase)
			} else {
				logf("phase observed: '%s' (baseline anchored to start_time=%d)", currentPhase, startTime)
			}
			phaseStartTime = now
			lastPhase = currentPhase
			warnEmitted = false
		}

		patterns := []string{"*.log", "*.md", "*.json", "*-observer-events.ndjson"}
		for _, pat := range patterns {
			m, p := newestMatch(filepath.Join(cfg.Workspace, pat))
			if m > bestMtime {
				bestMtime = m
				bestPath = p
			}
		}

		if m := mtimeUnix(cfg.CycleStatePath); m > bestMtime {
			bestMtime = m
			bestPath = cfg.CycleStatePath
		}

		if cfg.ProjectRoot != "" {
			ledger := filepath.Join(cfg.ProjectRoot, ".evolve", "ledger.jsonl")
			if m := mtimeUnix(ledger); m > bestMtime {
				bestMtime = m
				bestPath = ledger
			}
		}

		if bestMtime > 0 {
			idleClockStarted = true
		}

		if idleClockStarted && bestMtime > 0 {
			baseline := bestMtime
			if phaseStartTime > baseline {
				baseline = phaseStartTime
				bestPath = fmt.Sprintf("<phase-start anchor (phase=%s)>", lastPhase)
			}
			idleS := now - baseline
			if idleS < 0 {
				idleS = 0
			}

			if int(idleS) >= warnS && !warnEmitted {
				logf("WARN: idle for %ds (warn threshold %ds); stall threshold %ds; last activity: %s",
					idleS, warnS, cfg.ThresholdS, bestPath)
				warnEmitted = true
			}
			if int(idleS) < warnS {
				warnEmitted = false
			}

			if int(idleS) >= cfg.ThresholdS {
				logf("FIRE: idle for %ds >= threshold %ds; last file: %s", idleS, cfg.ThresholdS, bestPath)
				cfg.fireSequence(idleS, bestPath, logf)
				return ExitOK
			}
		}

		cfg.Sleep(time.Duration(cfg.PollS) * time.Second)
	}
}

func (cfg Config) fireSequence(idleS int64, lastPath string, logf func(string, ...any)) {
	checkpointTS := cfg.Now().UTC().Format("2006-01-02T15:04:05Z")
	stallEntry := map[string]any{
		"idle_s":          idleS,
		"threshold_s":     cfg.ThresholdS,
		"last_mtime_file": lastPath,
		"checkpoint_ts":   checkpointTS,
	}
	stallBytes, _ := json.Marshal(stallEntry)
	stallFile := filepath.Join(cfg.Workspace, "stall-progress.json")
	tmp := stallFile + ".tmp"
	if err := os.WriteFile(tmp, append(stallBytes, '\n'), 0o644); err == nil {
		_ = os.Rename(tmp, stallFile)
		logf("wrote stall-progress.json: %s", stallFile)
	}
	appendAbnormalEvent(cfg.Workspace, fmt.Sprintf("idle_s=%d threshold_s=%d last_file=%s cycle=%d",
		idleS, cfg.ThresholdS, lastPath, cfg.Cycle), cfg.Now)

	logf("sending SIGTERM to pgid %d", cfg.TargetPGID)
	_ = cfg.KillPgrp(cfg.TargetPGID, syscall.SIGTERM)

	cfg.Sleep(time.Duration(cfg.GraceS) * time.Second)

	logf("sending SIGKILL to pgid %d (post-grace)", cfg.TargetPGID)
	_ = cfg.KillPgrp(cfg.TargetPGID, syscall.SIGKILL)

	logf("kill sequence complete for pgid %d", cfg.TargetPGID)
}

func readPhase(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return doc.Phase
}

func mtimeUnix(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

func newestMatch(pattern string) (int64, string) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, ""
	}
	best := int64(0)
	bestPath := ""
	for _, p := range matches {
		m := mtimeUnix(p)
		if m > best {
			best = m
			bestPath = p
		}
	}
	return best, bestPath
}

func appendAbnormalEvent(workspace, details string, now func() time.Time) {
	if _, err := os.Stat(workspace); err != nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	entry := map[string]any{
		"event_type":       string(dispatchevents.EventStallDetected),
		"timestamp":        now().UTC().Format("2006-01-02T15:04:05Z"),
		"source_phase":     "phase-watchdog",
		"severity":         string(dispatchevents.SeverityError),
		"details":          details,
		"remediation_hint": "Check agent turn count; reduce scope or increase observer.stall_s in policy.json",
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(workspace, "abnormal-events.jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(b, '\n'))
}

func defaultKillPgrp(pgid int, sig syscall.Signal) error {
	return syscall.Kill(-pgid, sig)
}
