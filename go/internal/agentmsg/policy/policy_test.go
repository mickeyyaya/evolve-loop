package policy

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

var _ Authorizer = (*Engine)(nil)

var (
	scout   = identity.Principal{Cycle: 1850, Dispatch: "r/1850/scout/p11n1", Phase: "scout", Role: "scout"}
	tdd     = identity.Principal{Cycle: 1850, Dispatch: "r/1850/tdd/p12n1", Phase: "tdd", Role: "tdd"}
	build   = identity.Principal{Cycle: 1850, Dispatch: "r/1850/build/p13n1", Phase: "build", Role: "build"}
	audit   = identity.Principal{Cycle: 1850, Phase: "audit", Role: "audit"}
	lane    = identity.Principal{Cycle: 1851, Dispatch: "r/1851/audit/p21n1", Phase: "audit", Role: "audit"}
	planned = identity.Principal{Cycle: 1850, Phase: "ship", Role: "ship"}
)

func compile(t *testing.T, rules []Rule) *Engine {
	t.Helper()
	e, err := Compile(rules)
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	return e
}

func defaults(t *testing.T) *Engine { return compile(t, Defaults()) }

func TestDefaults_AllowEveryKindInsideTheCycle(t *testing.T) {
	e := defaults(t)
	for _, k := range envelope.DefaultKinds() {
		d := e.Authorize(Request{Sender: scout, Recipient: planned, Kind: k})
		if !d.Allow || d.Rule != "same_cycle" {
			t.Fatalf("Authorize(scout → ship, %s) = %+v, want allow by same_cycle", k, d)
		}
	}
}

func TestDefaults_DenyARecipientInAnotherCycle(t *testing.T) {
	d := defaults(t).Authorize(Request{Sender: scout, Recipient: lane, Kind: envelope.KindNote})
	if d.Allow || d.Rule != DefaultRule {
		t.Fatalf("Authorize(cycle 1850 → cycle 1851) = %+v, want deny by the default", d)
	}
	if d.Reason == "" {
		t.Fatal("a denial must give a reason")
	}
}

func TestDefaults_DenyFreeTextFromBuildAndTDDToAudit(t *testing.T) {
	e := defaults(t)
	for _, sender := range []identity.Principal{build, tdd} {
		for _, k := range envelope.DefaultKinds() {
			d := e.Authorize(Request{Sender: sender, Recipient: audit, Kind: k})
			wantAllow := k == envelope.KindEvidence
			if d.Allow != wantAllow {
				t.Fatalf("Authorize(%s → audit, %s) = %+v, want allow=%v", sender.Phase, k, d, wantAllow)
			}
			if !wantAllow && d.Rule != "audit_independence" {
				t.Fatalf("Authorize(%s → audit, %s) rule = %q, want audit_independence", sender.Phase, k, d.Rule)
			}
		}
	}
}

func TestDefaults_AuditCanSendFindingsToBuild(t *testing.T) {
	auditor := audit
	auditor.Dispatch = "r/1850/audit/p14n1"
	d := defaults(t).Authorize(Request{Sender: auditor, Recipient: build, Kind: envelope.KindFinding})
	if !d.Allow {
		t.Fatalf("Authorize(audit → build, finding) = %+v, want allow", d)
	}
}

func TestDefaults_AReplyGrantNeverOverridesADeny(t *testing.T) {
	d := defaults(t).Authorize(Request{Sender: build, Recipient: audit, Kind: envelope.KindReply, ReplyGrant: true})
	if d.Allow || d.Rule != "audit_independence" {
		t.Fatalf("Authorize(build → audit, granted reply) = %+v, want deny by audit_independence", d)
	}
}

func TestDefaults_ReturnsANewSliceEachCall(t *testing.T) {
	first := Defaults()
	first[0].Name = "changed"
	first[0].When.FromPhase[0] = "changed"
	again := Defaults()
	if again[0].Name == "changed" || again[0].When.FromPhase[0] == "changed" {
		t.Fatal("Defaults must return a new rule list each call")
	}
}

func TestEngine_ADenyWinsOverAnEarlierAllow(t *testing.T) {
	e := compile(t, []Rule{
		{Name: "open", Effect: EffectAllow},
		{Name: "no_notes", Effect: EffectDeny, When: Match{Kind: []envelope.Kind{envelope.KindNote}}},
		{Name: "no_scout", Effect: EffectDeny, When: Match{FromPhase: []string{"scout"}}},
	})
	d := e.Authorize(Request{Sender: scout, Recipient: build, Kind: envelope.KindNote})
	want := Decision{Allow: false, Rule: "no_notes", Reason: "the rule no_notes denies it"}
	if d != want {
		t.Fatalf("Authorize = %+v, want %+v: the first matching deny", d, want)
	}
}

func TestEngine_TheFirstMatchingAllowNamesTheDecision(t *testing.T) {
	e := compile(t, []Rule{
		{Name: "to_audit", Effect: EffectAllow, When: Match{ToPhase: []string{"audit"}}},
		{Name: "from_scout", Effect: EffectAllow, When: Match{FromPhase: []string{"scout"}}},
		{Name: "any", Effect: EffectAllow},
	})
	d := e.Authorize(Request{Sender: scout, Recipient: build, Kind: envelope.KindNote})
	if !d.Allow || d.Rule != "from_scout" {
		t.Fatalf("Authorize = %+v, want allow by from_scout", d)
	}
}

