package subagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// conformance_test.go — the subagent module conformance suite (plan B4).
//
// Every phase-agent role in the registry (agentRoles, run.go) must satisfy the
// SAME dispatch invariants: it goes through the bridge (never the in-process
// tool), its artifact is judged by the one verification SSOT (contract.go), its
// recursion command is sandbox-coherent (CLAUDECODE_TYPE cleared, depth
// threaded), its recursion is depth-bounded, and concurrent independent
// dispatches stay isolated (unique token, own artifact, no cross-leak, no race).
//
// The suite is driven by a FAKE BRIDGE — the RunOptions ExecAdapter seam, which
// stands in for `evolve subagent run` with no LLM cost — and is table-driven
// over agentRoles, so adding a role to the registry auto-subjects it to every
// invariant below.

// bridgeOutcome selects what the stand-in bridge "produces" for a dispatch.
type bridgeOutcome int

const (
	bridgeWritesValid   bridgeOutcome = iota // fresh token-bearing artifact → PASS
	bridgeWritesNothing                      // exits 0 but no artifact      → INTEGRITY_FAIL
	bridgeWritesNoToken                      // fresh artifact, token absent  → INTEGRITY_FAIL
)

// conformanceOpts builds a fake-bridge RunOptions for one role. It reuses the
// canonical runHappyOpts seams and overrides only (a) the profile so each role
// writes a role-named artifact, (b) Rand so each role mints a distinct token,
// and (c) the ExecAdapter to model the chosen bridge outcome.
func conformanceOpts(t *testing.T, role string, tokenSeed byte, b bridgeOutcome) RunOptions {
	t.Helper()
	opts := runHappyOpts(t)
	clock := opts.Now // the fixed clock runHappyOpts pinned
	opts.ReadProfile = func(string) (string, error) {
		return fmt.Sprintf(
			`{"role":%q,"cli":"claude","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/%s.md"}`,
			role, role), nil
	}
	opts.Rand = func(buf []byte) (int, error) {
		for i := range buf {
			buf[i] = tokenSeed
		}
		return len(buf), nil
	}
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		path := env["ARTIFACT_PATH"]
		if b == bridgeWritesNothing || path == "" {
			return 0, nil // "succeeded" but produced no artifact
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return 1, err
		}
		body := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\nbody\n"
		if b == bridgeWritesNoToken {
			body = "this artifact bears no challenge token\n"
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return 1, err
		}
		now := clock()
		_ = os.Chtimes(path, now, now)
		return 0, nil
	}
	return opts
}

// conformanceReq builds a standard valid RunRequest for a role, with an
// isolated project root + workspace (the workspace dir is created so Run's
// existence check passes).
func conformanceReq(t *testing.T, role string) RunRequest {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	return RunRequest{
		Agent:            role,
		Cycle:            5,
		WorkspacePath:    ws,
		ProfilesDir:      "/p",
		AdaptersDir:      "/a",
		ProjectRoot:      root,
		PluginRoot:       root,
		PromptReader:     strings.NewReader("Do the thing.\n"),
		AdversarialAudit: true,
	}
}

// TestConformance_AllRoles_BridgeOnly — every role rejects the retired
// in-process dispatch hatch (B1 invariant, uniformly across the registry).
func TestConformance_AllRoles_BridgeOnly(t *testing.T) {
	t.Parallel()
	for i, role := range agentRoles {
		role := role
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			req := conformanceReq(t, role)
			req.LegacyAgentDispatch = true
			_, err := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), bridgeWritesValid))
			if err == nil || !strings.Contains(err.Error(), "in-process dispatch") {
				t.Fatalf("role %s: want ErrInProcessDispatchBanned, got %v", role, err)
			}
		})
	}
}

// TestConformance_AllRoles_HappyDispatchIsolated — every role dispatches
// through the fake bridge to a PASS verdict, with a non-empty token bound into
// its own artifact under its own project root.
func TestConformance_AllRoles_HappyDispatchIsolated(t *testing.T) {
	t.Parallel()
	for i, role := range agentRoles {
		role := role
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			req := conformanceReq(t, role)
			res, err := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), bridgeWritesValid))
			if err != nil {
				t.Fatalf("role %s: unexpected error %v", role, err)
			}
			if res.Verdict != VerdictPASS {
				t.Fatalf("role %s: verdict=%s, want PASS", role, res.Verdict)
			}
			if res.ChallengeToken == "" {
				t.Fatalf("role %s: empty challenge token", role)
			}
			if !strings.HasPrefix(res.ArtifactPath, req.ProjectRoot) {
				t.Fatalf("role %s: artifact %s not under project root %s", role, res.ArtifactPath, req.ProjectRoot)
			}
			body, err := os.ReadFile(res.ArtifactPath)
			if err != nil {
				t.Fatalf("role %s: read artifact: %v", role, err)
			}
			if !strings.Contains(string(body), res.ChallengeToken) {
				t.Fatalf("role %s: artifact does not bear its challenge token", role)
			}
		})
	}
}

