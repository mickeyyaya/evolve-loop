package subagent

// run_unit16_pins_test.go — ADR-0103 unit 16 step 1: the behavioural pins the
// `evolve subagent run` execution path had never carried, written GREEN on the
// pre-extraction code (8e8f080f) and each proven red against its named mutant
// before the path moved into internal/subagent/subagentrun. They drive the
// production entry Run, so after the move they are the strangler proof: the
// facade over the leaf is byte-identical on the prompt, the adapter env, the
// Warns channel, every error text, the verdict wiring and the ledger line.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// unit16Goldens is where the pre-extraction goldens live (captured by a
// throwaway writer on 8e8f080f, paths templated as {WS} / {ROOT} / {WORKTREE}).
const unit16Goldens = "subagentrun/testdata"

func unit16Golden(t *testing.T, name string, pairs ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(unit16Goldens, name))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, pairs[i], pairs[i+1])
	}
	return s
}

// unit16Dirs is the fixture layout every pin shares: a workspace and a
// worktree under one root.
func unit16Dirs(t *testing.T) (root, ws, worktree string) {
	t.Helper()
	root = t.TempDir()
	ws, worktree = filepath.Join(root, "ws"), filepath.Join(root, "wt")
	for _, d := range []string{ws, worktree} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, ws, worktree
}

// unit16Capturing wraps the fixture's adapter so the delivered prompt and the
// adapter env are observable.
func unit16Capturing(opts *RunOptions, prompt *string, env *map[string]string) {
	orig := opts.ExecAdapter
	opts.ExecAdapter = func(ctx context.Context, adapter string, e map[string]string) (int, error) {
		if prompt != nil {
			b, _ := os.ReadFile(e["PROMPT_FILE"])
			*prompt = string(b)
		}
		if env != nil {
			*env = e
		}
		return orig(ctx, adapter, e)
	}
}

