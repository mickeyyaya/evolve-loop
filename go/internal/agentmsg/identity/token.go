package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const MinKeyBytes = 32

const tokenDomain = "evolve.agentmsg.token/1"

const (
	caveatCycle    = "cycle"
	caveatDispatch = "dispatch"
	caveatExpires  = "expires"
	caveatAgent    = "agent"
	caveatPhase    = "phase"
	caveatRole     = "role"
)

var encoding = base64.RawURLEncoding

type claim struct {
	principal Principal
	expires   time.Time
}

func root(key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(tokenDomain))
	return mac.Sum(nil)
}

func chain(sig []byte, caveat string) []byte {
	mac := hmac.New(sha256.New, sig)
	mac.Write([]byte(caveat))
	return mac.Sum(nil)
}

var canonicalKeys = []string{caveatCycle, caveatDispatch, caveatExpires, caveatAgent, caveatPhase, caveatRole}

func caveatsOf(p Principal, expires time.Time) []string {
	values := []string{strconv.Itoa(p.Cycle), p.Dispatch, strconv.FormatInt(expires.Unix(), 10), p.Agent, p.Phase, p.Role}
	caveats := make([]string, len(canonicalKeys))
	for i, key := range canonicalKeys {
		caveats[i] = key + "=" + values[i]
	}
	return caveats
}

func seal(key []byte, caveats []string) string {
	sig := root(key)
	parts := make([]string, 0, len(caveats)+1)
	for _, c := range caveats {
		sig = chain(sig, c)
		parts = append(parts, encoding.EncodeToString([]byte(c)))
	}
	return strings.Join(append(parts, encoding.EncodeToString(sig)), ".")
}

func open(key []byte, token string) ([]string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("%w: a token has caveats and a signature", ErrUnauthenticated)
	}
	got, err := encoding.DecodeString(parts[len(parts)-1])
	if err != nil {
		return nil, fmt.Errorf("%w: signature: %v", ErrUnauthenticated, err)
	}
	sig := root(key)
	caveats := make([]string, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		raw, err := encoding.DecodeString(part)
		if err != nil {
			return nil, fmt.Errorf("%w: caveat: %v", ErrUnauthenticated, err)
		}
		sig = chain(sig, string(raw))
		caveats = append(caveats, string(raw))
	}
	if !hmac.Equal(got, sig) {
		return nil, fmt.Errorf("%w: the signature does not match", ErrUnauthenticated)
	}
	return caveats, nil
}

func claimOf(caveats []string) (claim, error) {
	if len(caveats) < len(canonicalKeys) {
		return claim{}, fmt.Errorf("%w: a token holds the full identity", ErrUnauthenticated)
	}
	values := make(map[string]string, len(canonicalKeys))
	for i, key := range canonicalKeys {
		value, found := strings.CutPrefix(caveats[i], key+"=")
		if !found || value == "" {
			return claim{}, fmt.Errorf("%w: caveat %d is %q, want %s=…", ErrUnauthenticated, i, caveats[i], key)
		}
		values[key] = value
	}
	cycle, cerr := strconv.Atoi(values[caveatCycle])
	expires, eerr := expiryOf(values[caveatExpires])
	if cerr != nil || eerr != nil || cycle <= 0 {
		return claim{}, fmt.Errorf("%w: the cycle or the expiry is not a positive number", ErrUnauthenticated)
	}
	expires, err := narrowed(expires, caveats[len(canonicalKeys):])
	if err != nil {
		return claim{}, err
	}
	p := Principal{Cycle: cycle, Dispatch: values[caveatDispatch], Agent: values[caveatAgent], Phase: values[caveatPhase], Role: values[caveatRole]}
	return claim{principal: p, expires: expires}, nil
}

func narrowed(expires time.Time, added []string) (time.Time, error) {
	for _, c := range added {
		value, found := strings.CutPrefix(c, caveatExpires+"=")
		earlier, err := expiryOf(value)
		if !found || err != nil || !earlier.Before(expires) {
			return time.Time{}, fmt.Errorf("%w: an added caveat %q is not an earlier expiry", ErrUnauthenticated, c)
		}
		expires = earlier
	}
	return expires, nil
}

func expiryOf(value string) (time.Time, error) {
	unix, err := strconv.ParseInt(value, 10, 64)
	return time.Unix(unix, 0), err
}
