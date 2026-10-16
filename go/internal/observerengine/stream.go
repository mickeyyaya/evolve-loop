package observerengine

import "encoding/json"

func (e *Engine) processLine(line string) {
	if line == "" {
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		return
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

func firstBlock(doc map[string]any) map[string]any {
	msg, _ := doc["message"].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) == 0 {
		return nil
	}
	block, _ := content[0].(map[string]any)
	return block
}

func (e *Engine) applyAssistant(doc map[string]any) {
	if btype, _ := firstBlock(doc)["type"].(string); btype == "tool_use" {
		e.toolCallCount++
		e.lastProgressTS = e.lastEventTS
	}
}

func (e *Engine) applyUser(doc map[string]any) {
	block := firstBlock(doc)
	if rtype, _ := block["type"].(string); rtype != "tool_result" {
		return
	}
	e.toolResultCnt++
	e.lastProgressTS = e.lastEventTS
	if isErr, _ := block["is_error"].(bool); isErr {
		e.errorCount++
	}
}

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
