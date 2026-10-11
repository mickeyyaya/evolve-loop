package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

var _ Authenticator = (*Authority)(nil)

var (
	testKey   = []byte("0123456789abcdef0123456789abcdef")
	otherKey  = []byte("fedcba9876543210fedcba9876543210")
	testNow   = time.Date(2026, 10, 9, 21, 0, 0, 0, time.UTC)
	testUntil = testNow.Add(37 * time.Minute)
	testTTL   = 3 * time.Hour
	builder   = Principal{Cycle: 1850, Dispatch: "run-a/1850/build/p4242n1t0", Agent: "builder", Phase: "build", Role: "build"}
)

func newAuthority(t *testing.T, key []byte, now time.Time) *Authority {
	t.Helper()
	a, err := NewAuthority(key, func() time.Time { return now }, testTTL)
	if err != nil {
		t.Fatalf("NewAuthority error = %v", err)
	}
	return a
}

func mint(t *testing.T, a *Authority, p Principal) string {
	t.Helper()
	tok, err := a.Mint(p, testUntil)
	if err != nil {
		t.Fatalf("Mint error = %v", err)
	}
	return tok
}

func at(location, token string) Credential { return Credential{Token: token, Location: location} }

func encode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func signed(caveats ...string) string {
	parts := make([]string, 0, len(caveats)+1)
	sig := root(testKey)
	for _, c := range caveats {
		sig = chain(sig, c)
		parts = append(parts, encode(c))
	}
	return strings.Join(append(parts, base64.RawURLEncoding.EncodeToString(sig)), ".")
}

func extend(t *testing.T, tok, caveat string) string {
	t.Helper()
	parts := strings.Split(tok, ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[len(parts)-1])
	if err != nil {
		t.Fatal(err)
	}
	head := strings.Join(parts[:len(parts)-1], ".")
	return head + "." + encode(caveat) + "." + base64.RawURLEncoding.EncodeToString(chain(sig, caveat))
}

func unix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

func canonical(p Principal, expires time.Time) []string {
	return []string{
		"cycle=" + strconv.Itoa(p.Cycle), "dispatch=" + p.Dispatch, "expires=" + unix(expires),
		"agent=" + p.Agent, "phase=" + p.Phase, "role=" + p.Role,
	}
}

func TestAuthority_AMintedTokenAuthenticatesItsPrincipal(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	got, err := a.Authenticate(context.Background(), at(builder.Dispatch, mint(t, a, builder)))
	if err != nil {
		t.Fatalf("Authenticate error = %v", err)
	}
	if got != builder {
		t.Fatalf("Authenticate = %+v, want %+v", got, builder)
	}
}

func TestAuthority_TheLocationAndTheTokenMustNameOneDispatch(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	tok := mint(t, a, builder)
	_, err := a.Authenticate(context.Background(), at("run-a/1850/audit/p77n2", tok))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate with another dispatch's outbox error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_ACredentialWithoutALocationIsRefused(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	_, err := a.Authenticate(context.Background(), at("", mint(t, a, builder)))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate with no outbox location error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_ATokenFromAnotherKeyIsRefused(t *testing.T) {
	forger := newAuthority(t, otherKey, testNow)
	a := newAuthority(t, testKey, testNow)
	_, err := a.Authenticate(context.Background(), at(builder.Dispatch, mint(t, forger, builder)))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(token of another key) error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_AChangedCaveatIsRefused(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	tok := mint(t, a, builder)
	forged := strings.Replace(tok, encode("cycle=1850"), encode("cycle=1851"), 1)
	if forged == tok {
		t.Fatal("the test did not change the cycle caveat")
	}
	_, err := a.Authenticate(context.Background(), at(builder.Dispatch, forged))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(changed cycle) error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_AnAddedCaveatOtherThanAnEarlierExpiryIsRefused(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	tok := mint(t, a, builder)
	cases := map[string]string{
		"a second cycle":       "cycle=7",
		"a second phase":       "phase=audit",
		"a second role":        "role=audit",
		"a second agent":       "agent=auditor",
		"a later expiry":       "expires=" + unix(testUntil.Add(time.Hour)),
		"the same expiry":      "expires=" + unix(testUntil),
		"an unknown condition": "scope=all",
	}
	for name, caveat := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := a.Authenticate(context.Background(), at(builder.Dispatch, extend(t, tok, caveat)))
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("Authenticate(token + %q) error = %v, want ErrUnauthenticated", caveat, err)
			}
		})
	}
}