func TestEngine_NoMatchingRuleDenies(t *testing.T) {
	e := compile(t, []Rule{{Name: "audit_only", Effect: EffectAllow, When: Match{ToPhase: []string{"audit"}}}})
	d := e.Authorize(Request{Sender: scout, Recipient: build, Kind: envelope.KindNote})
	if d.Allow || d.Rule != DefaultRule {
		t.Fatalf("Authorize = %+v, want deny by the default", d)
	}
	if empty := compile(t, nil).Authorize(Request{Sender: scout, Recipient: build}); empty.Allow {
		t.Fatalf("an empty rule list allowed %+v, want deny", empty)
	}
}

func TestEngine_AReplyRightAllowsOnlyAGrantedReply(t *testing.T) {
	e := compile(t, []Rule{{Name: "reply_right", Effect: EffectAllow, When: Match{ReplyGrant: true}}})
	granted := e.Authorize(Request{Sender: build, Recipient: scout, Kind: envelope.KindReply, ReplyGrant: true})
	if !granted.Allow || granted.Rule != "reply_right" {
		t.Fatalf("Authorize(granted reply) = %+v, want allow by reply_right", granted)
	}
	free := e.Authorize(Request{Sender: build, Recipient: scout, Kind: envelope.KindReply})
	if free.Allow {
		t.Fatalf("Authorize(reply with no grant) = %+v, want deny", free)
	}
}

func TestEngine_EachConditionNarrowsTheMatch(t *testing.T) {
	base := Request{Sender: scout, Recipient: build, Kind: envelope.KindNote}
	cases := []struct {
		name string
		when Match
		req  Request
		want bool
	}{
		{"any scope holds inside the cycle", Match{Scope: ScopeAny}, base, true},
		{"any scope holds across cycles", Match{Scope: ScopeAny}, Request{Sender: scout, Recipient: lane}, true},
		{"same cycle holds", Match{Scope: ScopeSameCycle}, base, true},
		{"same cycle fails", Match{Scope: ScopeSameCycle}, Request{Sender: scout, Recipient: lane}, false},
		{"cross cycle holds", Match{Scope: ScopeCrossCycle}, Request{Sender: scout, Recipient: lane}, true},
		{"cross cycle fails", Match{Scope: ScopeCrossCycle}, base, false},
		{"from phase holds", Match{FromPhase: []string{"tdd", "scout"}}, base, true},
		{"from phase fails", Match{FromPhase: []string{"build"}}, base, false},
		{"from role holds", Match{FromRole: []string{"scout"}}, base, true},
		{"from role fails", Match{FromRole: []string{"audit"}}, base, false},
		{"to phase holds", Match{ToPhase: []string{"build"}}, base, true},
		{"to phase fails", Match{ToPhase: []string{"audit"}}, base, false},
		{"to role holds", Match{ToRole: []string{"build"}}, base, true},
		{"to role fails", Match{ToRole: []string{"scout"}}, base, false},
		{"kind holds", Match{Kind: []envelope.Kind{envelope.KindFinding, envelope.KindNote}}, base, true},
		{"kind fails", Match{Kind: []envelope.Kind{envelope.KindFinding}}, base, false},
		{"except kind holds", Match{ExceptKind: []envelope.Kind{envelope.KindEvidence}}, base, true},
		{"except kind fails", Match{ExceptKind: []envelope.Kind{envelope.KindNote}}, base, false},
		{"reply grant holds", Match{ReplyGrant: true}, Request{Sender: scout, Recipient: build, ReplyGrant: true}, true},
		{"reply grant fails", Match{ReplyGrant: true}, base, false},
		{"all conditions hold", Match{Scope: ScopeSameCycle, FromPhase: []string{"scout"}, ToRole: []string{"build"}, Kind: []envelope.Kind{envelope.KindNote}}, base, true},
		{"one condition of many fails", Match{Scope: ScopeSameCycle, FromPhase: []string{"scout"}, ToRole: []string{"audit"}}, base, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := compile(t, []Rule{{Name: "probe", Effect: EffectAllow, When: tc.when}})
			if got := e.Authorize(tc.req).Allow; got != tc.want {
				t.Fatalf("Authorize allow = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompile_RefusesABrokenRule(t *testing.T) {
	cases := map[string][]Rule{
		"no name":           {{Effect: EffectAllow}},
		"the reserved name": {{Name: DefaultRule, Effect: EffectDeny}},
		"a repeated name":   {{Name: "a", Effect: EffectAllow}, {Name: "a", Effect: EffectDeny}},
		"an unknown effect": {{Name: "a", Effect: "maybe"}},
		"an unknown scope":  {{Name: "a", Effect: EffectAllow, When: Match{Scope: "same_host"}}},
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(rules); !errors.Is(err, ErrRule) {
				t.Fatalf("Compile error = %v, want ErrRule", err)
			}
		})
	}
}

func TestCompile_KeepsItsOwnCopyOfTheRules(t *testing.T) {
	rules := []Rule{{Name: "deny_scout", Effect: EffectDeny, When: Match{FromPhase: []string{"scout"}}}, {Name: "open", Effect: EffectAllow}}
	e := compile(t, rules)
	rules[0].Effect = EffectAllow
	rules[0].When.FromPhase[0] = "audit"
	if d := e.Authorize(Request{Sender: scout, Recipient: build}); d.Allow {
		t.Fatalf("Authorize after the caller changed its rules = %+v, want deny by deny_scout", d)
	}
}