// unit16EnvJSON renders the adapter env the way the golden writer did: sorted
// keys, PROMPT_FILE and the fixture paths templated.
func unit16EnvJSON(env map[string]string, pairs ...string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		v := env[k]
		if k == "PROMPT_FILE" {
			v = "{PROMPT_FILE}"
		}
		for j := 0; j+1 < len(pairs); j += 2 {
			v = strings.ReplaceAll(v, pairs[j], pairs[j+1])
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(v)
		b.WriteString("  " + string(kb) + ": " + string(vb))
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// unit16Auditor is runHappyOpts over an auditor profile.
func unit16Auditor(t *testing.T) RunOptions {
	t.Helper()
	o := runHappyOpts(t)
	o.ReadProfile = func(string) (string, error) {
		return `{"role":"auditor","cli":"claude","model_tier_default":"opus","output_artifact":".evolve/runs/cycle-{cycle}/audit.md"}`, nil
	}
	return o
}

// unit16Artifact writes a sound artifact for the adapter stub: the token in
// the first line, mtime = the fixture clock.
func unit16Artifact(t *testing.T, path, token string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<!-- challenge-token: "+token+" -->\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// Test 1 — the six admission checks fire in a fixed order: prompt reader,
// role, cycle, workspace, the legacy escape hatch, the recursion depth.
func TestRun_AdmissionOrderIsFixed(t *testing.T) {
	_, ws, worktree := unit16Dirs(t)
	req := RunRequest{Agent: "bogus", Cycle: -1, WorkspacePath: "/non/existent", LegacyAgentDispatch: true, DispatchDepth: 99, WorktreePath: worktree}
	fix := []struct {
		name string
		fix  func(*RunRequest)
		want string
		is   error
	}{
		{"prompt reader", func(*RunRequest) {}, "subagent/run: PromptReader required (PROMPT_FILE_OVERRIDE or stdin)", nil},
		{"unknown agent", func(r *RunRequest) { r.PromptReader = strings.NewReader("hi") }, "subagent/run: unknown agent: bogus", nil},
		{"cycle", func(r *RunRequest) { r.Agent = "scout" }, "subagent/run: cycle must be >= 0, got -1", nil},
		{"workspace", func(r *RunRequest) { r.Cycle = 0 }, "subagent/run: workspace dir does not exist: /non/existent", nil},
		{"legacy dispatch", func(r *RunRequest) { r.WorkspacePath = ws }, "", ErrInProcessDispatchBanned},
		{"depth", func(r *RunRequest) { r.LegacyAgentDispatch = false }, "", ErrRecursionDepthExceeded},
	}
	for _, step := range fix {
		step.fix(&req)
		_, err := Run(context.Background(), req, runHappyOpts(t))
		if err == nil {
			t.Fatalf("%s: expected a rejection", step.name)
		}
		if step.is != nil && !errors.Is(err, step.is) {
			t.Fatalf("%s: got %v, want the sentinel %v", step.name, err, step.is)
		}
		if step.want != "" && err.Error() != step.want {
			t.Fatalf("%s: got %q, want %q", step.name, err.Error(), step.want)
		}
	}
	req.DispatchDepth = 0
	if res, err := Run(context.Background(), req, runHappyOpts(t)); err != nil || res.Verdict != VerdictPASS {
		t.Fatalf("every check satisfied ⇒ the run proceeds: %v %+v", err, res)
	}
}

// Test 2 — ONE token reaches the prompt, the adapter env, the result and the
// ledger line.
func TestRun_OneTokenReachesPromptEnvVerifyAndLedger(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	ledger := filepath.Join(root, "ledger.jsonl")
	opts := runHappyOpts(t)
	var prompt string
	var env map[string]string
	unit16Capturing(&opts, &prompt, &env)
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: ledger, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil {
		t.Fatal(err)
	}
	const token = "aaaaaaaaaaaaaaaa"
	if res.ChallengeToken != token || env["CHALLENGE_TOKEN"] != token || !strings.Contains(prompt, "- Challenge token: "+token+"\n") {
		t.Fatalf("one token everywhere: result %q env %q prompt %q", res.ChallengeToken, env["CHALLENGE_TOKEN"], prompt)
	}
	line, _ := os.ReadFile(ledger)
	if !strings.Contains(string(line), `"challenge_token":"`+token+`"`) {
		t.Fatalf("the ledger carries the same token: %s", line)
	}
}

// Test 3 — the prompt temp file exists (0600, the pattern, the full prompt)
// while the adapter runs and is gone afterwards.
func TestRun_PromptTempfileLivesOnlyDuringExec(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	opts := runHappyOpts(t)
	var promptPath string
	orig := opts.ExecAdapter
	opts.ExecAdapter = func(ctx context.Context, adapter string, env map[string]string) (int, error) {
		promptPath = env["PROMPT_FILE"]
		info, err := os.Stat(promptPath)
		if err != nil {
			t.Fatalf("the prompt file exists during exec: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("mode %v, want 0600", info.Mode().Perm())
		}
		base := filepath.Base(promptPath)
		if !strings.HasPrefix(base, "evolve-subagent-prompt-") || !strings.HasSuffix(base, ".txt") {
			t.Errorf("name pattern drifted: %s", base)
		}
		b, _ := os.ReadFile(promptPath)
		if !strings.HasPrefix(string(b), "## INVOCATION CONTEXT\n") || !strings.HasSuffix(string(b), "--- END TASK PROMPT ---\n") {
			t.Errorf("the full prompt is staged: %q", b)
		}
		return orig(ctx, adapter, env)
	}
	if _, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
		t.Fatalf("the prompt file is removed after the run: %v", err)
	}
}

// Test 4 — the composed prompt is byte-identical to the five goldens captured
// on the pre-extraction code.
func TestRun_ComposedPromptIsByteIdenticalToTheGolden(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	cases := []struct {
		golden string
		req    RunRequest
		opts   RunOptions
	}{
		{"prompt-plain.golden.txt", RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("Do the thing.\n"), AdversarialAudit: true}, runHappyOpts(t)},
		{"prompt-auditor.golden.txt", RunRequest{Agent: "auditor", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("audit body\n"), AdversarialAudit: true}, unit16Auditor(t)},
		{"prompt-auditor-off.golden.txt", RunRequest{Agent: "auditor", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("audit body\n"), AdversarialAudit: false}, unit16Auditor(t)},
		{"prompt-worker.golden.txt", RunRequest{Agent: "scout-worker-codebase", Cycle: 3, WorkspacePath: ws, WorktreePath: worktree, PromptReader: strings.NewReader("worker body\n"), AdversarialAudit: true}, runHappyOpts(t)},
		{"prompt-no-newline.golden.txt", RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("no trailing nl"), AdversarialAudit: true}, runHappyOpts(t)},
	}
	for _, c := range cases {
		var prompt string
		unit16Capturing(&c.opts, &prompt, nil)
		if _, err := Run(context.Background(), c.req, c.opts); err != nil {
			t.Fatalf("%s: %v", c.golden, err)
		}
		if want := unit16Golden(t, c.golden, "{WS}", ws, "{WORKTREE}", worktree, "{ROOT}", root); prompt != want {
			t.Errorf("%s drifted:\n got %q\nwant %q", c.golden, prompt, want)
		}
	}
}

// Test 5 — the adapter env is byte-identical to the two goldens: 17 keys with
// a project root, 16 without (EVOLVE_PROJECT_ROOT absent, never empty).
func TestRun_AdapterEnvIsByteIdenticalToTheGolden(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	pairs := []string{ws, "{WS}", worktree, "{WORKTREE}", root, "{ROOT}"}
	var env map[string]string
	opts := runHappyOpts(t)
	unit16Capturing(&opts, nil, &env)
	if _, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("Do the thing.\n"), AdversarialAudit: true}, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := unit16EnvJSON(env, pairs...), unit16Golden(t, "adapter-env-root.golden.json"); got != want || len(env) != 17 {
		t.Errorf("the 17-key env drifted (%d keys):\n got %s\nwant %s", len(env), got, want)
	}
	opts = runHappyOpts(t)
	unit16Capturing(&opts, nil, &env)
	if _, err := Run(context.Background(), RunRequest{Agent: "scout-worker-codebase", Cycle: 3, WorkspacePath: ws, WorktreePath: worktree, PromptReader: strings.NewReader("worker body\n"), AdversarialAudit: true}, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := unit16EnvJSON(env, pairs...), unit16Golden(t, "adapter-env-noroot.golden.json"); got != want || len(env) != 16 {
		t.Errorf("the 16-key env drifted (%d keys):\n got %s\nwant %s", len(env), got, want)
	}
}

// Test 6 — the framing keys on the parsed ROLE, so an auditor worker gets it
// and a scout worker does not.
func TestRun_AuditorWorkerGetsTheFraming(t *testing.T) {
	_, ws, worktree := unit16Dirs(t)
	for agent, want := range map[string]bool{"auditor-worker-deep": true, "scout-worker-codebase": false} {
		opts := unit16Auditor(t)
		var prompt string
		unit16Capturing(&opts, &prompt, nil)
		if _, err := Run(context.Background(), RunRequest{Agent: agent, Cycle: 5, WorkspacePath: ws, WorktreePath: worktree, PromptReader: strings.NewReader("body\n"), AdversarialAudit: true}, opts); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(prompt, "ADVERSARIAL AUDIT MODE"); got != want {
			t.Errorf("%s: framing present=%v, want %v", agent, got, want)
		}
	}
}

// Test 7 — the four clock reads are start, end, verify, ledger ts: the
// duration brackets only the adapter call and duration_s truncates.
func TestRun_DurationBracketsOnlyTheAdapterCall(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	ledger := filepath.Join(root, "ledger.jsonl")
	t0 := time.Date(2026, 5, 23, 17, 0, 0, 0, time.UTC)
	ticks := []time.Time{t0, t0.Add(1999 * time.Millisecond), t0.Add(time.Hour), t0.Add(time.Hour)}
	call := 0
	opts := runHappyOpts(t)
	opts.Now = func() time.Time {
		if call >= len(ticks) {
			t.Fatalf("a fifth clock read")
		}
		call++
		return ticks[call-1]
	}
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		unit16Artifact(t, env["ARTIFACT_PATH"], env["CHALLENGE_TOKEN"], t0.Add(time.Hour)) // fresh at the verify read
		return 0, nil
	}
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: ledger, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.DurationMS != 1999 || res.Verdict != VerdictPASS || call != 4 {
		t.Fatalf("duration %d verdict %s reads %d", res.DurationMS, res.Verdict, call)
	}
	line, _ := os.ReadFile(ledger)
	if !strings.Contains(string(line), `"duration_s":"1"`) || !strings.Contains(string(line), `"ts":"2026-05-23T18:00:00Z"`) {
		t.Fatalf("duration_s truncates and ts is the fourth read: %s", line)
	}
}

