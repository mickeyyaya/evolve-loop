//go:build acs

package cycle2098

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type capturingLauncher struct {
	requests []advisor.LaunchRequest
}

func (c *capturingLauncher) Launch(_ context.Context, req advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	c.requests = append(c.requests, req)
	return advisor.LaunchResponse{}, nil
}

type advisorSeat struct {
	name    string
	persona string
	drive   func(a *advisor.Advisor, in router.RouteInput)
}

func advisorSeats() []advisorSeat {
	return []advisorSeat{
		{"plan with persona", "PERSONA", func(a *advisor.Advisor, in router.RouteInput) { _, _ = a.Plan(in) }},
		{"replan with persona", "PERSONA", func(a *advisor.Advisor, in router.RouteInput) { _, _ = a.RePlan(in) }},
		{"plan without persona", "", func(a *advisor.Advisor, in router.RouteInput) { _, _ = a.Plan(in) }},
		{"proposal", "", func(a *advisor.Advisor, in router.RouteInput) { _, _ = a.Propose(in) }},
	}
}

func launchAdvisorSeat(t *testing.T, seat advisorSeat, workspace string) advisor.LaunchRequest {
	t.Helper()
	launcher := &capturingLauncher{}
	identity := advisor.Identity{CLI: "claude-tmux", Model: "deep", Persona: seat.persona, AgentLabel: "router"}
	a := advisor.New(launcher, identity, nil, advisor.WithOverlayResolver(func(string) []string { return nil }))
	seat.drive(a, router.RouteInput{Workspace: workspace, Cycle: 2098})
	if len(launcher.requests) != 1 {
		t.Fatalf("%s: launcher saw %d requests, want 1", seat.name, len(launcher.requests))
	}
	return launcher.requests[0]
}

func writeRecordedTranscript(t *testing.T, configRoot, cwd, firstUserPrompt string, ts time.Time) {
	t.Helper()
	user, err := json.Marshal(map[string]any{
		"type": "user", "cwd": cwd, "timestamp": ts.Format(time.RFC3339),
		"message": map[string]any{"id": "u1", "content": firstUserPrompt},
	})
	if err != nil {
		t.Fatal(err)
	}
	assistant := `{"type":"assistant","cwd":"` + cwd + `","timestamp":"` + ts.Add(4*time.Second).Format(time.RFC3339) +
		`","message":{"id":"m1","usage":{"input_tokens":11,"output_tokens":900,"cache_read_input_tokens":42000,"cache_creation_input_tokens":30}}}`
	dir := filepath.Join(configRoot, "projects", "-advisor-session")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := string(user) + "\n" + assistant + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resolveLikeTheBridge(configRoot string, req advisor.LaunchRequest, now time.Time) (tokenusage.Result, error) {
	return tokenusage.DefaultResolver(configRoot)(tokenusage.Window{
		Worktree:     req.Worktree,
		ArtifactPath: req.ArtifactPath,
		Driver:       req.CLI,
		Collector:    tokenusage.FormatClaudeJSONL,
		Start:        now.Add(-time.Hour),
		End:          now.Add(time.Hour),
	})
}

func TestC2098_001_AdvisorTranscriptResolvesToMeasuredSource(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, seat := range advisorSeats() {
		t.Run(seat.name, func(t *testing.T) {
			workspace := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2098")
			req := launchAdvisorSeat(t, seat, workspace)
			configRoot := t.TempDir()
			writeRecordedTranscript(t, configRoot, req.Worktree, req.Prompt, now)

			res, err := resolveLikeTheBridge(configRoot, req, now)
			if err != nil {
				t.Fatalf("resolver: %v", err)
			}
			if res.Source != tokenusage.SourceTranscript {
				t.Fatalf("RED: %s resolved to source %q, want %q: the advisor prompt itself must name %s behind an artifact anchor of the token scanner",
					seat.name, res.Source, tokenusage.SourceTranscript, req.ArtifactPath)
			}
			if res.Usage.Output != 900 || res.Usage.CacheRead != 42000 || res.Usage.CacheWrite != 30 {
				t.Errorf("RED: %s usage = %+v, want output 900, cache read 42000, cache write 30", seat.name, res.Usage)
			}
		})
	}
}

func TestC2098_002_TranscriptOfAnotherArtifactStaysUnattributed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, seat := range advisorSeats() {
		t.Run(seat.name, func(t *testing.T) {
			launched := launchAdvisorSeat(t, seat, filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2098"))
			other := launchAdvisorSeat(t, seat, filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2099"))
			configRoot := t.TempDir()
			writeRecordedTranscript(t, configRoot, other.Worktree, other.Prompt, now)

			res, err := resolveLikeTheBridge(configRoot, launched, now)
			if err != nil {
				t.Fatalf("resolver: %v", err)
			}
			if res.Source == tokenusage.SourceTranscript {
				t.Errorf("RED: %s: the transcript of %s was attributed to %s; attribution must anchor on the artifact path, not accept any advisor transcript",
					seat.name, other.ArtifactPath, launched.ArtifactPath)
			}
		})
	}
}

func TestC2098_003_PromptWithoutArtifactAnchorStaysUnattributed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	req := launchAdvisorSeat(t, advisorSeats()[0], filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2098"))
	configRoot := t.TempDir()
	writeRecordedTranscript(t, configRoot, req.Worktree, "plan the cycle and answer in prose, no file named here", now)

	res, err := resolveLikeTheBridge(configRoot, req, now)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if res.Source != tokenusage.SourceNone {
		t.Errorf("source = %q, want %q: a transcript whose first user text names no artifact path must not resolve", res.Source, tokenusage.SourceNone)
	}
}

func TestC2098_004_AnchorNamesTheLaunchArtifactExactlyOnceAsTheWritePath(t *testing.T) {
	for _, seat := range advisorSeats() {
		t.Run(seat.name, func(t *testing.T) {
			req := launchAdvisorSeat(t, seat, filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2098"))
			if got := strings.Count(req.Prompt, "Artifact path: "+req.ArtifactPath); got != 1 {
				t.Errorf("RED: %s prompt has %d anchors `Artifact path: <launch artifact>`, want exactly 1 (a repeated anchor would let a retrospective mention bill the advisor)", seat.name, got)
			}
		})
	}
}

// acs-predicate: config-check
func TestC2098_005_TokenusagePackageNoteRecordsAgyCacheWriteFinding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "packages", "internal-tokenusage.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		low := strings.ToLower(line)
		hasCacheWrite := strings.Contains(low, "cache write") || strings.Contains(low, "cache-write")
		if strings.Contains(low, "agy") && hasCacheWrite && strings.Contains(low, "field") {
			return
		}
	}
	t.Errorf("RED: no line of the tokenusage package note gives the agy cache write a field-level verdict (found a field, or no field exists), plan Q2")
}
