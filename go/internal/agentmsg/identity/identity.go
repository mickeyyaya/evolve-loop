package identity

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnauthenticated = errors.New("identity: unauthenticated")
	ErrKey             = errors.New("identity: key")
	ErrPrincipal       = errors.New("identity: principal")
	ErrConfig          = errors.New("identity: config")
	ErrExpiry          = errors.New("identity: expiry")
)

type Principal struct {
	Cycle    int    `json:"cycle"`
	Dispatch string `json:"dispatch,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Role     string `json:"role,omitempty"`
}

type Credential struct {
	Token    string
	Location string
}

type Authenticator interface {
	Authenticate(ctx context.Context, c Credential) (Principal, error)
}

type Authority struct {
	key    []byte
	now    func() time.Time
	maxTTL time.Duration
}

func NewAuthority(key []byte, now func() time.Time, maxTTL time.Duration) (*Authority, error) {
	if len(key) < MinKeyBytes {
		return nil, fmt.Errorf("%w: %d bytes, want at least %d", ErrKey, len(key), MinKeyBytes)
	}
	if now == nil || maxTTL <= 0 {
		return nil, fmt.Errorf("%w: an authority needs a clock and a positive token lifetime", ErrConfig)
	}
	return &Authority{key: append([]byte(nil), key...), now: now, maxTTL: maxTTL}, nil
}

func (a *Authority) Mint(p Principal, expires time.Time) (string, error) {
	if p.Cycle <= 0 || p.Dispatch == "" || p.Agent == "" || p.Phase == "" || p.Role == "" {
		return "", fmt.Errorf("%w: a token needs the cycle, the dispatch, the agent, the phase and the role, got %+v", ErrPrincipal, p)
	}
	now := a.now()
	if !expires.After(now) || expires.Sub(now) > a.maxTTL {
		return "", fmt.Errorf("%w: %s is not after now and within %s", ErrExpiry, expires.Format(time.RFC3339), a.maxTTL)
	}
	return seal(a.key, caveatsOf(p, expires)), nil
}

func (a *Authority) Authenticate(_ context.Context, c Credential) (Principal, error) {
	caveats, err := open(a.key, c.Token)
	if err != nil {
		return Principal{}, err
	}
	claim, err := claimOf(caveats)
	if err != nil {
		return Principal{}, err
	}
	if !a.now().Before(claim.expires) {
		return Principal{}, fmt.Errorf("%w: the token expired at %s", ErrUnauthenticated, claim.expires.Format(time.RFC3339))
	}
	if c.Location == "" || c.Location != claim.principal.Dispatch {
		return Principal{}, fmt.Errorf("%w: the outbox of %q holds a token of %q", ErrUnauthenticated, c.Location, claim.principal.Dispatch)
	}
	return claim.principal, nil
}
