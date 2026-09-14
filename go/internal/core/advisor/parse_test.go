package advisor

// parse_test.go — the pure parsers (ADR-0103 unit 04 §6 tests 22-24, 26, 36,
// 37; the core scrollback/failure/replay/tier tests moved verbatim in intent).

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Test 22 — the proposal parser takes the LAST balanced object (the reply,
// not the prompt's echoed example), tolerates a fence and prose, accepts a
// failure-fields-only proposal, and rejects the empty/absent/malformed ones
// with the pre-extraction texts.
func TestParseProposal_LastBalancedObjectFenceProseAndEmptyRules(t *testing.T) {
	scrollback := `Respond with STRICT JSON:
{"next_phase":"<phase>","insert_phases":["<phase>"],"justification":"<one sentence>"}

⏺ {"next_phase":"build","insert_phases":[],"justification":"scout done"}
❯`
	prop, err := ParseProposal(scrollback)
	if err != nil || prop.NextPhase != "build" {
		t.Fatalf("the reply, not the prompt's <phase> example: %+v %v", prop, err)
	}
	prop, err = ParseProposal("Here is my routing call:\n```json\n{\"next_phase\":\"audit\",\"justification\":\"done\"}\n```\nThanks!")
	if err != nil || prop.NextPhase != "audit" {
		t.Fatalf("fence + prose: %+v %v", prop, err)
	}
	prop, err = ParseProposal(`{"recovery_action":"end","justification":"budget"}`)
	if err != nil || prop.RecoveryAction != "end" {
		t.Fatalf("a failure-fields-only proposal is not empty: %+v %v", prop, err)
	}
	if prop, err = ParseProposal(`{"learning_richness":"memo"}`); err != nil || prop.LearningRichness != "memo" {
		t.Fatalf("learning_richness alone is not empty: %+v %v", prop, err)
	}
	for _, c := range []struct{ in, want, cause string }{
		{`{"justification":"nothing actionable"}`, "empty proposal", "empty"},
		{"I could not decide.", "no JSON object in proposer output", "no_json"},
		{`prefix {"next_phase": } suffix`, "parse proposal: invalid character '}' looking for beginning of value", "invalid_json"},
	} {
		_, err := ParseProposal(c.in)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: %v, want %q", c.in, err, c.want)
		}
		if pf, ok := err.(*parseFailure); !ok || pf.cause != c.cause || pf.Unwrap() == nil {
			t.Errorf("%q: the failure is classified %q: %#v", c.in, c.cause, err)
		}
	}
}