// Test 8 — the Warns channel is the capability warns, then the fallback
// sentence (golden), only when the worktree fell back.
func TestRun_WarnsOrderCapabilityThenFallbackTextPinned(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	opts := runHappyOpts(t)
	opts.InspectCapability = func(string, string) (capability.Inspection, error) {
		return capability.Inspection{Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true}, Warns: []string{"capA"}}, nil
	}
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 7, WorkspacePath: ws, ProjectRoot: root, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(unit16Golden(t, "warn-worktree-fallback.golden.txt", "{ROOT}", root), "\n")
	if len(res.Warns) != 2 || res.Warns[0] != "capA" || res.Warns[1] != want {
		t.Fatalf("warns: %q, want [capA, %q]", res.Warns, want)
	}
	res, err = Run(context.Background(), RunRequest{Agent: "scout", Cycle: 7, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warns) != 1 || res.Warns[0] != "capA" {
		t.Fatalf("with a worktree only the capability warn: %q", res.Warns)
	}
}

// Test 9 — a ledger write error masks the exec error; the result is populated.
func TestRun_LedgerWriteErrorMasksTheExecError(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := runHappyOpts(t)
	clock := opts.Now
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		unit16Artifact(t, env["ARTIFACT_PATH"], env["CHALLENGE_TOKEN"], clock())
		return -1, errors.New("adapter crashed")
	}
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: filepath.Join(blocker, "sub", "ledger.jsonl"), PromptReader: strings.NewReader("hi\n")}, opts)
	if err == nil || !strings.HasPrefix(err.Error(), "subagent/run: ledger write: ") || strings.Contains(err.Error(), "adapter exec") {
		t.Fatalf("the ledger error masks the exec error: %v", err)
	}
	if res.ExitCode != -1 || res.Verdict != VerdictFAIL {
		t.Fatalf("the result is populated: %+v", res)
	}
}

