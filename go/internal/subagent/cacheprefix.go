package subagent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CachePrefixRequest is the typed input to WriteCachePrefix. Workspace is the
// per-cycle .evolve/runs/cycle-N/ dir; OutPath is where the prefix file lands.
// ProjectRoot is the writable repo root (used to locate cycle-state.json).
type CachePrefixRequest struct {
	Cycle       int
	Agent       string
	Workspace   string
	OutPath     string
	ProjectRoot string
}

// CachePrefixOptions injects filesystem seams. Production uses defaults.
type CachePrefixOptions struct {
	// ReadOrchestratorPrompt returns the contents of <workspace>/orchestrator-prompt.md
	// or ("", os.ErrNotExist) when absent. Defaults to os.ReadFile.
	ReadOrchestratorPrompt func(workspace string) (string, error)
	// ReadCycleState returns the contents of <projectRoot>/.evolve/cycle-state.json
	// or ("", os.ErrNotExist) when absent. Defaults to os.ReadFile.
	ReadCycleState func(projectRoot string) (string, error)
}

// WriteCachePrefix renders a deterministic markdown cache-prefix file used by
// sibling fan-out workers in the same batch. Same cycle+workspace+agent must
// produce byte-identical bytes — no timestamps, no randomness — so the
// Anthropic prompt cache reuses the prefix across workers.
//
// Mirrors _write_cache_prefix in legacy/scripts/dispatch/subagent-run.sh
// (v8.23.0 Task C). Goal is extracted from orchestrator-prompt.md if present
// (line matching `^goal:\s*`); cycle-state condensed to a single-line summary
// of phase + active_agent + completed_phases.
func WriteCachePrefix(req CachePrefixRequest, opts CachePrefixOptions) error {
	if opts.ReadOrchestratorPrompt == nil {
		opts.ReadOrchestratorPrompt = defaultReadOrchestratorPrompt
	}
	if opts.ReadCycleState == nil {
		opts.ReadCycleState = defaultReadCycleState
	}

	goalText := "(no goal extracted)"
	if body, err := opts.ReadOrchestratorPrompt(req.Workspace); err == nil {
		if extracted := extractGoalLine(body); extracted != "" {
			goalText = extracted
		}
	}

	csSummary := "(cycle-state unavailable)"
	if body, err := opts.ReadCycleState(req.ProjectRoot); err == nil {
		csSummary = summarizeCycleState(body)
	}

	if err := os.MkdirAll(filepath.Dir(req.OutPath), 0o755); err != nil {
		return fmt.Errorf("subagent/cacheprefix: mkdir: %w", err)
	}

	f, err := os.Create(req.OutPath)
	if err != nil {
		return fmt.Errorf("subagent/cacheprefix: create %s: %w", req.OutPath, err)
	}
	defer func() { _ = f.Close() }()
	w := bufio.NewWriter(f)
	if err := renderCachePrefix(w, req, goalText, csSummary); err != nil {
		return err
	}
	return w.Flush()
}

// renderCachePrefix writes the markdown body. Format is byte-identical to the
// bash _write_cache_prefix output for the same (agent, cycle, workspace,
// goal, cs_summary) tuple.
func renderCachePrefix(w io.Writer, req CachePrefixRequest, goalText, csSummary string) error {
	parts := []string{
		"<!-- cache-prefix v8.23.0 — shared across sibling fan-out workers -->\n",
		fmt.Sprintf("<!-- agent=%s cycle=%d workspace=%s -->\n\n", req.Agent, req.Cycle, req.Workspace),
		fmt.Sprintf("# Shared Context for Cycle %d — %s phase\n\n", req.Cycle, req.Agent),
		fmt.Sprintf("## Cycle Goal\n\n%s\n\n", goalText),
		fmt.Sprintf("## Cycle-State Summary\n\n%s\n\n", csSummary),
		"## Trust Boundary Reminders\n\n",
		"- Personas cannot spawn personas (Claude Code structural enforcement)\n",
		"- Builder is excluded from fan-out (single-writer-per-worktree invariant)\n",
		"- Aggregate artifact is the only thing phase-gate validates\n",
		"- Worker artifacts are written under the workspace, not into source tree\n",
		"- Each fan-out worker is independent — no cross-worker writes\n\n",
		"## Output Format\n\n",
		"Write your worker artifact to the path passed in $EVOLVE_FANOUT_WORKER_ARTIFACT\n",
		"or the standard $WORKSPACE/workers/<agent>-<worker>.md location. The aggregator\n",
		"will merge sibling worker outputs into the canonical phase artifact.\n\n",
		"<!-- end cache-prefix -->\n",
	}
	for _, p := range parts {
		if _, err := io.WriteString(w, p); err != nil {
			return fmt.Errorf("subagent/cacheprefix: write: %w", err)
		}
	}
	return nil
}

// goalLineRE matches the first `goal: <text>` line in orchestrator-prompt.md.
// Bash uses `grep -m1 -E '^goal:[[:space:]]*'` + `sed -E 's/^goal:[[:space:]]*//`.
var goalLineRE = regexp.MustCompile(`(?m)^goal:[ \t]*(.*)$`)

func extractGoalLine(body string) string {
	m := goalLineRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimRight(m[1], "\r\n")
}

// completedPhasesRE captures the JSON array body. Bash uses jq, but we keep
// the package free of jq by parsing a narrow shape — same as ResolveModelTier's
// streak extraction in modeltier.go.
var (
	phaseFieldRE      = regexp.MustCompile(`"phase"\s*:\s*"([^"]*)"`)
	activeAgentRE     = regexp.MustCompile(`"active_agent"\s*:\s*"([^"]*)"`)
	completedPhasesRE = regexp.MustCompile(`"completed_phases"\s*:\s*\[([^\]]*)\]`)
)

func summarizeCycleState(body string) string {
	phase := "unknown"
	if m := phaseFieldRE.FindStringSubmatch(body); len(m) == 2 {
		phase = m[1]
	}
	activeAgent := "none"
	if m := activeAgentRE.FindStringSubmatch(body); len(m) == 2 {
		activeAgent = m[1]
	}
	completed := ""
	if m := completedPhasesRE.FindStringSubmatch(body); len(m) == 2 {
		// Body is like `"a","b","c"`. Strip quotes + whitespace per element,
		// rejoin with commas — matches bash `((.completed_phases // []) | join(","))`.
		raw := m[1]
		var items []string
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"`)
			if item != "" {
				items = append(items, item)
			}
		}
		completed = strings.Join(items, ",")
	}
	return fmt.Sprintf("phase=%s active_agent=%s completed_phases=[%s]", phase, activeAgent, completed)
}

func defaultReadOrchestratorPrompt(workspace string) (string, error) {
	body, err := os.ReadFile(filepath.Join(workspace, "orchestrator-prompt.md"))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func defaultReadCycleState(projectRoot string) (string, error) {
	body, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "cycle-state.json"))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