// Test 23 — the plan parser sanitizes tiers, defaults writes_source, collects
// run:false mints, reports the reserved-name drops as DATA (the original name
// quoted in the reason) and rejects the empty/absent/malformed bodies; the
// routing-eval corpus parses exactly as before.
func TestParsePhasePlan_SanitizesTiersAndReportsRejectedMintsAsData(t *testing.T) {
	raw := `[
	  {"phase":"scout","run":true,"justification":"fresh","tier":"opus","cli":"claude"},
	  {"phase":"build","run":true,"tier":"top"},
	  {"phase":"security-sweep","run":true,"justification":"auth changed","mint":{"prompt":"You are a security reviewer.","tier":"deep","cli":"claude","writes_source":false}},
	  {"phase":"deferred-probe","run":false,"mint":{"prompt":"probe persona","tier":"fast"}},
	  {"phase":"schema-drift-check","run":true,"mint":{"prompt":"drift persona","tier":"balanced","description":"Reports wire-schema drift.","when_to_use":"Select when router wire structs change."}},
	  {"phase":"router","run":true,"mint":{"prompt":"be a router"}},
	  {"phase":"evolve-router","run":true,"mint":{"prompt":"x"}},
	  {"phase":"Failure-Advisor","run":true,"mint":{"prompt":"x"}},
	  {"phase":"new-helper","run":true,"mint":{"prompt":"legit"}}
	]`
	parsed, err := ParsePhasePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Plan.Entries[0].Tier != "" || parsed.Plan.Entries[0].CLI != "claude" || parsed.Plan.Entries[1].Tier != "top" {
		t.Errorf("opus dropped, top kept, cli untouched: %+v", parsed.Plan.Entries[:2])
	}
	names := make([]string, 0, len(parsed.Plan.MintPhases))
	for _, m := range parsed.Plan.MintPhases {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "security-sweep,deferred-probe,schema-drift-check,new-helper" {
		t.Errorf("minted: %v", names)
	}
	sweep, probe, drift := parsed.Plan.MintPhases[0], parsed.Plan.MintPhases[1], parsed.Plan.MintPhases[2]
	if sweep.WritesSource || !probe.WritesSource || sweep.Dispatch.ModelTierDefault != "deep" || sweep.Dispatch.CLI != "claude" || !strings.Contains(sweep.Prompt, "security reviewer") {
		t.Errorf("mint fields: %+v / %+v", sweep, probe)
	}
	if drift.Description != "Reports wire-schema drift." || drift.WhenToUse != "Select when router wire structs change." || probe.Description != "" {
		t.Errorf("SELECT metadata carried, absent stays empty: %+v / %+v", drift, probe)
	}
	if len(parsed.RejectedMints) != 3 || parsed.RejectedMints[2].Phase != "Failure-Advisor" ||
		parsed.RejectedMints[2].Reason != `recursion guard: a minted phase may not assume the control-plane router/advisor identity "Failure-Advisor"` {
		t.Errorf("the drops are data quoting the ORIGINAL name: %+v", parsed.RejectedMints)
	}
	for _, c := range []struct{ in, want, cause string }{
		{"[]", "empty phase plan", "empty"},
		{"I could not decide.", "no JSON array in plan output", "no_json"},
		{`[{"phase":}]`, "parse phase plan: invalid character '}' looking for beginning of value", "invalid_json"},
	} {
		_, err := ParsePhasePlan(c.in)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: %v, want %q", c.in, err, c.want)
		}
		if pf, ok := err.(*parseFailure); !ok || pf.cause != c.cause {
			t.Errorf("%q: classified %q: %#v", c.in, c.cause, err)
		}
	}
	t.Run("the routing-eval corpus parses as before", func(t *testing.T) {
		files, err := filepath.Glob(filepath.Join("..", "..", "routingeval", "testdata", "*", "response.txt"))
		if err != nil || len(files) != 7 {
			t.Fatalf("corpus: %v %v", files, err)
		}
		for _, f := range files {
			body, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			_, perr := ParsePhasePlan(string(body))
			if wantErr := strings.Contains(f, "07-unparseable-response"); (perr != nil) != wantErr {
				t.Errorf("%s: parse error %v, want error=%v", f, perr, wantErr)
			}
		}
	})
}

// Test 24 — exactly the four canonical tiers survive; aliases, raw models and
// garbage drop to "". SanitizeTier PROJECTS modelcatalog.CanonicalTiers (the
// vocabulary's one home — the catalog is stdlib-only, so the import is
// free); the literal lists here are the consumer pin: a tier added to or
// removed from the catalog turns this red instead of silently widening or
// narrowing what the advisor may propose. The source pin keeps the leaf from
// growing a second spelling of the four (review fold, arch HIGH-1).
func TestSanitizeTier_KeepsExactlyTheFourCanonicalTiers(t *testing.T) {
	for _, good := range []string{"fast", "balanced", "deep", "top"} {
		if SanitizeTier(good) != good {
			t.Errorf("SanitizeTier(%q) must pass through", good)
		}
	}
	for _, bad := range []string{"high", "HIGH", "opus", "claude-fable-5", "", " deep", "ultra-mega-tier", "sonnet"} {
		if got := SanitizeTier(bad); got != "" {
			t.Errorf("SanitizeTier(%q) = %q, want \"\"", bad, got)
		}
	}
	if len(modelcatalog.CanonicalTiers) != 4 {
		t.Errorf("the catalog vocabulary drifted from the four the advisor clamps to: %v", modelcatalog.CanonicalTiers)
	}
	src, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), `"fast", "balanced", "deep", "top"`) {
		t.Error("parse.go spells the tier vocabulary itself: SanitizeTier must project modelcatalog.CanonicalTiers")
	}
}

