package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	failuresLiveState = `{"failedApproaches":[` +
		`{"cycle":1,"classification":"infrastructure-systemic","summary":"a","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
		`{"cycle":2,"classification":"code-build-fail","summary":"b","recordedAt":"2026-10-02T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
		`{"cycle":3,"classification":"ship-gate-config","summary":"c","recordedAt":"2026-10-03T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
		`{"cycle":4,"classification":"code-audit-fail","summary":"d","recordedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-02-01T00:00:00Z"}]}`
	failuresCorruptResolved = "{corrupt resolved fingerprints"
)

func failuresProject(t *testing.T, state, resolved string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"state.json": state, "resolved-fingerprints.json": resolved} {
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(evolveDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, evolveDir
}

func failedApproachClasses(t *testing.T, evolveDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		FailedApproaches []struct {
			Classification string `json:"classification"`
		} `json:"failedApproaches"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("state.json: %v (%q)", err, raw)
	}
	classes := make([]string, 0, len(st.FailedApproaches))
	for _, e := range st.FailedApproaches {
		classes = append(classes, e.Classification)
	}
	return strings.Join(classes, ",")
}

func resolvedFingerprintsBody(t *testing.T, evolveDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRunFailures_ListFiltersAndRejects(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"failedApproaches":[{"cycle":1,"classification":"code-build-fail","summary":"a"},{"cycle":2,"classification":"ship-gate-config","summary":"b"}]}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{"filtered", []string{"list", "--project-root", root, "--class", "ship-gate-config"}, 0, "1 failed approach(es)", ""},
		{"all", []string{"list", "--project-root", root}, 0, "2 failed approach(es)", ""},
		{"unknown class", []string{"list", "--project-root", root, "--class", "nope"}, 10, "", "unknown class"},
		{"no verb", nil, 10, "", "usage:"},
		{"reset needs root", []string{"reset"}, 1, "", "mutating run refused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := runFailures(tc.args, nil, &out, &errb)
			if code != tc.wantCode || !strings.Contains(out.String(), tc.wantOut) || !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("code=%d out=%q err=%q", code, out.String(), errb.String())
			}
		})
	}
}

