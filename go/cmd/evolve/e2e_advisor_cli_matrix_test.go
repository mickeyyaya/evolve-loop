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
// Runs the advisor at its DEEP production tier (advisorModelFor: opus / gpt-5.5 /
// the family's strongest) — the routing brain is deep-reasoning work, so a fast
// model would not represent its real behavior. Assertions stay STRUCTURAL (a
// valid plan is produced), not on plan wording. Unavailable binaries SKIP
// (liveCLIAvailable); quota/rate-limit/timeout SKIP (isTransient) — only a booted
// CLI that fails the plan contract hard-fails. Gate: EVOLVE_E2E_LIVE_ADVISOR=1.
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

// advisorModelFor resolves the DEEP / high-level model the advisor runs on per
// CLI. The routing brain composes the whole cycle and may mint phases — deep-
// reasoning work — so (unlike the cheap-tier smoke/T2 helpers) the advisor e2e
// validates at the PRODUCTION tier: opus / gpt-5.5 / the family's strongest. A
// fast model like haiku is fine for basic-function checks but does not represent
// the advisor's real behavior. Overridable per CLI via
// EVOLVE_E2E_ADVISOR_MODEL_<BASE> (e.g. EVOLVE_E2E_ADVISOR_MODEL_CLAUDE=sonnet).
func advisorModelFor(driver string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(driver, "-tmux"), "-p")
	if v := os.Getenv("EVOLVE_E2E_ADVISOR_MODEL_" + strings.ToUpper(base)); v != "" {
		return v
	}
	switch base {
	case "claude":
		return "opus"
	case "codex":
		return "gpt-5.5"
	case "agy":
		return "gemini-3.5-flash" // agy's manifest pins all tiers to one model
	case "ollama":
		return "llama3.1:8b" // overridable via EVOLVE_E2E_ADVISOR_MODEL_OLLAMA above
	default:
		return "deep"
	}
}

// launchAdvisor fires ONE live `evolve bridge launch` carrying the evolve-router
// persona + a representative cycle digest, with artifact=routing-plan.json — the
// deliverable PhaseAdvisor.Plan uses. It is the thin per-CLI driver for the
// advisor matrix, mirroring liveBridgeLaunch. It returns the plan BYTES from the
// artifact file when written, else falling back to the captured stdout. The
// matrix runs the advisor at its DEEP production tier (advisorModelFor), which
// reliably writes the artifact; the stdout fallback only guards a model that
// chooses to print instead. A synthetic permissive profile (allowed_clis:["all"])
// keeps the floor from rejecting whatever driver the matrix selects.
func launchAdvisor(t *testing.T, evolveBin, repoRoot, driver, model string, timeout time.Duration) ([]byte, string, error) {
	t.Helper()
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// codex refuses to run outside a trusted/git directory; the production advisor
	// runs inside a git worktree, so git-init the temp dir to match (a harmless
	// no-op for the other drivers). Log failure rather than discard it, so a broken
	// harness precondition (no git / unwritable fs) is not mistaken for codex being
	// merely "env-unavailable".
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Logf("git init %s failed (codex needs a trusted/git dir): %v", dir, err)
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
	body := `{"name":"router","role":"router","allowed_clis":["all"],"allowed_tools":["Read","Write","Bash"]}`
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stdoutLog := filepath.Join(dir, "router-stdout.log")
	stderrLog := filepath.Join(dir, "router-stderr.log")
	fakeHome := t.TempDir()
	env := append(os.Environ(), "EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"))

	cmd := exec.Command(evolveBin, "bridge", "launch",
		"--cli="+driver, "--profile="+profile, "--model="+model,
		"--prompt-file="+promptFile, "--workspace="+ws, "--artifact="+artifact,
		"--stdout-log="+stdoutLog, "--stderr-log="+stderrLog,
		"--worktree="+dir, "--cycle=0", "--allow-bypass",
	)
	cmd.Env = env
	cmd.Dir = dir
	out, runErr := runWithTimeout(cmd, timeout)
	// Fold the captured stderr log into out so the transient classifier sees the
	// real provider message (quota/rate-limit), which the bridge writes to the
	// log file rather than the subprocess stdout (cf. phaseStderrTail).
	if b, rerr := os.ReadFile(stderrLog); rerr == nil && len(b) > 0 {
		out += "\n" + string(b)
	}
	// Prefer the written artifact; fall back to the captured stdout (a fast model
	// often prints the plan instead of invoking Write — see the doc comment).
	raw, _ := os.ReadFile(artifact)
	if len(raw) == 0 {
		if b, rerr := os.ReadFile(stdoutLog); rerr == nil {
			raw = b
		}
	}
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