// Test 36 — the last-balanced-span scanner is string-literal aware and
// returns the LAST top-level span; the plan parser ignores the prompt's
// echoed example and leaves a clean body unchanged.
func TestLastBalancedSpan_StringLiteralAwareLastSpanWins(t *testing.T) {
	cases := []struct {
		name               string
		s                  string
		open, close        byte
		wantStart, wantEnd int
		wantOK             bool
	}{
		{"single array", "[1,2]", '[', ']', 0, 4, true},
		{"single object", `{"a":1}`, '{', '}', 0, 6, true},
		{"leading prose", "answer: [1]", '[', ']', 8, 10, true},
		{"two arrays takes last", "[1] then [2,3]", '[', ']', 9, 13, true},
		{"nested takes outermost-of-last", "x [ [1] ] y [ [2] ]", '[', ']', 12, 18, true},
		{"no close", "[1,2", '[', ']', 0, 0, false},
		{"no open match", "abc]", '[', ']', 0, 0, false},
		{"close byte inside string", `{"k":"a } b"}`, '{', '}', 0, 12, true},
		{"open+close inside string", `{"k":"[x] {y}"}`, '{', '}', 0, 14, true},
		{"escaped quote in string", `{"k":"she said \"hi\" }"}`, '{', '}', 0, 24, true},
	}
	for _, c := range cases {
		start, end, ok := LastBalancedSpan(c.s, c.open, c.close)
		if ok != c.wantOK || (ok && (start != c.wantStart || end != c.wantEnd)) {
			t.Errorf("%s: span = [%d,%d] ok=%v, want [%d,%d] ok=%v", c.name, start, end, ok, c.wantStart, c.wantEnd, c.wantOK)
		}
	}
}

func TestParsePhasePlan_IgnoresPromptEchoExample(t *testing.T) {
	scrollback := `## Respond with STRICT JSON only (a bare array):
[{"phase":"<phase>","run":true,"justification":"<one sentence>"},...]

⏺ [
  {"phase":"scout","run":true,"justification":"start of cycle"},
  {"phase":"build","run":true,"justification":"mandatory"},
  {"phase":"audit","run":true,"justification":"floor requires it"},
  {"phase":"ship","run":true,"justification":"completes spine"}
]

✻ Crunched for 8s
❯`
	parsed, err := ParsePhasePlan(scrollback)
	if err != nil || len(parsed.Plan.Entries) != 4 || parsed.Plan.Entries[0].Phase != "scout" {
		t.Fatalf("the reply, not the 1-entry prompt example: %+v %v", parsed.Plan, err)
	}
}

func TestParsePhasePlan_CleanBodyUnchanged(t *testing.T) {
	parsed, err := ParsePhasePlan(planJSON())
	if err != nil || len(parsed.Plan.Entries) != 1 || parsed.Plan.Entries[0].Phase != "scout" || parsed.Plan.MintPhases != nil || parsed.RejectedMints != nil {
		t.Fatalf("clean body: %+v %v", parsed, err)
	}
}

// Test 37 — replay equals parse + the real floor clamp; an unparseable
// capture is a loud error; a reserved-name mint is dropped silently.
func TestReplayPlanFromResponse_EqualsParsePlusClampAndDropsAreSilent(t *testing.T) {
	raw := `[{"phase":"audit","run":false,"justification":"skip"},{"phase":"ship","run":true,"justification":"done"},{"phase":"router","run":true,"mint":{"prompt":"x"}}]`
	in := router.RouteInput{}
	floor := router.DefaultShipFloor()
	clamped, clamps, err := ReplayPlanFromResponse(raw, in, floor)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	runs := func(p *router.PhasePlan, phase string) bool {
		for _, e := range p.Entries {
			if e.Phase == phase {
				return e.Run
			}
		}
		return false
	}
	if !runs(clamped, "audit") || len(clamps) == 0 {
		t.Errorf("replay clamps audit ON over the real floor: %+v %+v", clamped.Entries, clamps)
	}
	parsed, perr := ParsePhasePlan(raw)
	if perr != nil {
		t.Fatal(perr)
	}
	wantClamped, _ := router.ClampPlanToFloorWith(in, parsed.Plan, floor, in.IntentRequired)
	if !reflect.DeepEqual(clamped.Entries, wantClamped.Entries) {
		t.Errorf("replay == the live parse+clamp\n got %+v\nwant %+v", clamped.Entries, wantClamped.Entries)
	}
	if len(clamped.MintPhases) != 0 {
		t.Errorf("the reserved mint is dropped silently on replay: %+v", clamped.MintPhases)
	}
	again, _, _ := ReplayPlanFromResponse(raw, in, floor)
	if !reflect.DeepEqual(clamped.Entries, again.Entries) {
		t.Error("replay must be deterministic")
	}
	if _, _, err := ReplayPlanFromResponse("not json at all", in, floor); err == nil || err.Error() != "no JSON array in plan output" {
		t.Errorf("an unparseable response errors unwrapped: %v", err)
	}
}