func TestResetBatchState_KeepsBasePrefixesOnEveryErrorPath(t *testing.T) {
	liveState := `{"failedApproaches":[{"cycle":1,"classification":"infrastructure-systemic","summary":"a"},{"cycle":2,"classification":"code-build-fail","summary":"b"}]}`
	cases := []struct {
		name        string
		state       string
		resolved    string
		fingerprint string
		wantLines   []string
		denyLines   []string
		wantAcked   bool
	}{
		{"unparseable state still acks", "{not json", "", "fp-a", []string{"[loop] --reset: failurelog: parse state", `[loop] --reset --fingerprint: acknowledged "fp-a"`}, nil, true},
		{"ack error still reports the prune", liveState, "{corrupt", "fp-b", []string{"[loop] --reset: pruned 1 failedApproaches", "[loop] --reset --fingerprint: resolved-fingerprints.json"}, []string{"acknowledged", "acknowledge fingerprint"}, false},
		{"clean reset", liveState, "", "fp-c", []string{"[loop] --reset: pruned 1 failedApproaches", `[loop] --reset --fingerprint: acknowledged "fp-c"`}, nil, true},
		{"no fingerprint never acks", "{not json", "", "", []string{"[loop] --reset: failurelog: parse state"}, []string{"--fingerprint"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			statePath := filepath.Join(evolveDir, "state.json")
			if err := os.WriteFile(statePath, []byte(tc.state), 0o644); err != nil {
				t.Fatal(err)
			}
			resolvedPath := filepath.Join(evolveDir, "resolved-fingerprints.json")
			if tc.resolved != "" {
				if err := os.WriteFile(resolvedPath, []byte(tc.resolved), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var errb bytes.Buffer
			resetBatchState(loopConfig{EvolveDir: evolveDir, Fingerprint: tc.fingerprint}, statePath, &errb)
			for _, want := range tc.wantLines {
				if !strings.Contains(errb.String(), want) {
					t.Errorf("want %q in %q", want, errb.String())
				}
			}
			for _, deny := range tc.denyLines {
				if strings.Contains(errb.String(), deny) {
					t.Errorf("unexpected %q in %q", deny, errb.String())
				}
			}
			raw, _ := os.ReadFile(resolvedPath)
			if got := tc.fingerprint != "" && strings.Contains(string(raw), tc.fingerprint); got != tc.wantAcked {
				t.Errorf("acked=%v want %v (resolved-fingerprints.json=%q)", got, tc.wantAcked, raw)
			}
		})
	}
}

func TestRunFailures_PruneRemovesOnlyExpiredEntries(t *testing.T) {
	root, evolveDir := failuresProject(t, failuresLiveState, "")
	var out, errb bytes.Buffer
	if code := runFailures([]string{"prune", "--project-root", root}, nil, &out, &errb); code != 0 {
		t.Fatalf("prune code=%d err=%q", code, errb.String())
	}
	if got := failedApproachClasses(t, evolveDir); got != "infrastructure-systemic,code-build-fail,ship-gate-config" {
		t.Errorf("after prune classes=%q, want only the expired code-audit-fail removed", got)
	}
	if !strings.Contains(out.String(), "removed 1 expired failedApproaches (4→3)") {
		t.Errorf("prune report=%q", out.String())
	}
	errb.Reset()
	if code := runFailures([]string{"prune"}, nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "mutating run refused") {
		t.Errorf("prune without --project-root: code=%d err=%q", code, errb.String())
	}
	corruptRoot, corruptDir := failuresProject(t, "{not json", "")
	errb.Reset()
	if code := runFailures([]string{"prune", "--project-root", corruptRoot}, nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "evolve failures prune:") {
		t.Errorf("prune of unparseable state: code=%d err=%q", code, errb.String())
	}
	if raw, _ := os.ReadFile(filepath.Join(corruptDir, "state.json")); string(raw) != "{not json" {
		t.Errorf("a failed prune rewrote state.json: %q", raw)
	}
}

func TestRunFailures_ResetErrorPaths(t *testing.T) {
	cases := []struct {
		name        string
		state       string
		resolved    string
		fingerprint string
		wantCode    int
		wantClasses string
		wantOut     []string
		denyOut     []string
		wantAcked   bool
	}{
		{"drops only reset classes", failuresLiveState, "", "", 0, "code-build-fail,code-audit-fail", []string{"pruned 2 failedApproaches (4→2)"}, []string{"acknowledged", "--fingerprint"}, false},
		{"acks the fingerprint", failuresLiveState, "", "fp-reset", 0, "code-build-fail,code-audit-fail", []string{"pruned 2", `acknowledged "fp-reset"`}, nil, true},
		{"unparseable state still acks", "{not json", "", "fp-reset", 1, "", []string{"parse state", `acknowledged "fp-reset"`}, nil, true},
		{"ack error still reports the prune", failuresLiveState, failuresCorruptResolved, "fp-reset", 1, "code-build-fail,code-audit-fail", []string{"pruned 2", "evolve failures reset: --fingerprint:", "resolved-fingerprints.json"}, []string{"acknowledged"}, false},
		{"no fingerprint never acks", "{not json", "", "", 1, "", []string{"parse state"}, []string{"--fingerprint"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, evolveDir := failuresProject(t, tc.state, tc.resolved)
			args := []string{"reset", "--project-root", root}
			if tc.fingerprint != "" {
				args = append(args, "--fingerprint", tc.fingerprint)
			}
			var out, errb bytes.Buffer
			code := runFailures(args, nil, &out, &errb)
			all := out.String() + errb.String()
			if code != tc.wantCode {
				t.Errorf("code=%d want %d; out=%q", code, tc.wantCode, all)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(all, want) {
					t.Errorf("want %q in %q", want, all)
				}
			}
			for _, deny := range tc.denyOut {
				if strings.Contains(all, deny) {
					t.Errorf("unexpected %q in %q", deny, all)
				}
			}
			if tc.wantClasses != "" {
				if got := failedApproachClasses(t, evolveDir); got != tc.wantClasses {
					t.Errorf("classes=%q want %q", got, tc.wantClasses)
				}
			}
			body := resolvedFingerprintsBody(t, evolveDir)
			if acked := tc.fingerprint != "" && strings.Contains(body, tc.fingerprint); acked != tc.wantAcked {
				t.Errorf("acked=%v want %v (resolved-fingerprints.json=%q)", acked, tc.wantAcked, body)
			}
			if tc.resolved != "" && !tc.wantAcked && body != tc.resolved {
				t.Errorf("a failed ack rewrote resolved-fingerprints.json: %q", body)
			}
		})
	}
}

func TestRunLoop_ResetErrorPathsKeepBasePrefixes(t *testing.T) {
	cases := []struct {
		name        string
		state       string
		resolved    string
		fingerprint string
		wantLines   []string
		denyLines   []string
		wantAcked   bool
	}{
		{"unparseable state still acks", "{not json", "", "fp-loop", []string{"[loop] --reset: failurelog: parse state", `[loop] --reset --fingerprint: acknowledged "fp-loop"`}, nil, true},
		{"ack error still reports the prune", failuresLiveState, failuresCorruptResolved, "fp-loop", []string{"[loop] --reset: pruned 2 failedApproaches", "[loop] --reset --fingerprint: "}, []string{`acknowledged "fp-loop"`, "[loop] --reset: acknowledge fingerprint"}, false},
		{"no fingerprint never acks", "{not json", "", "", []string{"[loop] --reset: failurelog: parse state"}, []string{"[loop] --reset --fingerprint"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, evolveDir := failuresProject(t, tc.state, tc.resolved)
			writeDispatchPolicy(t, evolveDir, "off")
			defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()
			args := []string{"--project-root", root, "--evolve-dir", evolveDir, "--reset", "--goal-text", "x", "--cycles", "1"}
			if tc.fingerprint != "" {
				args = append(args, "--fingerprint", tc.fingerprint)
			}
			var stdout, stderr bytes.Buffer
			runLoop(args, nil, &stdout, &stderr)
			for _, want := range tc.wantLines {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("want %q in %q", want, stderr.String())
				}
			}
			for _, deny := range tc.denyLines {
				if strings.Contains(stderr.String(), deny) {
					t.Errorf("unexpected %q in %q", deny, stderr.String())
				}
			}
			body := resolvedFingerprintsBody(t, evolveDir)
			if acked := tc.fingerprint != "" && strings.Contains(body, tc.fingerprint); acked != tc.wantAcked {
				t.Errorf("acked=%v want %v (resolved-fingerprints.json=%q)", acked, tc.wantAcked, body)
			}
			if tc.resolved != "" && body != tc.resolved {
				t.Errorf("a failed ack rewrote resolved-fingerprints.json: %q", body)
			}
		})
	}
}