func TestAuthority_AnAddedEarlierExpiryNarrowsTheToken(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	earlier := testNow.Add(10 * time.Minute)
	tok := extend(t, mint(t, a, builder), "expires="+unix(earlier))
	if _, err := a.Authenticate(context.Background(), at(builder.Dispatch, tok)); err != nil {
		t.Fatalf("Authenticate before the narrowed expiry error = %v, want nil", err)
	}
	later := newAuthority(t, testKey, earlier)
	if _, err := later.Authenticate(context.Background(), at(builder.Dispatch, tok)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate at the narrowed expiry error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_APhaseAddedToAPhaseLessTokenIsRefused(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	phaseless := signed("cycle=1850", "dispatch="+builder.Dispatch, "expires="+unix(testUntil))
	_, err := a.Authenticate(context.Background(), at(builder.Dispatch, extend(t, phaseless, "phase=audit")))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(phase-less token + phase=audit) error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_ATokenExpiresAtItsDeadline(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	tok := mint(t, a, builder)
	before := newAuthority(t, testKey, testUntil.Add(-time.Second))
	if _, err := before.Authenticate(context.Background(), at(builder.Dispatch, tok)); err != nil {
		t.Fatalf("Authenticate one second before the expiry error = %v, want nil", err)
	}
	atExpiry := newAuthority(t, testKey, testUntil)
	if _, err := atExpiry.Authenticate(context.Background(), at(builder.Dispatch, tok)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate at the expiry error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthority_RefusesATokenThatIsNotTheCanonicalSet(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	full := canonical(builder, testUntil)
	with := func(i int, caveat string) []string {
		c := append([]string(nil), full...)
		c[i] = caveat
		return c
	}
	swapped := append([]string(nil), full...)
	swapped[3], swapped[4] = swapped[4], swapped[3]
	cases := map[string]string{
		"empty":                        "",
		"one part":                     encode("cycle=1"),
		"a caveat not base64":          "!!!." + base64.RawURLEncoding.EncodeToString(root(testKey)),
		"a signature not base64":       encode("cycle=1") + ".!!!",
		"only the old required set":    signed(full[:3]...),
		"no role":                      signed(full[:5]...),
		"two caveats swapped":          signed(swapped...),
		"a caveat without a value":     signed(with(0, "cycle")...),
		"a cycle not a number":         signed(with(0, "cycle=x")...),
		"a cycle of zero":              signed(with(0, "cycle=0")...),
		"an empty dispatch":            signed(with(1, "dispatch=")...),
		"an expiry not a number":       signed(with(2, "expires=soon")...),
		"an empty phase":               signed(with(4, "phase=")...),
		"an added expiry not a number": signed(append(append([]string(nil), full...), "expires=soon")...),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := a.Authenticate(context.Background(), at(builder.Dispatch, tok))
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("Authenticate(%q) error = %v, want ErrUnauthenticated", tok, err)
			}
		})
	}
	if _, err := a.Authenticate(context.Background(), at(builder.Dispatch, signed(full...))); err != nil {
		t.Fatalf("Authenticate(the canonical set) error = %v, want nil", err)
	}
}

func TestNewAuthority_RefusesABrokenConfig(t *testing.T) {
	cases := map[string]struct {
		key []byte
		now func() time.Time
		ttl time.Duration
		err error
	}{
		"a short key":         {testKey[:MinKeyBytes-1], time.Now, testTTL, ErrKey},
		"no clock":            {testKey, nil, testTTL, ErrConfig},
		"a zero lifetime":     {testKey, time.Now, 0, ErrConfig},
		"a negative lifetime": {testKey, time.Now, -time.Second, ErrConfig},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAuthority(tc.key, tc.now, tc.ttl); !errors.Is(err, tc.err) {
				t.Fatalf("NewAuthority error = %v, want %v", err, tc.err)
			}
		})
	}
}

func TestNewAuthority_KeepsItsOwnCopyOfTheKey(t *testing.T) {
	key := append([]byte(nil), testKey...)
	a := newAuthority(t, key, testNow)
	tok := mint(t, a, builder)
	key[0] ^= 0xff
	if _, err := a.Authenticate(context.Background(), at(builder.Dispatch, tok)); err != nil {
		t.Fatalf("Authenticate after the caller changed its key slice error = %v, want nil", err)
	}
}

func TestMint_RefusesAPrincipalWithoutTheFullIdentity(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	for name, change := range map[string]func(*Principal){
		"no cycle":    func(p *Principal) { p.Cycle = 0 },
		"no dispatch": func(p *Principal) { p.Dispatch = "" },
		"no agent":    func(p *Principal) { p.Agent = "" },
		"no phase":    func(p *Principal) { p.Phase = "" },
		"no role":     func(p *Principal) { p.Role = "" },
	} {
		t.Run(name, func(t *testing.T) {
			p := builder
			change(&p)
			if _, err := a.Mint(p, testUntil); !errors.Is(err, ErrPrincipal) {
				t.Fatalf("Mint(%+v) error = %v, want ErrPrincipal", p, err)
			}
		})
	}
}

func TestMint_RefusesAnExpiryOutsideTheLifetime(t *testing.T) {
	a := newAuthority(t, testKey, testNow)
	for name, expires := range map[string]time.Time{
		"zero":              {},
		"past":              testNow.Add(-time.Second),
		"now":               testNow,
		"past the lifetime": testNow.Add(testTTL + time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := a.Mint(builder, expires); !errors.Is(err, ErrExpiry) {
				t.Fatalf("Mint(expires %s) error = %v, want ErrExpiry", expires, err)
			}
		})
	}
	if _, err := a.Mint(builder, testNow.Add(testTTL)); err != nil {
		t.Fatalf("Mint(expires at the lifetime) error = %v, want nil", err)
	}
}
