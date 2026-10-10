package sandbox

import (
	"strings"
	"testing"
)

const runSocketDeny = `(deny network-outbound (remote unix-socket (path-literal "/private/tmp/tmux-501/evolve-bridge-p42")))`

func TestGenerateSBPL_DeniesTheRunSocketAfterEveryNetworkGrant(t *testing.T) {
	cfg := canonicalConfig()
	cfg.AllowNetwork = true
	cfg.DenySockets = []string{"", "/private/tmp/tmux-501/evolve-bridge-p42"}
	sbpl := GenerateSBPL(cfg)
	grant, deny := strings.Index(sbpl, "(allow network*)"), strings.Index(sbpl, runSocketDeny)
	if grant < 0 || deny < grant {
		t.Fatalf("the run socket deny must close the profile, after every network grant, so no grant emitted later can reopen the socket (grant=%d deny=%d):\n%s", grant, deny, sbpl)
	}
	if n := strings.Count(sbpl, "unix-socket"); n != 1 {
		t.Fatalf("an empty socket path must emit no rule; got %d unix-socket rules:\n%s", n, sbpl)
	}
}

func TestGenerateSBPL_NoSocketPathsEmitsNoSocketRule(t *testing.T) {
	if sbpl := GenerateSBPL(canonicalConfig()); strings.Contains(sbpl, "unix-socket") {
		t.Fatalf("a profile without socket denials must not name a socket:\n%s", sbpl)
	}
}

func TestGenerateSBPL_LiteralWriteDenialsFollowTheScratchGrant(t *testing.T) {
	cfg := canonicalConfig()
	cfg.DenyLiterals = []string{"", "/private/tmp/tmux-501", "/private/tmp/tmux-501/evolve-bridge-p42"}
	sbpl := GenerateSBPL(cfg)
	grant := strings.Index(sbpl, `(allow file-write* (subpath "/private/tmp"))`)
	for _, want := range []string{
		`(deny file-write* (literal "/private/tmp/tmux-501"))`,
		`(deny file-write* (literal "/private/tmp/tmux-501/evolve-bridge-p42"))`,
	} {
		if at := strings.Index(sbpl, want); at < grant {
			t.Fatalf("%s must follow the /private/tmp write grant (grant=%d at=%d):\n%s", want, grant, at, sbpl)
		}
	}
	if n := strings.Count(sbpl, "(deny file-write* (literal"); n != 2 {
		t.Fatalf("an empty literal must emit no rule; got %d literal rules:\n%s", n, sbpl)
	}
}

func TestGenerateSBPL_SignalsReachOnlyTheSameSandbox(t *testing.T) {
	sbpl := GenerateSBPL(canonicalConfig())
	if !strings.Contains(sbpl, "(allow signal (target same-sandbox))\n") || strings.Contains(sbpl, "(allow signal)\n") {
		t.Fatalf("the agent may signal only processes of its own sandbox:\n%s", sbpl)
	}
}
