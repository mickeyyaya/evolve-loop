package sandbox

import (
	"slices"
	"strings"
	"testing"
)

const claudeExecDeny = `(deny process-exec (require-all (literal "/opt/llm/claude") (require-not (signing-identifier "com.apple.sandbox-exec"))))`

func TestGenerateSBPL_EnforceDeniesEachLLMBinaryAndSandboxExecAfterTheExecGrant(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ExecDenyPaths, cfg.ExecDenyEnforce = []string{"", "/opt/llm/claude"}, true
	sbpl := GenerateSBPL(cfg)
	grant := strings.Index(sbpl, "(allow process-exec)\n")
	for _, want := range []string{claudeExecDeny, `(deny process-exec (require-all (literal "/usr/bin/sandbox-exec") (require-not (signing-identifier "com.apple.sandbox-exec"))))`} {
		if at := strings.Index(sbpl, want); at < 0 || at < grant {
			t.Fatalf("%s must follow the exec grant, a later rule wins (grant=%d at=%d):\n%s", want, grant, at, sbpl)
		}
	}
	if n := strings.Count(sbpl, "process-exec (require-all"); n != 2 {
		t.Fatalf("an empty path must emit no rule; got %d exec rules:\n%s", n, sbpl)
	}
}

func TestGenerateSBPL_ShadowReportsTheExecAndDeniesNothing(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ExecDenyPaths = []string{"/opt/llm/claude"}
	sbpl := GenerateSBPL(cfg)
	want := `(allow process-exec (require-all (literal "/opt/llm/claude") (require-not (signing-identifier "com.apple.sandbox-exec"))) (with report))`
	if !strings.Contains(sbpl, want) || strings.Contains(sbpl, "(deny process-exec") {
		t.Fatalf("shadow must report the exec of an LLM binary and deny no exec:\n%s", sbpl)
	}
}

func TestGenerateSBPL_NoExecDenyPathsEmitsNoExecRule(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ExecDenyEnforce = true
	if sbpl := GenerateSBPL(cfg); strings.Count(sbpl, "process-exec") != 1 {
		t.Fatalf("a profile without exec denials keeps only the exec grant:\n%s", sbpl)
	}
}

func TestBwrapPrefix_EnforceMasksEachLLMBinaryWithDevNullAfterEveryBind(t *testing.T) {
	cfg := Config{RepoRoot: "/repo", ExecDenyPaths: []string{"", "/opt/llm/agy"}, ExecDenyEnforce: true}
	argv := BwrapPrefix(cfg)
	at := slices.Index(argv, "/opt/llm/agy")
	if at < 2 || argv[at-2] != "--ro-bind" || argv[at-1] != "/dev/null" || at != len(argv)-1 {
		t.Fatalf("enforce must end the prefix with --ro-bind /dev/null /opt/llm/agy, so no earlier bind uncovers it: %q", argv)
	}
	if n := strings.Count(strings.Join(argv, " "), "/dev/null"); n != 1 {
		t.Fatalf("an empty path must emit no mask; got %d: %q", n, argv)
	}
}

func TestBwrapPrefix_ShadowMasksNothing(t *testing.T) {
	argv := BwrapPrefix(Config{ExecDenyPaths: []string{"/opt/llm/agy"}})
	if slices.Contains(argv, "/opt/llm/agy") {
		t.Fatalf("shadow must not mask a binary on Linux: %q", argv)
	}
}

func TestBwrapPrefix_TheExecMaskFollowsTheParentBindAndTheReadDenials(t *testing.T) {
	cfg := Config{
		RepoRoot: "/w/repo", WritePaths: []string{"", "/w"}, DenyReadPaths: []string{"", "/w/repo/private"},
		ExecDenyPaths: []string{"/w/bin/claude"}, ExecDenyEnforce: true,
	}
	argv := strings.Join(BwrapPrefix(cfg), " ")
	order := []string{"--bind /w /w", "--bind /w/repo /w/repo", "--tmpfs /w/repo/private --chmod 000 /w/repo/private --remount-ro /w/repo/private", "--cap-drop ALL", "--unshare-net", "--ro-bind /dev/null /w/bin/claude"}
	last := -1
	for _, want := range order {
		at := strings.Index(argv, want)
		if at <= last {
			t.Fatalf("%q must follow the earlier binds (at=%d after %d): %s", want, at, last, argv)
		}
		last = at
	}
	if strings.Count(argv, "--bind /w /w") != 1 {
		t.Fatalf("a write path that holds the repo is bound once, before the repo: %s", argv)
	}
}