// TestConformance_AllRoles_ContractGuards — the B3 verification SSOT guards
// every role: a bridge that produces no artifact, or one without the token,
// yields INTEGRITY_FAIL (never a false PASS).
func TestConformance_AllRoles_ContractGuards(t *testing.T) {
	t.Parallel()
	for i, role := range agentRoles {
		role := role
		for _, tc := range []struct {
			name   string
			bridge bridgeOutcome
		}{
			{"no artifact", bridgeWritesNothing},
			{"no token", bridgeWritesNoToken},
		} {
			tc := tc
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				req := conformanceReq(t, role)
				res, _ := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), tc.bridge))
				if res.Verdict != VerdictIntegrityFail {
					t.Fatalf("role %s (%s): verdict=%s, want INTEGRITY_FAIL", role, tc.name, res.Verdict)
				}
			})
		}
	}
}

// TestConformance_AllRoles_RecursionSandboxCoherent — every role's fan-out
// worker command re-enters the bridge (`subagent run <role>-worker-<subtask>`),
// clears the host marker (CLAUDECODE_TYPE=) so the child stays nested (no inner
// sandbox wrap — B2/Part A), and threads the child recursion depth.
func TestConformance_AllRoles_RecursionSandboxCoherent(t *testing.T) {
	t.Parallel()
	for _, role := range agentRoles {
		role := role
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			cmd := buildWorkerRecursionCommand("/bin/evolve", role, "sub1", 7, 1, "/ws", "/p.md", "wtok-worker-sub1")
			if !strings.Contains(cmd, "CLAUDECODE_TYPE= ") {
				t.Errorf("role %s: worker command must clear CLAUDECODE_TYPE: %s", role, cmd)
			}
			if !strings.Contains(cmd, dispatchDepthEnv+"=1") {
				t.Errorf("role %s: worker command must thread child depth: %s", role, cmd)
			}
			if !strings.Contains(cmd, "subagent run "+role+"-worker-sub1 ") {
				t.Errorf("role %s: worker command must re-enter the bridge dispatch path: %s", role, cmd)
			}
		})
	}
}

// TestConformance_AllRoles_DepthCapEnforced — every role rejects a dispatch that
// runs deeper than the recursion cap (B2), so a fan-out loop can't recurse
// unboundedly regardless of which role it spawns.
func TestConformance_AllRoles_DepthCapEnforced(t *testing.T) {
	t.Parallel()
	for i, role := range agentRoles {
		role := role
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			req := conformanceReq(t, role)
			req.DispatchDepth = maxDispatchDepth + 1
			_, err := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), bridgeWritesValid))
			if err == nil || !strings.Contains(err.Error(), "recursion depth cap") {
				t.Fatalf("role %s: want ErrRecursionDepthExceeded, got %v", role, err)
			}
		})
	}
}

// TestConformance_NoCrossAgentTokenLeakage — dispatching every role mints a
// distinct token, and each role's artifact bears only its own token, never
// another role's. Proves dispatches don't share token/artifact state.
func TestConformance_NoCrossAgentTokenLeakage(t *testing.T) {
	t.Parallel()
	type out struct {
		token string
		body  string
	}
	results := make(map[string]out, len(agentRoles))
	for i, role := range agentRoles {
		req := conformanceReq(t, role)
		res, err := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), bridgeWritesValid))
		if err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		body, err := os.ReadFile(res.ArtifactPath)
		if err != nil {
			t.Fatalf("role %s: read artifact: %v", role, err)
		}
		results[role] = out{token: res.ChallengeToken, body: string(body)}
	}

	// Tokens are pairwise distinct.
	seen := map[string]string{}
	for role, o := range results {
		if prev, dup := seen[o.token]; dup {
			t.Fatalf("token collision: %s and %s both minted %s", prev, role, o.token)
		}
		seen[o.token] = role
	}

	// No role's artifact contains a different role's token.
	for role, o := range results {
		for other, oo := range results {
			if other == role {
				continue
			}
			if strings.Contains(o.body, oo.token) {
				t.Fatalf("cross-agent leak: %s artifact contains %s token %s", role, other, oo.token)
			}
		}
	}
}

// TestConformance_ConcurrentDispatch_NoRace — every role dispatched concurrently
// with isolated project roots/workspaces all reach PASS with no data race (run
// under -race). Pins that independent dispatches share no mutable state.
func TestConformance_ConcurrentDispatch_NoRace(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	errs := make([]error, len(agentRoles))
	verdicts := make([]string, len(agentRoles))
	for i, role := range agentRoles {
		wg.Add(1)
		go func(i int, role string) {
			defer wg.Done()
			req := conformanceReq(t, role)
			res, err := Run(context.Background(), req, conformanceOpts(t, role, byte(i+1), bridgeWritesValid))
			errs[i] = err
			verdicts[i] = res.Verdict
		}(i, role)
	}
	wg.Wait()
	for i, role := range agentRoles {
		if errs[i] != nil {
			t.Errorf("role %s: concurrent dispatch error: %v", role, errs[i])
		}
		if verdicts[i] != VerdictPASS {
			t.Errorf("role %s: concurrent verdict=%s, want PASS", role, verdicts[i])
		}
	}
}
