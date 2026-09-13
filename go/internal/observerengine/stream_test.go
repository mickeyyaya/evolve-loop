package observerengine

// stream_test.go — §6 test 18: the stream-json decoder. Eight tests MOVED from
// the host (phaseobserver_test.go:94-154, coverage_test.go:162-207) with the
// four loopHistory/rateLimitHist assertions rewritten to the counters they
// imply (declared edited moves — D2 deleted the write-only accumulators), plus
// the preserved content[0] quirk and the per-line clock read pinned.

import (
	"testing"
	"time"
)

func streamEngine(t *testing.T) *Engine {
	t.Helper()
	e, _, _ := newEngine(t, nil)
	return e
}

// moved from phaseobserver_test.go:94 — loopHistory[0].tool == "Bash" rewritten
// to toolCallCount/lastProgressTS. Kills M13a (the tool_use arm).
func TestProcessLine_AssistantToolUse(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`)
	if e.toolCallCount != 1 || e.eventCount != 1 {
		t.Errorf("tool_call_count = %d, event_count = %d, want 1/1", e.toolCallCount, e.eventCount)
	}
	if e.lastProgressTS != e.lastEventTS {
		t.Error("tool dispatch is meaningful progress: lastProgressTS follows lastEventTS")
	}
}

// moved from phaseobserver_test.go:112. Kills M13b (the tool_result arm) and
// M13c (is_error not counted).
func TestProcessLine_UserToolResultError(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true}]}}`)
	if e.toolResultCnt != 1 || e.errorCount != 1 {
		t.Errorf("expected tool_result and error increments; got tr=%d err=%d", e.toolResultCnt, e.errorCount)
	}
	if e.lastProgressTS != e.lastEventTS {
		t.Error("a tool return is meaningful progress")
	}
}

// moved from phaseobserver_test.go:122. Kills M13d (the result arm).
func TestProcessLine_ResultEventAccumulatesCost(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"result","total_cost_usd":0.45,"usage":{"cache_read_input_tokens":1024,"cache_creation_input_tokens":256}}`)
	e.processLine(`{"type":"result","total_cost_usd":0.05}`)
	if e.cumulativeCost != 0.5 {
		t.Errorf("cost = %v, want 0.5 (accumulated)", e.cumulativeCost)
	}
	if e.cacheReadTok != 1024 || e.cacheCreateTok != 256 {
		t.Errorf("cache tokens wrong: r=%d c=%d", e.cacheReadTok, e.cacheCreateTok)
	}
}

// moved from phaseobserver_test.go:134 — the rateLimitHist assertion rewritten
// to the counter. Kills M13e (the rate_limit arm).
func TestProcessLine_RateLimitEventTracked(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"rate_limit_event","reason":"quota"}`)
	if e.rateLimitCnt != 1 || e.eventCount != 1 {
		t.Errorf("rate_limit_count = %d, event_count = %d, want 1/1", e.rateLimitCnt, e.eventCount)
	}
	if e.lastProgressTS != fixtureAt {
		t.Error("a rate-limit event is not progress")
	}
}

// moved from phaseobserver_test.go:146. Kills M13f (the malformed skip).
func TestProcessLine_MalformedJSONSkipped(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine("not json at all")
	e.processLine("")
	if e.eventCount != 0 {
		t.Errorf("malformed/empty should not count, got %d", e.eventCount)
	}
}