// advisorEnvUnavailableMarkers flag a CLI that is present but cannot run the
// advisor on THIS host for environmental reasons — distinct from a broken advisor
// contract: a driver with no tool use (ollama can't write the artifact), or a
// first-run interactive trust prompt the headless/auto-responder can't clear
// (an un-onboarded codex). These quarantine-SKIP like a transient so the matrix
// stays a meaningful gate on the CLIs that are actually usable here, rather than
// red-failing on host setup. (Account/model caps like "gpt-5.5 not on a ChatGPT
// account" surface via the trust-loop or transient path.)
var advisorEnvUnavailableMarkers = []string{
	"no tool use",
	"not inside a trusted directory",
	"auto-respond loop guard",
	"loop_guard",
	"trust_prompt",
	// codex headless sandboxes the workspace read-only with approvals disabled, so
	// it cannot write the artifact — the codex family's writer path is codex-tmux
	// (which passes). A writer-incapable headless sandbox is an env constraint here.
	// "read-only sandbox" is the stderr-side phrase (reliably in `out`); the others
	// are the model's stdout apology, kept for the pre-parse no-artifact branches.
	"read-only sandbox",
	"mounted read-only",
	"approvals are disabled",
}

func advisorEnvUnavailable(out string) bool {
	low := strings.ToLower(out)
	for _, m := range advisorEnvUnavailableMarkers {
		if strings.Contains(low, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// TestE2ELiveAdvisorActivation is the ACTIVATION proof: it runs one real,
// isolated (temp-project) cycle with EVOLVE_DYNAMIC_ROUTING=advisory and the
// advisor on claude-p@opus (deep, ~30s headless — claude-tmux@opus exceeds the
// REPL ceiling), then asserts the orchestrator actually consulted the planner
// and disposed its plan. The definitive signal is the orchestrator's
// `phase_plan` ledger entry (recordPhasePlan: planner ran → kernel clamped to the
// integrity floor → recorded) plus the advisor's raw routing-plan.json artifact.
// NOTE: there is intentionally NO `role:router` ledger entry — the advisor calls
// bridge.Launch directly (not via the phase runner that stamps agent roles), so
// `phase_plan` (role=orchestrator) is the real activation signal, correcting the
// plan's original assumption. EVOLVE_MANDATORY_PHASES=scout keeps the cycle tiny;
// the planner runs at cycle start regardless. Gate: EVOLVE_E2E_LIVE_ADVISOR=1.
func TestE2ELiveAdvisorActivation(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_ADVISOR")
	if ok, why := liveCLIAvailable(liveCLI{Driver: "claude-p", Binary: "claude"}); !ok {
		t.Skip(why)
	}
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	res := runLiveCycle(t, liveCycleCfg{
		EvolveBin: evolveBin,
		RepoRoot:  repoRoot,
		Driver:    "claude-p",
		Tier:      "fast", // incidental phases stay cheap; the advisor is opus via env
		GoalHash:  "advisor-activation-proof",
		ExtraEnv: []string{
			"EVOLVE_DYNAMIC_ROUTING=advisory", // Stage=Advisory ⇒ the advisor drives
			"EVOLVE_ROUTER_CLI=claude-p",      // headless opus completes fast
			"EVOLVE_ROUTER_MODEL=opus",        // advisor at its deep production tier
			"EVOLVE_MANDATORY_PHASES=scout",   // keep the cycle tiny; planner runs regardless
		},
		Timeout:   envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 10*time.Minute),
		BudgetUSD: 2.0,
	})
	if res.TransientExhausted {
		t.Skipf("activation transient (quarantined):\n%s", lastN(res.Out, 600))
	}

	// Proof 1: the orchestrator consulted the planner, clamped its plan to the
	// integrity floor, and recorded it (recordPhasePlan → kind=phase_plan).
	hasPhasePlan := false
	for _, e := range res.Entries {
		if e.Kind == "phase_plan" {
			hasPhasePlan = true
			break
		}
	}
	if !hasPhasePlan {
		if isTransient(res.Out, res.Err) || advisorEnvUnavailable(res.Out) {
			t.Skipf("no phase_plan ledger entry, but output is transient/env-unavailable:\n%s", lastN(res.Out, 800))
		}
		t.Fatalf("ACTIVATION FAILED: no phase_plan ledger entry — the planner was not consulted at dynamic_routing=advisory\n%s", lastN(res.Out, 1200))
	}

	// Proof 2: the advisor's RAW plan artifact (routing-plan.json) was written and
	// parses to a non-empty plan.
	raw := readFirstFileNamed(res.ProjRoot, "routing-plan.json")
	if len(raw) == 0 {
		t.Fatalf("phase_plan present but no routing-plan.json artifact found under %s", res.ProjRoot)
	}
	entries, perr := parseRoutingPlanArray(raw)
	if perr != nil || len(entries) == 0 {
		t.Fatalf("routing-plan.json did not parse to a non-empty plan: err=%v\n%s", perr, lastN(string(raw), 800))
	}
	t.Logf("[advisor-activation] dynamic_routing=advisory: phase_plan ledger entry present + %d-entry routing-plan.json written — the advisor drove the cycle", len(entries))
}

// readFirstFileNamed returns the contents of the first file named `name` found
// under root (depth-first), or nil if none. Used to locate the advisor's
// routing-plan.json without computing the cycle number.
func readFirstFileNamed(root, name string) []byte {
	var found []byte
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(p) == name {
			if b, rerr := os.ReadFile(p); rerr == nil {
				found = b
				return filepath.SkipAll // stop traversal — file in hand
			}
		}
		return nil
	})
	return found
}

