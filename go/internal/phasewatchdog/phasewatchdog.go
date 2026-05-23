// Package phasewatchdog ports legacy/scripts/dispatch/phase-watchdog.sh.
//
// Activity-based phase watchdog. Background loop that polls file mtimes
// within a workspace and kills a stalled process group when no file
// activity has been detected for longer than the threshold.
//
// v9.4.0 phase-aware baseline: the effective idle baseline is the LATER of
// (a) the freshest watched-file mtime and (b) the current phase's start
// time. A new phase advance resets the clock — preventing false positives
// when a phase produces zero writes but is actively progressing.
package phasewatchdog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Config is the runtime configuration for the watchdog. All durations are
// in seconds. Defaults are applied when zero values are provided.
type Config struct {
	Workspace       string        // required
	TargetPGID      int           // required > 0
	Cycle           int           // required > 0
	CycleStatePath  string        // required
	ProjectRoot     string        // optional, enables ledger.jsonl mtime tracking
	ThresholdS      int           // default 600
	PollS           int           // default 15
	WarnPct         int           // default 75
	GraceS          int           // default 10
	Disabled        bool          // EVOLVE_INACTIVITY_DISABLE=1
	Now             func() time.Time
	Sleep           func(d time.Duration)
	KillPgrp        func(pgid int, sig syscall.Signal) error
	StopAfter       int           // testing seam: stop loop after N iterations (0 = unlimited)
}

// Exit codes:
//   0 — fired completed kill sequence OR watchdog disabled
//   1 — invalid arguments / workspace missing
const (
	ExitOK         = 0
	ExitInvalidArg = 1
)

// Run executes the watchdog loop. stderr receives [phase-watchdog] log lines.
// Returns 0 on completion (fire or disabled exit) or 1 on validation errors.
func Run(cfg Config, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-watchdog] "+format+"\n", args...)
	}

	// defaults
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

	// validation
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
		logf("EVOLVE_INACTIVITY_DISABLE=1 — watchdog disabled, exiting.")
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

		// phase transition detection
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

		// workspace globs — match phase-watchdog.sh's 4 patterns
		patterns := []string{"*.log", "*.md", "*.json", "*-observer-events.ndjson"}
		for _, pat := range patterns {
			m, p := newestMatch(filepath.Join(cfg.Workspace, pat))
			if m > bestMtime {
				bestMtime = m
				bestPath = p
			}
		}

		// cycle-state path
		if m := mtimeUnix(cfg.CycleStatePath); m > bestMtime {
			bestMtime = m
			bestPath = cfg.CycleStatePath
		}

		// ledger (optional)
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

// fireSequence writes stall-progress.json + abnormal-events.jsonl, then
// sends SIGTERM → grace → SIGKILL to the target process group.
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

	// SIGTERM
	logf("sending SIGTERM to pgid %d", cfg.TargetPGID)
	_ = cfg.KillPgrp(cfg.TargetPGID, syscall.SIGTERM)

	// grace
	cfg.Sleep(time.Duration(cfg.GraceS) * time.Second)

	// SIGKILL
	logf("sending SIGKILL to pgid %d (post-grace)", cfg.TargetPGID)
	_ = cfg.KillPgrp(cfg.TargetPGID, syscall.SIGKILL)

	logf("kill sequence complete for pgid %d", cfg.TargetPGID)
}

// readPhase reads cycle-state.json:phase. Returns "" on any error.
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

// mtimeUnix returns the file's modification time as a Unix timestamp, or 0
// if the file doesn't exist or can't be stat'd.
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

// newestMatch scans a glob pattern and returns the newest mtime + its path.
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

// appendAbnormalEvent writes a one-line JSON event to abnormal-events.jsonl
// in the workspace (best-effort; ignores errors).
func appendAbnormalEvent(workspace, details string, now func() time.Time) {
	if _, err := os.Stat(workspace); err != nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	entry := map[string]any{
		"event_type":        "stall-detected",
		"timestamp":         now().UTC().Format("2006-01-02T15:04:05Z"),
		"source_phase":      "phase-watchdog",
		"severity":          "HIGH",
		"details":           details,
		"remediation_hint":  "Check agent turn count; reduce scope or increase EVOLVE_INACTIVITY_THRESHOLD_S",
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
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// defaultKillPgrp sends sig to the process group identified by pgid.
// On Unix systems, sending to -pgid signals the entire group.
func defaultKillPgrp(pgid int, sig syscall.Signal) error {
	return syscall.Kill(-pgid, sig)
}