// Test 10 (characterisation) — the profile read error drops its cause.
func TestRun_ProfileReadErrorTextDropsItsCause(t *testing.T) {
	_, ws, worktree := unit16Dirs(t)
	opts := runHappyOpts(t)
	opts.ReadProfile = func(p string) (string, error) { return "", &os.PathError{Op: "open", Path: p, Err: syscall.EACCES} }
	_, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProfilesDir: "/p", WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts)
	if err == nil || err.Error() != "subagent/run: profile not found: /p/scout.json" {
		t.Fatalf("the text carries no cause: %v", err)
	}
}

// Test 11 — an erroring or empty git state stamps "unknown" into the line.
func TestRun_GitStateErrorStampsUnknown(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	for name, c := range map[string]struct {
		state func(context.Context, string) (string, string, error)
		want  string
	}{
		"error":      {func(context.Context, string) (string, string, error) { return "", "", errors.New("no git") }, `"git_head":"unknown","tree_state_sha":"unknown"`},
		"empty diff": {func(context.Context, string) (string, string, error) { return "h", "", nil }, `"git_head":"h","tree_state_sha":"unknown"`},
	} {
		ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
		opts := runHappyOpts(t)
		opts.GitState = c.state
		if _, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: ledger, PromptReader: strings.NewReader("hi\n")}, opts); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		line, _ := os.ReadFile(ledger)
		if !strings.Contains(string(line), c.want) {
			t.Errorf("%s: %s lacks %s", name, line, c.want)
		}
	}
}

