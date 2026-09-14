package subagentrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// envJSON renders an adapter env the way the golden writer did: sorted keys,
// PROMPT_FILE and the fixture paths templated.
func envJSON(env map[string]string, pairs ...string) string {
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

// Test 35 — AdapterEnv.Map reproduces the two goldens (17 keys with a project
// root, 16 without — the key absent, never empty) and never leaks AdapterPath.
func TestEnv_MapIsGoldenAndOmitsProjectRootWhenEmpty(t *testing.T) {
	f := newFixture(t)
	pairs := []string{f.ws, "{WS}", f.worktree, "{WORKTREE}", f.root, "{ROOT}"}
	var env AdapterEnv
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(ctx context.Context, e AdapterEnv) (int, error) { env = e; return soundAdapter(t).Exec(ctx, e) })
	d, _ := observed(t, deps)
	req := f.request()
	req.ProfilesDir, req.AdversarialAudit = "", true
	if _, err := d.Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	m := env.Map()
	if got, want := envJSON(m, pairs...), golden(t, "adapter-env-root.golden.json"); got != want || len(m) != 17 || env.AdapterPath != "/a/claude.sh" {
		t.Errorf("the 17-key env (%d keys, adapter %q):\n got %s\nwant %s", len(m), env.AdapterPath, got, want)
	}
	req.Agent, req.Cycle, req.ProjectRoot = "scout-worker-codebase", 3, ""
	if _, err := d.Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	m = env.Map()
	if _, present := m["EVOLVE_PROJECT_ROOT"]; present {
		t.Fatal("an empty project root produces no key")
	}
	if got, want := envJSON(m, pairs...), golden(t, "adapter-env-noroot.golden.json"); got != want || len(m) != 16 {
		t.Errorf("the 16-key env (%d keys):\n got %s\nwant %s", len(m), got, want)
	}
	for _, v := range m {
		if v == env.AdapterPath {
			t.Fatal("AdapterPath is not in the map")
		}
	}
	if BoolEnv(true) != "true" || BoolEnv(false) != "false" {
		t.Fatal("BoolEnv")
	}
}

// Test 36 — an empty worktree keeps the Warns entry (the golden sentence,
// carrying WORKTREE_PATH and the root) and emits ONE WORKTREE_FALLBACK; with a
// worktree neither.
func TestEnv_WorktreeFallbackKeepsTheWarnsEntryAndEmitsTheCode(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.Inspect = func(string, string) (Capability, error) {
		return Capability{BudgetNative: true, PermissionScoping: true, Warns: []string{"capA"}}, nil
	}
	var env AdapterEnv
	deps.Adapter = AdapterFunc(func(ctx context.Context, e AdapterEnv) (int, error) { env = e; return soundAdapter(t).Exec(ctx, e) })
	d, r := observed(t, deps)
	req := f.request()
	req.Cycle, req.WorktreePath = 7, ""
	out, err := d.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(golden(t, "warn-worktree-fallback.golden.txt", "{ROOT}", f.root), "\n")
	if len(out.Warns) != 2 || out.Warns[0] != "capA" || out.Warns[1] != want || env.WorktreePath != f.root {
		t.Fatalf("warns %q worktree %q", out.Warns, env.WorktreePath)
	}
	e := r.only(t, CodeWorktreeFallback)
	if e.Fields["step"] != "env" || e.Fields["worktree"] != f.root || e.Fields["project_root"] != f.root || !strings.HasSuffix(want, e.Reason) || !strings.Contains(e.Reason, "WORKTREE_PATH") {
		t.Fatalf("%+v", e)
	}
	req.WorktreePath = f.worktree
	d, r = observed(t, deps)
	out, err = d.Dispatch(context.Background(), req)
	if err != nil || len(out.Warns) != 1 || out.Warns[0] != "capA" || env.WorktreePath != f.worktree || len(r.events) != 0 {
		t.Fatalf("with a worktree: %v %q %q %v", err, out.Warns, env.WorktreePath, r.codes())
	}
}

// Test 37 — the production stager: the name pattern, the exact bytes, mode
// 0600, cleanup removes; a create error names op=create, a closed file
// op=write and leaks nothing.
func TestStage_TempfileFailuresNameTheOpAndTheDefaultStagerCleansUp(t *testing.T) {
	path, cleanup, err := tempFileStager{create: os.CreateTemp}.Stage("prompt bytes")
	if err != nil {
		t.Fatal(err)
	}
	info, statErr := os.Stat(path)
	b, _ := os.ReadFile(path)
	base := filepath.Base(path)
	if statErr != nil || info.Mode().Perm() != 0o600 || string(b) != "prompt bytes" || !strings.HasPrefix(base, "evolve-subagent-prompt-") || !strings.HasSuffix(base, ".txt") {
		t.Fatalf("%v %v %q %s", statErr, info, b, base)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup removes the file")
	}

	f := newFixture(t)
	errTmpFull := errors.New("tmp full")
	failing := tempFileStager{create: func(string, string) (*os.File, error) { return nil, errTmpFull }}
	d, r := observed(t, happyDeps(t), WithStager(failing))
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || err.Error() != "subagent/run: prompt tempfile: tmp full" || !errors.Is(err, errTmpFull) {
		t.Fatalf("the cause stays reachable through the wrap: %v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["step"] != "stage" || e.Fields["op"] != "create" || e.Reason != "subagent/run: prompt tempfile: tmp full" {
		t.Fatalf("%+v", e)
	}

	var leaked string
	closed := tempFileStager{create: func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		leaked = file.Name()
		_ = file.Close()
		return file, nil
	}}
	d, r = observed(t, happyDeps(t), WithStager(closed))
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || !strings.HasPrefix(err.Error(), "subagent/run: write prompt tempfile: ") {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["op"] != "write" || e.Fields["step"] != "stage" {
		t.Fatalf("%+v", e)
	}
	if _, err := os.Stat(leaked); !os.IsNotExist(err) {
		t.Fatal("a failed write leaks no file")
	}
	plain := stagerFunc(func(string) (string, func(), error) { return "", nil, errors.New("opaque") })
	d, r = observed(t, happyDeps(t), WithStager(plain))
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || err.Error() != "opaque" {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["op"] != "" {
		t.Fatalf("an opaque stager error names no op: %+v", e)
	}
}

// Test 38 — the duration is the difference of the first two clock reads
// (exec start, exec end); verify and the ledger ts are the third and fourth.
func TestExecute_DurationFromTwoClockReads(t *testing.T) {
	f := newFixture(t)
	t0 := fixedNow
	ticks := []time.Time{t0, t0.Add(1999 * time.Millisecond), t0.Add(time.Hour), t0.Add(time.Hour)}
	call := 0
	stepping := func() time.Time {
		if call >= len(ticks) {
			t.Fatal("a fifth clock read")
		}
		call++
		return ticks[call-1]
	}
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, t0.Add(time.Hour))
		return 0, nil
	})
	d, _ := observed(t, deps, WithClock(stepping))
	req := f.request()
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	out, err := d.Dispatch(context.Background(), req)
	if err != nil || out.DurationMS != 1999 || out.Verdict != VerdictPASS || call != 4 {
		t.Fatalf("%v %+v reads %d", err, out, call)
	}
	line, _ := os.ReadFile(req.LedgerPath)
	if !strings.Contains(string(line), `"ts":"2026-05-23T18:00:00Z"`) || !strings.Contains(string(line), `"duration_s":"1"`) {
		t.Fatalf("%s", line)
	}
}
