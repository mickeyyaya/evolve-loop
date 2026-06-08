package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// report.go — re-derive a structured JSON summary from a past workspace
// (Go port of lib/report.sh). Verdict is derived from file state, not a
// status file. Consumed by `evolve bridge report` and the orchestrator.

// FileRef describes a workspace file's presence + size.
type FileRef struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
}

// ArtifactRef is the artifact's presence + challenge-token match.
type ArtifactRef struct {
	Path              string `json:"path"`
	Exists            bool   `json:"exists"`
	SizeBytes         int64  `json:"size_bytes"`
	HasChallengeToken bool   `json:"has_challenge_token"`
}

// Report is the structured workspace summary (mirrors report.sh's JSON).
type Report struct {
	Workspace      string      `json:"workspace"`
	ScannedAt      string      `json:"scanned_at"`
	Verdict        string      `json:"verdict"`
	Artifact       ArtifactRef `json:"artifact"`
	ChallengeToken *string     `json:"challenge_token"`
	Logs           struct {
		StdoutLog      FileRef `json:"stdout_log"`
		StderrLog      FileRef `json:"stderr_log"`
		TmuxScrollback FileRef `json:"tmux_scrollback"`
		ResolvedPrompt FileRef `json:"resolved_prompt"`
	} `json:"logs"`
	EscalationReport  FileRef `json:"escalation_report"`
	AutoRespondCounts FileRef `json:"auto_respond_counts"`
	TokenUsage        int     `json:"token_usage"`
}

// statRef builds a FileRef by stat'ing path.
func statRef(path string) FileRef {
	r := FileRef{Path: path}
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		r.Exists = true
		r.SizeBytes = fi.Size()
	}
	return r
}

// BuildReport scans a workspace and derives the summary + verdict:
//
//	escalated                  — escalation-report.json present
//	complete                   — artifact present + (token matches or no token file)
//	incomplete-token-mismatch  — artifact present but token file value not in artifact
//	incomplete                 — no artifact, no escalation
func BuildReport(workspace, artifactName string, now time.Time) (Report, error) {
	if artifactName == "" {
		artifactName = "artifact.md"
	}
	if fi, err := os.Stat(workspace); err != nil || !fi.IsDir() {
		return Report{}, os.ErrNotExist
	}

	artifactPath := filepath.Join(workspace, artifactName)
	art := ArtifactRef{Path: artifactPath}
	var artifactBytes []byte
	if fi, err := os.Stat(artifactPath); err == nil && !fi.IsDir() {
		art.Exists = true
		art.SizeBytes = fi.Size()
		artifactBytes, _ = os.ReadFile(artifactPath)
	}

	var token *string
	if b, err := os.ReadFile(filepath.Join(workspace, "challenge-token.txt")); err == nil {
		v := strings.TrimSpace(string(b))
		token = &v
		if art.Exists && v != "" && strings.Contains(string(artifactBytes), v) {
			art.HasChallengeToken = true
		}
	}

	rep := Report{
		Workspace:      workspace,
		ScannedAt:      now.UTC().Format("2006-01-02T15:04:05Z"),
		Artifact:       art,
		ChallengeToken: token,
	}
	rep.Logs.StdoutLog = statRef(filepath.Join(workspace, "stdout.log"))
	rep.Logs.StderrLog = statRef(filepath.Join(workspace, "stderr.log"))
	rep.Logs.TmuxScrollback = statRef(filepath.Join(workspace, "tmux-final-scrollback.txt"))
	rep.Logs.ResolvedPrompt = statRef(filepath.Join(workspace, "resolved-prompt.txt"))
	rep.EscalationReport = statRef(filepath.Join(workspace, "escalation-report.json"))
	rep.AutoRespondCounts = statRef(filepath.Join(workspace, "auto-respond-counts.csv"))
	rep.TokenUsage = readTokenUsage(filepath.Join(workspace, "token-usage.json"))

	switch {
	case rep.EscalationReport.Exists:
		rep.Verdict = "escalated"
	case art.Exists:
		if token == nil || *token == "" || art.HasChallengeToken {
			rep.Verdict = "complete"
		} else {
			rep.Verdict = "incomplete-token-mismatch"
		}
	default:
		rep.Verdict = "incomplete"
	}
	return rep, nil
}

func readTokenUsage(path string) int {
	var usage struct {
		PeakTokens int `json:"peak_tokens"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if err := json.Unmarshal(data, &usage); err != nil || usage.PeakTokens < 0 {
		return 0
	}
	return usage.PeakTokens
}
