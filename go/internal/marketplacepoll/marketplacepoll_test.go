package marketplacepoll

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

// makeMarketplace creates a fake marketplace directory at the given version.
// Mirrors the bash test helper `make_marketplace`.
func makeMarketplace(t *testing.T, version string) string {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf(`{"name":"evo","version":"%s"}`, version)
	if err := os.WriteFile(filepath.Join(d, ".claude-plugin", "plugin.json"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	return d
}

// stubClock returns a (now, advance) pair. advance(dur) moves the clock
// forward by dur, so deadline math is deterministic.
func stubClock(start time.Time) (now func() time.Time, advance func(time.Duration)) {
	var t atomic.Int64
	t.Store(start.UnixNano())
	return func() time.Time {
			return time.Unix(0, t.Load())
		},
		func(d time.Duration) {
			t.Add(int64(d))
		}
}

// === Test 1: marketplace already at target → match on first poll ============
func TestRun_MatchOnFirstPoll(t *testing.T) {
	m := makeMarketplace(t, "1.2.3")
	now, _ := stubClock(time.Now())
	releaseCalls := 0
	var buf bytes.Buffer

	res, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        5 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
		Pull:           func(string) error { return nil },
		ReleaseSh: func(_, target string) error {
			releaseCalls++
			if target != "1.2.3" {
				t.Errorf("release.sh called with target=%q want 1.2.3", target)
			}
			return nil
		},
		Stderr: &buf,
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if !res.Converged {
		t.Error("want Converged=true")
	}
	if res.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", res.Attempts)
	}
	if releaseCalls != 1 {
		t.Errorf("release.sh calls = %d, want 1", releaseCalls)
	}
	if !res.ReleaseShRunOK {
		t.Error("want ReleaseShRunOK=true")
	}
	if !strings.Contains(buf.String(), "converged to v1.2.3") {
		t.Errorf("log missing converged line: %s", buf.String())
	}
}

// === Test 2: marketplace catches up after N polls ===========================
func TestRun_MatchAfterDelay(t *testing.T) {
	m := makeMarketplace(t, "0.9.0")
	now, advance := stubClock(time.Now())
	releaseCalls := 0
	pullCalls := 0

	res, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        10 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		// Sleep advances our stub clock so we cross the poll interval.
		Sleep: func(d time.Duration) { advance(d) },
		// On the 3rd pull, bump the marketplace version.
		Pull: func(dir string) error {
			pullCalls++
			if pullCalls == 3 {
				body := []byte(`{"name":"evo","version":"1.2.3"}`)
				if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "plugin.json"),
					body, 0o644); err != nil {
					return err
				}
			}
			return nil
		},
		ReleaseSh: func(_, _ string) error { releaseCalls++; return nil },
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if !res.Converged {
		t.Error("want Converged=true")
	}
	if res.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", res.Attempts)
	}
	if releaseCalls != 1 {
		t.Errorf("release.sh calls = %d, want 1", releaseCalls)
	}
}

// === Test 3: STALE-VERSION REGRESSION — never matches → ErrTimeout =========
func TestRun_StaleVersionTimeout(t *testing.T) {
	m := makeMarketplace(t, "0.9.0")
	now, advance := stubClock(time.Now())
	releaseCalls := 0

	res, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        4 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(d time.Duration) { advance(d) },
		Pull:           func(string) error { return nil }, // never updates
		ReleaseSh:      func(_, _ string) error { releaseCalls++; return nil },
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Run err = %v, want ErrTimeout", err)
	}
	if res.Converged {
		t.Error("want Converged=false")
	}
	if releaseCalls != 0 {
		t.Errorf("release.sh calls = %d, want 0 (ordering invariant)", releaseCalls)
	}
	if res.FinalVersion != "0.9.0" {
		t.Errorf("FinalVersion = %q, want 0.9.0", res.FinalVersion)
	}
}

