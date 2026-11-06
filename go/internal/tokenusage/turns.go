package tokenusage

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type Turn struct {
	Index        int
	Timestamp    string
	Usage        cyclestate.TokenUsage
	PromptTokens int
	Tools        []string
}

type messageTurn struct {
	usage     cyclestate.TokenUsage
	timestamp string
	tools     []string
	toolIDs   map[string]bool
	seq       int
}

type toolUse struct {
	id   string
	name string
}

func (m *messageTurn) addTools(uses []toolUse) {
	for _, u := range uses {
		if u.id != "" {
			if m.toolIDs[u.id] {
				continue
			}
			if m.toolIDs == nil {
				m.toolIDs = map[string]bool{}
			}
			m.toolIDs[u.id] = true
		}
		m.tools = append(m.tools, u.name)
	}
}

func orderedTurns(perMsg map[string]messageTurn) []Turn {
	msgs := make([]messageTurn, 0, len(perMsg))
	for _, m := range perMsg {
		msgs = append(msgs, m)
	}
	sort.Slice(msgs, func(i, j int) bool {
		ti, tj := parsedTime(msgs[i].timestamp), parsedTime(msgs[j].timestamp)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return msgs[i].seq < msgs[j].seq
	})
	turns := make([]Turn, len(msgs))
	for i, m := range msgs {
		turns[i] = Turn{Index: i + 1, Timestamp: m.timestamp, Usage: m.usage, PromptTokens: PromptTokens(m.usage), Tools: m.tools}
	}
	return turns
}

func parsedTime(ts string) time.Time {
	t, _ := time.Parse(time.RFC3339, ts)
	return t
}

type contentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toolUses(content json.RawMessage) []toolUse {
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	var uses []toolUse
	for _, b := range blocks {
		if b.Type == "tool_use" && b.Name != "" {
			uses = append(uses, toolUse{id: b.ID, name: b.Name})
		}
	}
	return uses
}

func messageUsages(perMsg map[string]messageTurn) []cyclestate.TokenUsage {
	usages := make([]cyclestate.TokenUsage, 0, len(perMsg))
	for _, m := range perMsg {
		usages = append(usages, m.usage)
	}
	return usages
}
