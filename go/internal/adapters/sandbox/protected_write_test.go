package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func protectedConfig(enforce bool) Config {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/wt"}
	cfg.ProtectedWriteSubpaths = []string{"", "/wt/go/internal/guards"}
	cfg.ProtectedWritePrefixes = []string{"", "/wt/go/internal/phases/audit/probe_quarantine"}
	cfg.ProtectedWriteEnforce = enforce
	return cfg
}

func TestGenerateSBPL_EnforceDeniesEachProtectedPathAfterTheWorktreeGrant(t *testing.T) {
	sbpl := GenerateSBPL(protectedConfig(true))
	grant := strings.Index(sbpl, `(allow file-write* (subpath "/wt"))`)
	for _, want := range []string{
		`(deny file-write* (subpath "/wt/go/internal/guards"))`,
		`(deny file-write* (prefix "/wt/go/internal/phases/audit/probe_quarantine"))`,
	} {
		if at := strings.Index(sbpl, want); grant < 0 || at < grant {
			t.Fatalf("%s must follow the worktree grant, a later rule wins (grant=%d at=%d):\n%s", want, grant, at, sbpl)
		}
	}
	if n := strings.Count(sbpl, "(deny file-write* (subpath \"/wt") + strings.Count(sbpl, "(prefix "); n != 2 {
		t.Fatalf("an empty path must emit no rule; got %d protected rules:\n%s", n, sbpl)
	}
}

func TestGenerateSBPL_ShadowReportsTheProtectedWriteAndDeniesNothing(t *testing.T) {
	sbpl := GenerateSBPL(protectedConfig(false))
	for _, want := range []string{
		`(allow file-write* (subpath "/wt/go/internal/guards") (with report))`,
		`(allow file-write* (prefix "/wt/go/internal/phases/audit/probe_quarantine") (with report))`,
	} {
		if !strings.Contains(sbpl, want) {
			t.Fatalf("shadow must report %s:\n%s", want, sbpl)
		}
	}
	if strings.Contains(sbpl, `(deny file-write* (subpath "/wt/`) || strings.Contains(sbpl, "(deny file-write* (prefix") {
		t.Fatalf("shadow must deny no protected write:\n%s", sbpl)
	}
}

func TestGenerateSBPL_ShadowReportsBeforeTheDeniesSoAnEarlierDenyStillWins(t *testing.T) {
	cfg := protectedConfig(false)
	cfg.DenyPaths = []string{"/wt/go/internal/guards/sealed"}
	sbpl := GenerateSBPL(cfg)
	report := strings.Index(sbpl, `(allow file-write* (subpath "/wt/go/internal/guards") (with report))`)
	deny := strings.Index(sbpl, `(deny file-write* (subpath "/wt/go/internal/guards/sealed"))`)
	if report < 0 || deny < report {
		t.Fatalf("a shadow report must come before each deny, a later rule wins (report=%d deny=%d):\n%s", report, deny, sbpl)
	}
}

func TestBwrapPrefix_EnforceBindsEachProtectedPathReadOnlyBeforeTheExecMasks(t *testing.T) {
	wt := t.TempDir()
	audit := filepath.Join(wt, "go", "internal", "phases", "audit")
	if err := os.MkdirAll(audit, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"probe_quarantine.go", "probe_quarantine_test.go", "audit.go"} {
		if err := os.WriteFile(filepath.Join(audit, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	guards := filepath.Join(wt, "go", "internal", "guards")
	if err := os.MkdirAll(guards, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{WritePaths: []string{wt}, ProtectedWriteEnforce: true, ExecDenyEnforce: true, ExecDenyPaths: []string{"/opt/llm/agy"},
		ProtectedWriteSubpaths: []string{"", guards, filepath.Join(wt, "skills", "audit")},
		ProtectedWritePrefixes: []string{"", filepath.Join(audit, "probe_quarantine"), filepath.Join(wt, "missing", "x")}}
	argv := BwrapPrefix(cfg)
	joined := strings.Join(argv, " ")
	bind := strings.Index(joined, "--bind "+wt+" "+wt)
	for _, p := range []string{guards, filepath.Join(audit, "probe_quarantine.go"), filepath.Join(audit, "probe_quarantine_test.go")} {
		if at := strings.Index(joined, "--ro-bind "+p+" "+p); at < 0 || at < bind {
			t.Fatalf("enforce must bind %s read-only after the worktree bind: %q", p, argv)
		}
	}
	if strings.Contains(joined, "audit.go") || strings.Count(joined, "--ro-bind "+wt) != 3 {
		t.Fatalf("only the protected paths that exist get a bind, as Linux binds only a path that exists: %q", argv)
	}
	if argv[len(argv)-1] != "/opt/llm/agy" {
		t.Fatalf("the exec masks stay last: %q", argv)
	}
}

func TestBwrapPrefix_ShadowBindsNoProtectedPath(t *testing.T) {
	argv := BwrapPrefix(Config{ProtectedWriteSubpaths: []string{"/wt/go/internal/guards"}})
	if slices.Contains(argv, "/wt/go/internal/guards") {
		t.Fatalf("shadow must not bind a protected path on Linux: %q", argv)
	}
}

func TestSandboxContract_KillMinusOneFromACellReachesNoHostProcess(t *testing.T) {
	sbpl := GenerateSBPL(protectedConfig(true))
	var signalRules []string
	for _, line := range strings.Split(sbpl, "\n") {
		if strings.Contains(line, "signal") {
			signalRules = append(signalRules, line)
		}
	}
	if !strings.Contains(sbpl, "(deny default)") || !slices.Equal(signalRules, []string{"(allow signal (target same-sandbox))"}) {
		t.Fatalf("macOS: kill(-1) must reach only the processes of the own sandbox; deny default plus one same-sandbox signal grant, got %q", signalRules)
	}
	if argv := BwrapPrefix(protectedConfig(true)); !slices.Contains(argv, "--unshare-pid") {
		t.Fatalf("Linux: kill(-1) must reach only the own PID namespace: %q", argv)
	}
}