// === Test 4: missing marketplace dir → ErrRuntime ===========================
func TestRun_MissingMarketplaceDir(t *testing.T) {
	now, _ := stubClock(time.Now())
	_, err := Run(Options{
		Target:         "1.0.0",
		MarketplaceDir: "/tmp/does-not-exist-mkpoll-test-xyz",
		MaxWait:        2 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want contains 'not found'", err)
	}
}

// === Test 5: CACHE-REFRESH ORDERING — release.sh runs after match ===========
func TestRun_ReleaseShCalledAfterConvergence(t *testing.T) {
	m := makeMarketplace(t, "1.2.3")
	now, _ := stubClock(time.Now())
	var releaseCalls []string

	_, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        5 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
		Pull:           func(string) error { return nil },
		ReleaseSh: func(_, target string) error {
			releaseCalls = append(releaseCalls, target)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if len(releaseCalls) != 1 {
		t.Fatalf("release.sh invocations = %d, want 1: %v", len(releaseCalls), releaseCalls)
	}
	if releaseCalls[0] != "1.2.3" {
		t.Errorf("release.sh called with %q, want 1.2.3", releaseCalls[0])
	}
}

// === Test 6: missing plugin.json → ErrRuntime ===============================
func TestRun_MissingPluginJSON(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No plugin.json file at all.
	now, _ := stubClock(time.Now())
	_, err := Run(Options{
		Target:         "1.0.0",
		MarketplaceDir: d,
		MaxWait:        2 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
}

// === Test 7: --dry-run: no polling, no release.sh ===========================
func TestRun_DryRun(t *testing.T) {
	m := makeMarketplace(t, "0.9.0")
	releaseCalls := 0
	pullCalls := 0
	var buf bytes.Buffer

	res, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        60 * time.Second,
		PollInterval:   1 * time.Second,
		DryRun:         true,
		Pull:           func(string) error { pullCalls++; return nil },
		ReleaseSh: func(_, _ string) error {
			releaseCalls++
			return errors.New("should not be called")
		},
		Stderr: &buf,
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if res.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0 (no polling in dry-run)", res.Attempts)
	}
	if pullCalls != 0 {
		t.Errorf("Pull calls = %d, want 0", pullCalls)
	}
	if releaseCalls != 0 {
		t.Errorf("ReleaseSh calls = %d, want 0", releaseCalls)
	}
	if !strings.Contains(buf.String(), "DRY-RUN") {
		t.Errorf("log missing DRY-RUN: %s", buf.String())
	}
}

// === Test 8 (impl): invalid semver target → ErrRuntime ======================
// (Test 8 in bash exercises arg parsing; that lives in the cmd layer. Here
// we cover Run's own semver guard.)
func TestRun_InvalidSemver(t *testing.T) {
	now, _ := stubClock(time.Now())
	_, err := Run(Options{
		Target:         "garbage",
		MarketplaceDir: t.TempDir(),
		MaxWait:        2 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
}

// === Test 9: release.sh failure → ErrRuntime, FAIL message ==================
func TestRun_ReleaseShFailure(t *testing.T) {
	m := makeMarketplace(t, "1.2.3")
	now, _ := stubClock(time.Now())
	var buf bytes.Buffer

	_, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        5 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
		Pull:           func(string) error { return nil },
		ReleaseSh:      func(_, _ string) error { return errors.New("exit 7") },
		Stderr:         &buf,
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
	if !strings.Contains(buf.String(), "release.sh exited non-zero") {
		t.Errorf("log missing release.sh failure: %s", buf.String())
	}
}

// === Test 10: zero MaxWait → ErrRuntime =====================================
func TestRun_ZeroMaxWait(t *testing.T) {
	_, err := Run(Options{
		Target:         "1.0.0",
		MarketplaceDir: t.TempDir(),
		MaxWait:        0,
		PollInterval:   1 * time.Second,
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
}

func TestRun_ZeroPollInterval(t *testing.T) {
	_, err := Run(Options{
		Target:         "1.0.0",
		MarketplaceDir: t.TempDir(),
		MaxWait:        1 * time.Second,
		PollInterval:   0,
	})
	if !errors.Is(err, ErrRuntime) {
		t.Fatalf("Run err = %v, want ErrRuntime", err)
	}
}

// === IsSemver unit table ====================================================
func TestIsSemver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1.2.3", true},
		{"11.7.2", true},
		{"0.0.0", true},
		{"v1.2.3", false},
		{"1.2", false},
		{"1.2.3.4", false},
		{"1.2.3-alpha", false},
		{"", false},
		{"garbage", false},
	}
	for _, tc := range cases {
		if got := semvercheck.IsSemver(tc.in); got != tc.want {
			t.Errorf("IsSemver(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// === parseVersionField ======================================================
func TestParseVersionField(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"basic", `{"version":"1.2.3"}`, "1.2.3"},
		{"with spaces", `{"version"  :  "9.9.9"}`, "9.9.9"},
		{"first match", `{"version":"1.0.0","other":"2.0.0"}`, "1.0.0"},
		{"no field", `{"name":"foo"}`, ""},
		{"empty", "", ""},
		{"malformed", `version: 1.2.3`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseVersionField(tc.in); got != tc.want {
				t.Errorf("parseVersionField(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// === ReadMarketplaceVersion =================================================
func TestReadMarketplaceVersion(t *testing.T) {
	d := makeMarketplace(t, "5.5.5")
	v, err := ReadMarketplaceVersion(d)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if v != "5.5.5" {
		t.Errorf("version = %q, want 5.5.5", v)
	}
}

func TestReadMarketplaceVersion_MissingFile(t *testing.T) {
	d := t.TempDir()
	_, err := ReadMarketplaceVersion(d)
	if err == nil {
		t.Error("want err, got nil")
	}
}

// === DefaultPull: non-git dir is no-op =====================================
func TestDefaultPull_NonGitDir(t *testing.T) {
	d := t.TempDir()
	if err := DefaultPull(d); err != nil {
		t.Errorf("DefaultPull on non-git dir err = %v, want nil", err)
	}
}

// === DefaultPull: .git directory present — git cmds run, errors swallowed ===
// Exercises the git-checkout branch: when .git is a directory the function
// runs git fetch + git reset; since d is not a real repo both fail, but
// DefaultPull swallows every error and returns nil (mirrors bash behaviour).
func TestDefaultPull_GitDirPresent_ExecutesGit_ErrorsSwallowed(t *testing.T) {
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := DefaultPull(d); err != nil {
		t.Errorf("DefaultPull with .git dir = %v, want nil (errors must be swallowed)", err)
	}
}

// === DefaultReleaseSh tests ==================================================

// TestDefaultReleaseSh_AlwaysNil: DefaultReleaseSh is a Go-native no-op in v12+
// (ADR-0062/T1.7) — it returns nil regardless of repo contents and never shells
// out. The former Script{Absent,ExitsNonZero,ExitsOK} tests asserted the deleted
// bash path and were removed; the no-shell-out guarantee is additionally proven
// by TestDefaultReleaseSh_IgnoresLegacyScript (releasesh_noexec_test.go).
func TestDefaultReleaseSh_AlwaysNil(t *testing.T) {
	if err := DefaultReleaseSh(t.TempDir(), "1.2.3"); err != nil {
		t.Errorf("DefaultReleaseSh = %v, want nil (Go no-op)", err)
	}
}
