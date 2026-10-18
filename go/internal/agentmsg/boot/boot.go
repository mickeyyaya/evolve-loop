package boot

import (
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type Minter interface {
	Mint(p identity.Principal, expires time.Time) (string, error)
}

func Env(m Minter, p identity.Principal, expires time.Time, env []string) ([]string, error) {
	token, err := m.Mint(p, expires)
	if err != nil {
		return nil, err
	}
	out := slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		return strings.HasPrefix(kv, ipcenv.MsgTokenKey+"=")
	})
	return append(out, ipcenv.MsgTokenKey+"="+token), nil
}