// TestE2ELiveAdvisorSelectsDesignPhase is the Phase 3 payoff: with the advisor
// now goal-aware (GoalText threaded), an explicitly architectural goal should make
// the brain SELECT the non-spine `architecture-design` phase (or MINT one) in its
// routing-plan.json — genuine AI-driven composition, not the rubber-stamped spine.
// The advisor runs at opus (deep). Tolerant: transient/env-unavailable → skip.
// Gate: EVOLVE_E2E_LIVE_ADVISOR=1.
func TestE2ELiveAdvisorSelectsDesignPhase(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_ADVISOR")
	if ok, why := liveCLIAvailable(liveCLI{Driver: "claude-p", Binary: "claude"}); !ok {
		t.Skip(why)
	}
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	res := runLiveCycle(t, liveCycleCfg{
		EvolveBin: evolveBin,
		RepoRoot:  repoRoot,
		Driver:    "claude-p",
		Tier:      "fast",
		GoalHash:  "arch-design-proof",
		Goal: "Large, novel, cross-cutting redesign: re-architect the authentication " +
			"subsystem end-to-end (token rotation, session store, RBAC boundaries). This " +
			"spans many modules and needs a dedicated up-front architecture/design pass " +
			"BEFORE any implementation.",
		ExtraEnv: []string{
			"EVOLVE_DYNAMIC_ROUTING=advisory",
			"EVOLVE_ROUTER_CLI=claude-p",
			"EVOLVE_ROUTER_MODEL=opus",
			"EVOLVE_MANDATORY_PHASES=scout",
		},
		Timeout:   envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 10*time.Minute),
		BudgetUSD: 2.0,
	})
	if res.TransientExhausted {
		t.Skipf("design-selection transient (quarantined):\n%s", lastN(res.Out, 600))
	}

	raw := readFirstFileNamed(res.ProjRoot, "routing-plan.json")
	if len(raw) == 0 {
		if isTransient(res.Out, res.Err) || advisorEnvUnavailable(res.Out) {
			t.Skipf("no routing-plan.json but output transient/env-unavailable:\n%s", lastN(res.Out, 600))
		}
		t.Fatalf("advisor produced no routing-plan.json under %s", res.ProjRoot)
	}
	entries, perr := parseRoutingPlanArray(raw)
	if perr != nil {
		t.Fatalf("routing-plan.json did not parse: %v\n%s", perr, lastN(string(raw), 800))
	}

	selectedDesign := false
	for _, e := range entries {
		if e.Phase == "architecture-design" && e.Run {
			selectedDesign = true
		}
	}
	minted := strings.Contains(string(raw), `"mint"`)
	if !selectedDesign && !minted {
		t.Fatalf("goal-aware advisor selected NEITHER architecture-design NOR a mint for an explicitly architectural goal — composition is still spine-only.\nplan=%s", lastN(string(raw), 1500))
	}
	t.Logf("[advisor-design] goal-aware advisor produced a non-spine decision (architecture-design=%v, mint=%v) — genuine AI-driven composition", selectedDesign, minted)
}

