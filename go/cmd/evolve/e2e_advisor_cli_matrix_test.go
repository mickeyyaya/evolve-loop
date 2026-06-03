//go:build e2e

// Advisor × all-CLIs LIVE matrix. The routing brain (agents/evolve-router.md) is
// configurable to ANY LLM CLI, so this proves the per-CLI WIRING the brain
// depends on: for each available driver (claude-p/codex/agy + tmux variants),
// fire ONE live `evolve bridge launch` carrying the router persona under the
// artifact-completion contract and assert the driver WRITES a parseable
// routing-plan.json. This is the e2e half of "the advisor is e2e-validated on
// every LLM CLI" — the unit half (phase_advisor_test.go, resolveRouterDispatch)
// pins the Go-side option/precedence wiring without spending quota.
//
// Deliberately at the CHEAP/FAST tier and STRUCTURAL: the matrix proves wiring
// (boots, accepts the persona, writes a valid plan), NOT opus reasoning quality.
// Unavailable binaries SKIP (liveCLIAvailable); quota/rate-limit/timeout SKIP
// (isTransient) — only a booted CLI that fails the artifact contract hard-fails.
// Gate: EVOLVE_E2E_LIVE_ADVISOR=1.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// advisorPlanEntry is the minimal shape of a routing-plan.json entry. Kept local
// (not internal/router.PhasePlanEntry) so the live assertion stays a structural
// contract check decoupled from the kernel types. The "mint" block is
// deliberately omitted: the matrix asserts a valid plan was written, not that a
// phase was minted (mint coverage lives in the kernel unit tests).
type advisorPlanEntry struct {
	Phase         string `json:"phase"`
	Run           bool   `json:"run"`
	Justification string `json:"justification"`
}

// stripFrontmatter removes a leading YAML frontmatter block (--- … ---) from a
// persona markdown body so only the prose instructions reach the prompt. Only a
// genuine block (opening "---" on its own line) is stripped; a decorative
// thematic break that merely starts with dashes is left intact.
func stripFrontmatter(md string) string {
	s := strings.TrimLeft(md, "\n ")
	if !strings.HasPrefix(s, "---\n") {
		return md
	}
	rest := s[strings.IndexByte(s, '\n')+1:] // drop the opening "---" line
	if i := strings.Index(rest, "\n---"); i >= 0 {
		return strings.TrimLeft(rest[i+len("\n---"):], "-\n ")
	}
	return md
}

// parseRoutingPlanArray tolerantly decodes a routing-plan.json artifact into its
// phase entries. Under the artifact-completion contract the file is normally
// clean JSON, but a CLI may wrap it in a ```json fence or add stray prose, so we
// strip a fence and fall back to the FIRST balanced [ … ] span before failing.
func parseRoutingPlanArray(raw []byte) ([]advisorPlanEntry, error) {
	body := strings.TrimSpace(string(raw))
	if after, ok := strings.CutPrefix(body, "```json"); ok {
		body = after
	} else {
		body = strings.TrimPrefix(body, "```")
	}
	body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "```"))

	var entries []advisorPlanEntry
	if err := json.Unmarshal([]byte(body), &entries); err == nil {
		return entries, nil
	}
	span, ok := firstBalancedArray(body)
	if !ok {
		return nil, fmt.Errorf("no JSON array in artifact")
	}
	if err := json.Unmarshal([]byte(span), &entries); err != nil {
		return nil, fmt.Errorf("unmarshal plan array: %w", err)
	}
	return entries, nil
}

// firstBalancedArray returns the first top-level [ … ] span in s, depth-tracking
// brackets while skipping string-literal contents (with escapes) so a ']' inside
// a justification value — or in trailing prose after the array — is not
// miscounted (the bug a naive LastIndexByte(']') would introduce). ok=false when
// no balanced array exists. Mirrors the kernel's lastBalancedSpan but returns the
// FIRST span: under artifact completion the file is the agent's direct write, so
// the array comes before any trailing note.
func firstBalancedArray(s string) (string, bool) {
	depth, start := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			if depth == 0 {
				start = i
			}
			depth++
		case ']':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return s[start : i+1], true
				}
			}
		}
	}
	return "", false
}

// launchAdvisor fires ONE live `evolve bridge launch` carrying the evolve-router
// persona + a representative cycle digest, with artifact=routing-plan.json under
// the artifact-completion contract — the same contract PhaseAdvisor.Plan uses. It
// is the thin per-CLI driver for the advisor matrix, mirroring liveBridgeLaunch
// but returning the artifact BYTES so the caller can assert the plan parses. A
// synthetic permissive profile (allowed_clis:["all"]) keeps the floor from
// rejecting whatever driver the matrix selects — wiring, not the real profile's
// path-scoped permission surface, is what this proves.
func launchAdvisor(t *testing.T, evolveBin, repoRoot, driver, tier string, timeout time.Duration) ([]byte, string, error) {
	t.Helper()
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(ws, "routing-plan.json")

	personaMD, err := os.ReadFile(filepath.Join(repoRoot, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read router persona: %v", err)
	}
	prompt := stripFrontmatter(string(personaMD)) +
		"\n\n---\n# This cycle\n\n" +
		"## Cycle\n- cycle: 0\n- just_completed: start\n- completed_phases: \n" +
		"- mandatory_spine: scout, build, audit, ship\n- budget_remaining_usd: 5.00\n\n" +
		"## Objective signals (digested from handoff artifacts)\n" +
		"- scout: cycle_size_estimate=small item_count=2 carryover=0 backlog=0\n\n" +
		fmt.Sprintf("Now write your whole-cycle plan as a strict JSON array of "+
			"{\"phase\":\"<phase>\",\"run\":true,\"justification\":\"<one sentence>\"} objects "+
			"to the file %s and then stop. No prose, no markdown fence.\n", artifact)

	promptFile := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "router-live-profile.json")
	body := fmt.Sprintf(`{"name":"router","role":"router","allowed_clis":["all"],`+
		`"model_tier_default":%q,"model_tier_envelope":{"min":"fast","default":%q,"max":"deep"},`+
		`"allowed_tools":["Read","Write","Bash"]}`, tier, tier)
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeHome := t.TempDir()
	env := append(os.Environ(), "EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"))

	cmd := exec.Command(evolveBin, "bridge", "launch",
		"--cli="+driver, "--profile="+profile, "--model="+tier,
		"--prompt-file="+promptFile, "--workspace="+ws, "--artifact="+artifact,
		"--worktree="+dir, "--cycle=0", "--allow-bypass",
	)
	cmd.Env = env
	cmd.Dir = dir
	out, runErr := runWithTimeout(cmd, timeout)
	raw, _ := os.ReadFile(artifact) // nil/empty when not written; caller classifies
	return raw, out, runErr
}