// moved from coverage_test.go:165 — len(loopHistory) == 0 rewritten to
// toolCallCount == 0.
func TestProcessLine_AssistantEmptyContent(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"assistant","message":{"content":[]}}`)
	if e.eventCount != 1 {
		t.Errorf("eventCount = %d, want 1 (line counted)", e.eventCount)
	}
	if e.toolCallCount != 0 {
		t.Errorf("empty content must not record a tool call; tc=%d", e.toolCallCount)
	}
}

// rewritten from coverage_test.go:181 (ToolUseMissingNameDefaultsQuestion): the
// "?" default fell with loopHistory (D2); what stays observable is that a
// nameless tool_use still counts as a call and as progress.
func TestProcessLine_NamelessToolUseStillCountsProgress(t *testing.T) {
	t.Parallel()
	e, _, _ := newEngine(t, func(_ *Settings, d *Deps) {
		calls := 0
		d.Now = func() time.Time {
			calls++
			if calls == 1 {
				return fixtureAt
			}
			return fixtureAt.Add(time.Duration(calls) * time.Second)
		}
	})
	e.processLine(`{"type":"assistant","message":{"content":[{"type":"tool_use","input":{}}]}}`)
	if e.toolCallCount != 1 {
		t.Fatalf("toolCallCount = %d, want 1", e.toolCallCount)
	}
	if e.lastProgressTS == fixtureAt || e.lastProgressTS != e.lastEventTS {
		t.Errorf("progress clock bumped to the line's read: %v / %v", e.lastProgressTS, e.lastEventTS)
	}
}

// moved from coverage_test.go:196.
func TestProcessLine_UserEmptyContent(t *testing.T) {
	t.Parallel()
	e := streamEngine(t)
	e.processLine(`{"type":"user","message":{"content":[]}}`)
	if e.eventCount != 1 {
		t.Errorf("eventCount = %d, want 1", e.eventCount)
	}
	if e.toolResultCnt != 0 || e.errorCount != 0 {
		t.Errorf("empty user content must not record a tool result; tr=%d err=%d", e.toolResultCnt, e.errorCount)
	}
}

// TestProcessLine_OnlyTheFirstContentBlockCounts pins the preserved quirk:
// only content[0] is inspected, so a text block followed by a tool_use counts
// no progress (a possible false stuck_no_progress). Kills M14 (a range over
// the content blocks).
func TestProcessLine_OnlyTheFirstContentBlockCounts(t *testing.T) {
	t.Parallel()
	e, _, _ := newEngine(t, func(_ *Settings, d *Deps) { d.Now = fixedClock(fixtureAt.Add(time.Minute)) })
	e.lastProgressTS = fixtureAt // construction read the shifted clock; pin the baseline
	e.processLine(`{"type":"assistant","message":{"content":[{"type":"text","text":"thinking"},{"type":"tool_use","name":"Edit","input":{}}]}}`)
	e.processLine(`{"type":"user","message":{"content":[{"type":"text","text":"x"},{"type":"tool_result"}]}}`)
	if e.toolCallCount != 0 || e.toolResultCnt != 0 {
		t.Errorf("content[0] only: tc=%d tr=%d", e.toolCallCount, e.toolResultCnt)
	}
	if e.lastProgressTS != fixtureAt {
		t.Error("no progress recorded when the first block is text")
	}
	if e.eventCount != 2 || e.lastEventTS != fixtureAt.Add(time.Minute) {
		t.Errorf("both lines still count as events: %d %v", e.eventCount, e.lastEventTS)
	}
}

// TestProcessLine_LastEventTSReadsTheClockPerLine pins the :469 clock site:
// every VALID line reads the clock exactly once (lastEventTS is that read); a
// malformed or empty line reads nothing. Load-bearing for the host's
// count-stepping clocks (design §8).
func TestProcessLine_LastEventTSReadsTheClockPerLine(t *testing.T) {
	t.Parallel()
	reads := 0
	e, _, _ := newEngine(t, func(_ *Settings, d *Deps) {
		d.Now = func() time.Time {
			reads++
			return fixtureAt.Add(time.Duration(reads) * time.Second)
		}
	})
	reads = 0 // construction read consumed
	e.processLine("garbage")
	e.processLine("")
	if reads != 0 {
		t.Fatalf("a skipped line must not read the clock; reads=%d", reads)
	}
	e.processLine(`{"type":"result"}`)
	e.processLine(`{"type":"unknown_kind"}`)
	if reads != 2 || e.lastEventTS != fixtureAt.Add(2*time.Second) {
		t.Errorf("one read per valid line: reads=%d lastEventTS=%v", reads, e.lastEventTS)
	}
}
