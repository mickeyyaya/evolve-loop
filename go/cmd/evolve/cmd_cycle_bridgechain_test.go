package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The raw br may only be constructed, configured through its Set* methods,
// wrapped, used as the catalog publisher's sink and exposed on orchDeps; never launched.
func TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func wireOrchestratorDeps(")
	if start < 0 {
		t.Fatal("wireOrchestratorDeps not found")
	}
	end := strings.Index(body[start:], "\n}\n")
	fn := body[start : start+end]
	if n := strings.Count(fn, "bridgechain.New("); n != 1 {
		t.Fatalf("the raw bridge is wrapped exactly once (got %d)", n)
	}
	rawUse := regexp.MustCompile(`\bbr\b`)
	allowed := []*regexp.Regexp{
		regexp.MustCompile(`^\s*br := bridge\.NewDefault\(`),
		regexp.MustCompile(`^\s*br\.Set[A-Za-z]+\(`),
		regexp.MustCompile(`bridgechain\.New\(br,`),
		regexp.MustCompile(`catalogPublisher\(br\)`),   // the concrete adapter's contract-resolver sink — configured, never launched
		regexp.MustCompile(`wireBridgeStages\(br, `),   // takes a bridgeStageSink (three setters, no Launch) — a configurer by type
		regexp.MustCompile(`^\s*Bridge:\s{2,}br,\s*$`), // orchDeps.Bridge (*bridge.Adapter): the host's own Set* handle — never launched
	}
	for i, line := range strings.Split(fn, "\n") {
		if !rawUse.MatchString(line) || strings.Contains(strings.TrimSpace(line), "//") && strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		ok := false
		for _, re := range allowed {
			if re.MatchString(line) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("line %d hands the RAW bridge to a consumer — use the chain-walking handle: %s", i+1, strings.TrimSpace(line))
		}
	}
}
