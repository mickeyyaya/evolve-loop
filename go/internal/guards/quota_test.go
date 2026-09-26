package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestQuota_Name(t *testing.T) {
	g := NewQuota(QuotaConfig{})
	if g.Name() != "quota" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestQuota_DefaultCapDeniesOverlimit(t *testing.T) {
	g := NewQuota(QuotaConfig{}) // defaults: ws=3
	for i := 0; i < 3; i++ {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "WebSearch",
			ToolInput: map[string]any{"agent": "scout"},
		})
		if !dec.Allow {
			t.Fatalf("call %d should allow: %s", i, dec.Reason)
		}
	}
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "scout"},
	})
	if dec.Allow {
		t.Error("4th WebSearch by scout must deny")
	}
}

func TestQuota_DeepResearchBypass(t *testing.T) {
	g := NewQuota(QuotaConfig{AllowDeepResearch: true})
	for i := 0; i < 10; i++ {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "WebSearch",
			ToolInput: map[string]any{"agent": "scout"},
		})
		if !dec.Allow {
			t.Errorf("call %d with bypass denied: %s", i, dec.Reason)
		}
	}
}

func TestQuota_BucketsIsolated(t *testing.T) {
	g := NewQuota(QuotaConfig{WebSearch: 1, WebFetch: 1})
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "x"},
	})
	if !dec.Allow {
		t.Fatal("first WebSearch denied")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "x"},
	})
	if dec.Allow {
		t.Error("second WebSearch must deny (cap=1)")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebFetch",
		ToolInput: map[string]any{"agent": "x"},
	})
	if !dec.Allow {
		t.Errorf("first WebFetch denied: %s", dec.Reason)
	}
}

func TestQuota_AgentsIsolated(t *testing.T) {
	g := NewQuota(QuotaConfig{WebSearch: 1})
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "scout"},
	})
	if !dec.Allow {
		t.Fatal("scout first denied")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "scout"},
	})
	if dec.Allow {
		t.Fatal("scout second must deny")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{"agent": "auditor"},
	})
	if !dec.Allow {
		t.Errorf("auditor first denied (different agent counter): %s", dec.Reason)
	}
}

func TestQuota_NonResearchToolsPass(t *testing.T) {
	g := NewQuota(QuotaConfig{WebSearch: 0})
	for _, tool := range []string{"Edit", "Write", "Read", "Glob"} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  tool,
			ToolInput: map[string]any{"agent": "x"},
		})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

func TestQuota_KbSearchViaBash(t *testing.T) {
	g := NewQuota(QuotaConfig{KbSearch: 1})
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "bash scripts/research/kb-search.sh 'foo'", "agent": "scout"},
	})
	if !dec.Allow {
		t.Fatal("first kb-search denied")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "bash scripts/research/kb-search.sh 'bar'", "agent": "scout"},
	})
	if dec.Allow {
		t.Error("second kb-search must deny (cap=1)")
	}
}

func TestQuota_UnknownAgentFallback(t *testing.T) {
	g := NewQuota(QuotaConfig{WebSearch: 1})
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{},
	})
	if !dec.Allow {
		t.Fatal("first call denied")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "WebSearch",
		ToolInput: map[string]any{},
	})
	if dec.Allow {
		t.Error("second call must deny (unknown agent cap=1)")
	}
}

func TestQuota_DefaultCapsUsedWhenZero(t *testing.T) {
	g := NewQuota(QuotaConfig{})
	if g.cfg.WebSearch != 3 || g.cfg.WebFetch != 5 || g.cfg.KbSearch != 20 {
		t.Errorf("defaults not applied: %+v", g.cfg)
	}
}

func TestQuota_RegularBashPasses(t *testing.T) {
	g := NewQuota(QuotaConfig{KbSearch: 0})
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "ls", "agent": "x"},
	})
	if !dec.Allow {
		t.Errorf("regular Bash denied under cap=0: %s", dec.Reason)
	}
}
