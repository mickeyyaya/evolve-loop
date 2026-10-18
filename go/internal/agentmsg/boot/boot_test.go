package boot_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/boot"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var (
	now  = time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	key  = []byte("0123456789abcdef0123456789abcdef")
	pBld = identity.Principal{Cycle: 7, Dispatch: "r/7/build/p1n1", Agent: "builder", Phase: "build", Role: "build"}
)

func authority(t *testing.T) *identity.Authority {
	t.Helper()
	a, err := identity.NewAuthority(key, func() time.Time { return now }, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestBoot_ExportsTheMessageToken(t *testing.T) {
	a := authority(t)
	base := []string{"PATH=/bin", ipcenv.MsgTokenKey + "=stale", "HOME=/h"}

	env, err := boot.Env(a, pBld, now.Add(time.Hour), base)

	if err != nil {
		t.Fatalf("Env error = %v", err)
	}
	var tokens []string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, ipcenv.MsgTokenKey+"="); ok {
			tokens = append(tokens, v)
		}
	}
	if len(tokens) != 1 || tokens[0] == "stale" {
		t.Fatalf("env = %v, want one fresh %s", env, ipcenv.MsgTokenKey)
	}
	got, err := a.Authenticate(context.Background(), identity.Credential{Token: tokens[0], Location: pBld.Dispatch})
	if err != nil || got != pBld {
		t.Errorf("Authenticate(minted token) = %+v, %v, want %+v", got, err, pBld)
	}
	if !slices.Contains(env, "PATH=/bin") || !slices.Contains(env, "HOME=/h") {
		t.Errorf("env = %v, want the other entries kept", env)
	}
	if base[1] != ipcenv.MsgTokenKey+"=stale" || len(base) != 3 {
		t.Errorf("base = %v, want the input unchanged", base)
	}
}

func TestBoot_RefusesAnIdentityTheAuthorityRefuses(t *testing.T) {
	env, err := boot.Env(authority(t), identity.Principal{Cycle: 7}, now.Add(time.Hour), []string{"A=1"})

	if !errors.Is(err, identity.ErrPrincipal) || env != nil {
		t.Errorf("Env = %v, %v, want nil and ErrPrincipal", env, err)
	}
}

type refusingMinter struct{ err error }

func (m refusingMinter) Mint(identity.Principal, time.Time) (string, error) { return "", m.err }

func TestBoot_AMinterFailureIsReturnedAndNoEnvIsBuilt(t *testing.T) {
	boom := errors.New("boom")
	var m boot.Minter = refusingMinter{err: boom}

	env, err := boot.Env(m, pBld, now.Add(time.Hour), []string{"A=1"})

	if !errors.Is(err, boom) || env != nil {
		t.Errorf("Env = %v, %v, want nil and the minter error", env, err)
	}
}
