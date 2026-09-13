package observerengine

import "encoding/json"

// processLine decodes one stream-json line: an empty or malformed line is
// skipped without touching the clocks; a valid one counts as an event and
// refreshes the idle clock (one clock read per valid line), then dispatches
// on its type.
func (e *Engine) processLine(line string) {
	if line == "" {
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		return // malformed JSON — skip
	}
	e.eventCount++
	e.lastEventTS = e.d.Now()
	t, _ := doc["type"].(string)
	switch t {
	case "assistant":
		e.applyAssistant(doc)
	case "user":
		e.applyUser(doc)
	case "result":
		e.applyResult(doc)
	case "rate_limit_event":
		e.rateLimitCnt++
	}
}

// firstBlock is the preserved quirk: only message.content[0] is inspected
// (a text block followed by a tool_use counts no progress).
func firstBlock(doc map[string]any) map[string]any {
	msg, _ := doc["message"].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) == 0 {
		return nil
	}
	block, _ := content[0].(map[string]any)
	return block
}

// applyAssistant counts a tool dispatch as a call and as meaningful progress.
func (e *Engine) applyAssistant(doc map[string]any) {
	if btype, _ := firstBlock(doc)["type"].(string); btype == "tool_use" {
		e.toolCallCount++
		e.lastProgressTS = e.lastEventTS // WS-E1: tool dispatch is meaningful progress
	}
}

// applyUser counts a tool result as progress and an is_error result as an
// error.
func (e *Engine) applyUser(doc map[string]any) {
	block := firstBlock(doc)
	if rtype, _ := block["type"].(string); rtype != "tool_result" {
		return
	}
	e.toolResultCnt++
	e.lastProgressTS = e.lastEventTS // WS-E1: tool return is meaningful progress
	if isErr, _ := block["is_error"].(bool); isErr {
		e.errorCount++
	}
}

// applyResult accumulates the cost and the cache-token counters.
func (e *Engine) applyResult(doc map[string]any) {
	if cost, ok := doc["total_cost_usd"].(float64); ok {
		e.cumulativeCost += cost
	}
	usage, _ := doc["usage"].(map[string]any)
	if cr, ok := usage["cache_read_input_tokens"].(float64); ok {
		e.cacheReadTok += int(cr)
	}
	if cc, ok := usage["cache_creation_input_tokens"].(float64); ok {
		e.cacheCreateTok += int(cc)
	}
}
