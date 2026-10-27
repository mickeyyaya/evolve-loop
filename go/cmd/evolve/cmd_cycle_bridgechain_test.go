package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge(t *testing.T) {
	t.Parallel()
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
		regexp.MustCompile(`bridgechain\.New\(withUsageEvidence\(br, explainUsage, signals\),`),
		regexp.MustCompile(`catalogPublisher\(br, cliRouter\)`),
		regexp.MustCompile(`wireBridgeStages\(br, `),
		regexp.MustCompile(`applySandboxOff\(br, `),
		regexp.MustCompile(`^\s*Bridge:\s{2,}br,\s*$`),
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
