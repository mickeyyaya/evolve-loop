package phaseregistrar

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestCoverfixRegisterRejectsCLIWithNoRegisteredDriver(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.CLI = "no-such-cli"
	cfg.Dispatch.AllowedCLIs = []string{"no-such-cli"}
	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "does not resolve to a registered driver")
	if fixtures.FilePresent(r.ProfilesDir + "/minted-reviewer.json") {
		t.Error("profile persisted despite an unresolvable driver")
	}
}

func TestCoverfixRegisterRejectsDerivedProfileNameThatStripsToNonKebab(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Agent = "evolve-1x"
	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, `derived profile name "1x"`)
	if fixtures.FilePresent(r.PhasesDir + "/minted-reviewer/phase.json") {
		t.Error("phase spec persisted despite a rejected derived profile name")
	}
}

func TestCoverfixRegisterStampsMintWithInjectedClock(t *testing.T) {
	r := newRegistrar(t)
	r.RegistryPath = mintregistry.Path(t.TempDir())
	minted := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	r.NowFn = func() time.Time { return minted }
	if _, err := r.Register(validCfg()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	atMint, err := mintregistry.ActiveNames(r.RegistryPath, minted)
	if err != nil {
		t.Fatal(err)
	}
	if !atMint["minted-reviewer"] {
		t.Errorf("mint stamped with NowFn must be active at that instant; got %v", atMint)
	}
	farLater, err := mintregistry.ActiveNames(r.RegistryPath, minted.Add(mintregistry.TTL+time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if farLater["minted-reviewer"] {
		t.Errorf("mint stamped in 2001 must have expired once the TTL has passed (wall clock was used instead of NowFn); got %v", farLater)
	}
}

func TestRegisterRejectsTheRetiredBareNameGemini(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.CLI = "gemini"
	cfg.Dispatch.AllowedCLIs = []string{"gemini"}
	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "agy-tmux")
}