// Test 12 — a hash error leaves artifact_sha256 empty and the run green.
func TestRun_HashErrorLeavesSHAEmptyInTheLedger(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	ledger := filepath.Join(root, "ledger.jsonl")
	opts := runHappyOpts(t)
	opts.HashFile = func(string) (string, error) { return "", errors.New("hash boom") }
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: ledger, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil || res.Verdict != VerdictPASS || res.ArtifactSHA256 != "" {
		t.Fatalf("%v %+v", err, res)
	}
	line, _ := os.ReadFile(ledger)
	if !strings.Contains(string(line), `"artifact_sha256":""`) {
		t.Fatalf("the line stamps an empty sha: %s", line)
	}
}

// Test 13 (characterisation, Q1) — an empty output_artifact template proceeds
// to INTEGRITY_FAIL with an empty artifact path in the ledger line.
func TestRun_EmptyArtifactTemplateProceedsToIntegrityFail(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	ledger := filepath.Join(root, "ledger.jsonl")
	opts := runHappyOpts(t)
	opts.ReadProfile = func(string) (string, error) {
		return `{"role":"scout","cli":"claude","model_tier_default":"sonnet"}`, nil
	}
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, LedgerPath: ledger, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil || res.ArtifactPath != "" || res.Verdict != VerdictIntegrityFail {
		t.Fatalf("%v %+v", err, res)
	}
	line, _ := os.ReadFile(ledger)
	if !strings.Contains(string(line), `"artifact_path":""`) {
		t.Fatalf("the line carries the empty path: %s", line)
	}
}

// Test 14 — the verdict wiring per rung through the adapter stub.
func TestRun_VerdictWiringPerRung(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	clock := fixedClock(t, "2026-05-23T17:00:00Z")
	cases := []struct {
		name    string
		adapter func(path, token string) (int, error)
		verdict string
		exit    int
		execErr bool
	}{
		{"missing", func(string, string) (int, error) { return 0, nil }, VerdictIntegrityFail, 0, false},
		{"stale", func(p, tok string) (int, error) {
			unit16Artifact(t, p, tok, clock().Add(-6*time.Minute))
			return 0, nil
		}, VerdictIntegrityFail, 0, false},
		{"empty", func(p, _ string) (int, error) {
			unit16Artifact(t, p, "", clock())
			_ = os.WriteFile(p, nil, 0o644)
			_ = os.Chtimes(p, clock(), clock())
			return 0, nil
		}, VerdictIntegrityFail, 0, false},
		{"token missing", func(p, _ string) (int, error) { unit16Artifact(t, p, "other", clock()); return 0, nil }, VerdictIntegrityFail, 0, false},
		{"sound exit 3", func(p, tok string) (int, error) { unit16Artifact(t, p, tok, clock()); return 3, nil }, VerdictFAIL, 3, false},
		{"sound exec error", func(p, tok string) (int, error) { unit16Artifact(t, p, tok, clock()); return -1, errors.New("boom") }, VerdictFAIL, -1, true},
	}
	for _, c := range cases {
		opts := runHappyOpts(t)
		opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
			return c.adapter(env["ARTIFACT_PATH"], env["CHALLENGE_TOKEN"])
		}
		res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: t.TempDir(), WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts)
		if res.Verdict != c.verdict || res.ExitCode != c.exit {
			t.Errorf("%s: verdict %s exit %d, want %s %d", c.name, res.Verdict, res.ExitCode, c.verdict, c.exit)
		}
		if c.execErr != (err != nil && strings.Contains(err.Error(), "subagent/run: adapter exec: ")) {
			t.Errorf("%s: err %v", c.name, err)
		}
	}
	_ = root
}

// Test 15 (the ledger goldens) relocated to the leaf as
// subagentrun.TestLedger_LineGoldenChainAndRunID with the writer it pins.

// Test 16 (characterisation) — an LLM resolver error falls back to the
// profile's cli with source=profile and no Warns entry.
func TestRun_LLMResolverErrorFallsBackWithoutWarns(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) { return resolvellm.Result{}, errors.New("no llm") }
	var env map[string]string
	unit16Capturing(&opts, nil, &env)
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil || res.CLI != "claude" || env["CLI_RESOLUTION_SOURCE"] != "profile" || len(res.Warns) != 0 {
		t.Fatalf("%v %+v %q", err, res, env["CLI_RESOLUTION_SOURCE"])
	}
}