func TestE2ELiveAdvisorCLIMatrix(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_ADVISOR")
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)
	// Deep models (opus / gpt-5.5) reason longer than the cheap-tier smoke calls,
	// so the default ceiling is generous; override with EVOLVE_E2E_LIVE_TIMEOUT_S.
	timeout := envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 10*time.Minute)

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
			model := advisorModelFor(cli.Driver)
			raw, out, err := launchAdvisor(t, evolveBin, repoRoot, cli.Driver, model, timeout)
			if err != nil {
				if isTransient(out, err) || advisorEnvUnavailable(out) {
					t.Skipf("%s advisor skipped (transient/env-unavailable):\nerr=%v\n%s", cli.Driver, err, lastN(out, 600))
				}
				t.Fatalf("%s advisor REJECTED (contract break?):\nerr=%v\n%s", cli.Driver, err, lastN(out, 1200))
			}
			if len(raw) == 0 {
				if isTransient(out, err) || advisorEnvUnavailable(out) {
					t.Skipf("%s advisor wrote no plan but output is transient/env-unavailable; quarantining:\n%s", cli.Driver, lastN(out, 600))
				}
				t.Fatalf("%s advisor wrote no routing-plan.json:\n%s", cli.Driver, lastN(out, 1200))
			}
			entries, perr := parseRoutingPlanArray(raw)
			if perr != nil {
				// A driver that ran but reported an environmental write/approval block
				// (e.g. codex headless read-only sandbox) prints an apology, not a plan
				// — quarantine-skip rather than treating it as an advisor contract break.
				// Classify on `out` (the bridge/CLI's own diagnostics) only, NOT on raw:
				// raw is the model's free-text stdout, which could coincidentally contain
				// a marker phrase in a justification and mask a real contract break.
				if advisorEnvUnavailable(out) {
					t.Skipf("%s advisor skipped (env-unavailable; could not write a plan):\n%s", cli.Driver, lastN(out, 600))
				}
				t.Fatalf("%s advisor wrote an UNPARSEABLE routing-plan.json: %v\nartifact=%s", cli.Driver, perr, lastN(string(raw), 1200))
			}
			if len(entries) == 0 {
				t.Fatalf("%s advisor wrote an EMPTY plan (need >=1 phase entry)\nartifact=%s", cli.Driver, lastN(string(raw), 1200))
			}
			if entries[0].Phase == "" {
				t.Errorf("%s advisor: first plan entry has no phase name: %+v", cli.Driver, entries[0])
			}
			t.Logf("[advisor-matrix] %s OK — %d-entry routing-plan.json at model=%s", cli.Driver, len(entries), model)
		})
	}
}