// TestParseRoutingPlanArray is a pure-function test (no live gate) that runs
// under `go test -tags e2e` without spending quota. It pins parseRoutingPlanArray's
// tolerance — clean JSON, code fences, and (the regression the review caught)
// trailing prose that contains a stray ']' which a naive LastIndexByte would
// mis-slice.
func TestParseRoutingPlanArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantLen int
		wantErr bool
	}{
		{"clean array", `[{"phase":"scout","run":true,"justification":"x"}]`, 1, false},
		{"json fence", "```json\n[{\"phase\":\"scout\",\"run\":false,\"justification\":\"queued\"}]\n```", 1, false},
		{"bare fence", "```\n[{\"phase\":\"build\",\"run\":true,\"justification\":\"y\"}]\n```", 1, false},
		// The HIGH regression: a valid array followed by prose containing ']'.
		// LastIndexByte(']') would extend the slice into the prose and fail to parse.
		{"trailing prose with stray bracket", `[{"phase":"scout","run":true,"justification":"go"}]` + "\nNote: see phases [scout] and [build].", 1, false},
		{"bracket inside justification", `[{"phase":"build","run":true,"justification":"touches arr[i] and map[k]"}]`, 1, false},
		{"no array", "I could not produce a plan.", 0, true},
		{"empty array parses to zero entries", "[]", 0, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			entries, err := parseRoutingPlanArray([]byte(c.raw))
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got entries=%+v", entries)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(entries) != c.wantLen {
				t.Fatalf("entries=%d, want %d (%+v)", len(entries), c.wantLen, entries)
			}
			if c.wantLen > 0 && entries[0].Phase == "" {
				t.Errorf("first entry has empty phase: %+v", entries[0])
			}
		})
	}
}

// TestStripFrontmatter pins that a genuine YAML block is removed while a body
// without frontmatter is returned unchanged.
func TestStripFrontmatter(t *testing.T) {
	t.Parallel()
	got := stripFrontmatter("---\nname: x\nmodel: tier-3\n---\n\n# Body\ntext")
	if strings.Contains(got, "name: x") || !strings.Contains(got, "# Body") {
		t.Errorf("frontmatter not stripped cleanly: %q", got)
	}
	plain := "# No Frontmatter\njust prose"
	if stripFrontmatter(plain) != plain {
		t.Errorf("body without frontmatter must be unchanged, got %q", stripFrontmatter(plain))
	}
}

func TestE2ELiveAdvisorCLIMatrix(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_ADVISOR")
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)
	timeout := envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 3*time.Minute)

	all := append(append([]liveCLI{}, liveHeadlessCLIs...), liveTmuxCLIs...)
	for _, cli := range all {
		cli := cli
		t.Run(cli.Driver, func(t *testing.T) {
			if ok, why := liveCLIAvailable(cli); !ok {
				t.Skip(why)
			}
			if strings.HasSuffix(cli.Driver, "-tmux") {
				requireTmuxForLive(t)
			}
			if ok, _ := liveBudgetRemaining(); !ok {
				t.Skip("live budget exhausted")
			}

			raw, out, err := launchAdvisor(t, evolveBin, repoRoot, cli.Driver, cli.CheapTier, timeout)
			if err != nil {
				if isTransient(out, err) {
					t.Skipf("%s advisor transient (quarantined):\nerr=%v\n%s", cli.Driver, err, lastN(out, 600))
				}
				t.Fatalf("%s advisor REJECTED (contract break?):\nerr=%v\n%s", cli.Driver, err, lastN(out, 1200))
			}
			if len(raw) == 0 {
				if isTransient(out, err) {
					t.Skipf("%s advisor wrote no artifact but output is transient; quarantining:\n%s", cli.Driver, lastN(out, 600))
				}
				t.Fatalf("%s advisor wrote no routing-plan.json:\n%s", cli.Driver, lastN(out, 1200))
			}
			entries, perr := parseRoutingPlanArray(raw)
			if perr != nil {
				t.Fatalf("%s advisor wrote an UNPARSEABLE routing-plan.json: %v\nartifact=%s", cli.Driver, perr, lastN(string(raw), 1200))
			}
			if len(entries) == 0 {
				t.Fatalf("%s advisor wrote an EMPTY plan (need >=1 phase entry)\nartifact=%s", cli.Driver, lastN(string(raw), 1200))
			}
			if entries[0].Phase == "" {
				t.Errorf("%s advisor: first plan entry has no phase name: %+v", cli.Driver, entries[0])
			}
			t.Logf("[advisor-matrix] %s OK — wrote a %d-entry routing-plan.json at tier=%s", cli.Driver, len(entries), cli.CheapTier)
		})
	}
}
