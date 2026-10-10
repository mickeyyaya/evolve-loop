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

type bridgeOutcome int

const (
	bridgeWritesValid bridgeOutcome = iota
	bridgeWritesNothing
	bridgeWritesNoToken
)

func conformanceOpts(t *testing.T, role string, tokenSeed byte, b bridgeOutcome) RunOptions {
	t.Helper()
	opts := runHappyOpts(t)
	clock := opts.Now
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
			return 0, nil
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

	seen := map[string]string{}
	for role, o := range results {
		if prev, dup := seen[o.token]; dup {
			t.Fatalf("token collision: %s and %s both minted %s", prev, role, o.token)
		}
		seen[o.token] = role
	}

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
