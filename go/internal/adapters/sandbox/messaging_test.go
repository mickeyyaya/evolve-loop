package sandbox

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func messagingFixture() *MessagingScope {
	return &MessagingScope{
		Tree:        "/nonexistent/repo/.evolve/msg",
		Dispatch:    "/nonexistent/repo/.evolve/msg/7/S0VZ",
		Out:         "/nonexistent/repo/.evolve/msg/7/S0VZ/out",
		StorePrefix: "/nonexistent/repo/.evolve/events/ch/msg.",
		StoreDirs:   []string{"/nonexistent/repo/.evolve/events/ch/msg.7", "/nonexistent/repo/.evolve/events/ch/msg.dead.7"},
	}
}

func golden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const messagingTail = `(deny file-read* (subpath "/nonexistent/repo/.evolve/msg"))
(allow file-read* (subpath "/nonexistent/repo/.evolve/msg/7/S0VZ"))
(deny file-write* (subpath "/nonexistent/repo/.evolve/msg"))
(allow file-write* (subpath "/nonexistent/repo/.evolve/msg/7/S0VZ/out"))
(deny file-write* (prefix "/nonexistent/repo/.evolve/events/ch/msg."))
(deny file-read* (prefix "/nonexistent/repo/.evolve/events/ch/msg."))
`

func TestSBPL_NoMessagingIdentity_ProfileIsByteIdentical(t *testing.T) {
	t.Parallel()
	if got, want := GenerateSBPL(fullFixtureConfig()), golden(t, "sbpl-no-messaging.golden"); got != want {
		t.Fatalf("a config with no messaging scope must keep today's profile byte for byte:\n%s", got)
	}
}

func TestBwrap_NoMessagingIdentity_ArgvIsByteIdentical(t *testing.T) {
	t.Parallel()
	got := strings.Join(BwrapPrefix(fullFixtureConfig()), "\n") + "\n"
	if want := golden(t, "bwrap-no-messaging.golden"); got != want {
		t.Fatalf("a config with no messaging scope must keep today's argv:\n%s", got)
	}
}

func TestSBPL_MessagingBlockComesLastSoItBeatsEveryEarlierRuleIncludingTheProtectedPass(t *testing.T) {
	t.Parallel()
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	got := GenerateSBPL(cfg)
	if want := golden(t, "sbpl-no-messaging.golden") + messagingTail; got != want {
		t.Fatalf("the messaging rules must follow the whole base profile in this order:\n%s", got)
	}
}

func TestSBPL_AllowsOnlyTheOwnOutbox(t *testing.T) {
	t.Parallel()
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	tail := strings.TrimPrefix(GenerateSBPL(cfg), golden(t, "sbpl-no-messaging.golden"))
	var writeAllows []string
	for _, line := range strings.Split(tail, "\n") {
		if strings.HasPrefix(line, "(allow file-write*") {
			writeAllows = append(writeAllows, line)
		}
	}
	if want := []string{`(allow file-write* (subpath "/nonexistent/repo/.evolve/msg/7/S0VZ/out"))`}; !slices.Equal(writeAllows, want) {
		t.Fatalf("only the own outbox may be written: %q", writeAllows)
	}
}

func TestSBPL_DeniesTheStoreAndOtherMailboxes(t *testing.T) {
	t.Parallel()
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	got := GenerateSBPL(cfg)
	for _, rule := range []string{
		`(deny file-write* (subpath "/nonexistent/repo/.evolve/msg"))`,
		`(deny file-write* (prefix "/nonexistent/repo/.evolve/events/ch/msg."))`,
	} {
		if !strings.Contains(got, rule) {
			t.Fatalf("missing %s", rule)
		}
	}
}

func TestSBPL_DeniesReadsOfOtherMailboxesAndOtherCycles(t *testing.T) {
	t.Parallel()
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	got := GenerateSBPL(cfg)
	deny := strings.Index(got, `(deny file-read* (subpath "/nonexistent/repo/.evolve/msg"))`)
	allow := strings.Index(got, `(allow file-read* (subpath "/nonexistent/repo/.evolve/msg/7/S0VZ"))`)
	if deny < 0 || allow < deny {
		t.Fatalf("the tree read deny must come first, then the own dispatch allow: deny=%d allow=%d", deny, allow)
	}
	if !strings.Contains(got, `(deny file-read* (prefix "/nonexistent/repo/.evolve/events/ch/msg."))`) {
		t.Fatal("the store must be unreadable, or a mailbox deny is hollow")
	}
}

func TestSBPL_MessagingRulesUseNoLiteralAndNoTrailingSlash(t *testing.T) {
	t.Parallel()
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	tail := strings.TrimPrefix(GenerateSBPL(cfg), golden(t, "sbpl-no-messaging.golden"))
	if strings.Contains(tail, "literal") || strings.Contains(tail, `/")`) || strings.Contains(tail, `/"`) {
		t.Fatalf("a literal does not cover children and a trailing slash is not a clean path:\n%s", tail)
	}
}

func TestBwrap_MessagingMasksTheTreeAndTheStoreAndBindsOnlyTheOwnDispatch(t *testing.T) {
	t.Parallel()
	base := BwrapPrefix(fullFixtureConfig())
	cfg := fullFixtureConfig()
	cfg.Messaging = messagingFixture()
	got := BwrapPrefix(cfg)
	segment := []string{
		"--tmpfs", "/nonexistent/repo/.evolve/msg",
		"--ro-bind-try", "/nonexistent/repo/.evolve/msg/7/S0VZ", "/nonexistent/repo/.evolve/msg/7/S0VZ",
		"--bind-try", "/nonexistent/repo/.evolve/msg/7/S0VZ/out", "/nonexistent/repo/.evolve/msg/7/S0VZ/out",
		"--tmpfs", "/nonexistent/repo/.evolve/events/ch/msg.7",
		"--tmpfs", "/nonexistent/repo/.evolve/events/ch/msg.dead.7",
	}
	at := slices.Index(got, segment[1]) - 1
	if at < 0 || !slices.Equal(got[at:at+len(segment)], segment) {
		t.Fatalf("the tree and the store must be masked by tmpfs, then the own dispatch bound: %q", got)
	}
	if rest := slices.Concat(got[:at], got[at+len(segment):]); !slices.Equal(rest, base) {
		t.Fatalf("nothing else may change: %q", rest)
	}
	repoBind := slices.Index(got, "/nonexistent/repo")
	if repoBind < 0 || repoBind > at {
		t.Fatal("the masks must come after the repository bind, or the bind would hide them")
	}
}
