package tokenusage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	SourceAgyConversation Source = "agy_conversation"

	agyTimeLayout   = "2006-01-02 15:04:05.000000"
	agyQueryTimeout = 10 * time.Second

	agyGenerationField = 1
	agyUsageField      = 4
	agyInputField      = 2
	agyOutputField     = 3
	agyCacheReadField  = 5
)

type SQLiteQuery func(db, query string) (string, error)

var agyConversationID = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)

var errMalformedProto = errors.New("tokenusage: malformed agy generation record")

func AgyDataRoot(env map[string]string) string {
	home := envOrProcess(env, "HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli")
}

func SQLiteCLI(db, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), agyQueryTimeout)
	defer cancel()
	out, err := sysexec.Command(ctx, "sqlite3", "-readonly", "-noheader", db, query).Output()
	return string(out), err
}

func AgyConversationCollector(root string, w Window, query SQLiteQuery) Collector {
	return func() Result {
		if root == "" || w.Worktree == "" || w.Start.IsZero() || w.End.IsZero() {
			return Result{Source: SourceNone}
		}
		ids, err := query(filepath.Join(root, "conversation_summaries.db"), agyConversationsSQL(w))
		if err != nil {
			return Result{Source: SourceNone}
		}
		var r Result
		for _, id := range strings.Fields(ids) {
			if agyConversationID.MatchString(id) {
				r = addAgyConversation(r, root, id, query)
			}
		}
		if r.Usage.Output == 0 {
			return Result{Source: SourceNone}
		}
		r.Source = SourceAgyConversation
		return r
	}
}

func agyConversationsSQL(w Window) string {
	return fmt.Sprintf("select conversation_id from conversation_summaries where instr(workspace_uris, '\"file://%s\"') > 0 and last_user_input_time >= '%s' and last_user_input_time <= '%s' order by last_user_input_time;",
		strings.ReplaceAll(w.Worktree, "'", "''"), w.Start.UTC().Format(agyTimeLayout), w.End.UTC().Format(agyTimeLayout))
}

func addAgyConversation(r Result, root, id string, query SQLiteQuery) Result {
	rows, err := query(filepath.Join(root, "conversations", id+".db"), "select hex(data) from gen_metadata order by idx;")
	if err != nil {
		r.Warn = joinAgyWarn(r.Warn, fmt.Sprintf("agy conversation %s unread (%v); its usage is not counted", id, err))
		return r
	}
	for _, row := range strings.Fields(rows) {
		u, err := agyGenerationUsage(row)
		if err != nil {
			r.Warn = joinAgyWarn(r.Warn, fmt.Sprintf("agy conversation %s has a malformed generation row (%v); its usage is not counted", id, err))
			continue
		}
		r.Usage = cyclestate.TokenUsage{Input: r.Usage.Input + u.Input, Output: r.Usage.Output + u.Output, CacheRead: r.Usage.CacheRead + u.CacheRead}
		if prompt := u.Input + u.CacheRead; prompt > r.PeakPromptTokens {
			r.PeakPromptTokens, r.PeakUsage = prompt, u
		}
	}
	return r
}

func agyGenerationUsage(row string) (cyclestate.TokenUsage, error) {
	raw, err := hex.DecodeString(row)
	if err != nil {
		return cyclestate.TokenUsage{}, err
	}
	var u cyclestate.TokenUsage
	err = walkFields(raw, func(gen protoField) error {
		if gen.num != agyGenerationField || gen.wire != wireBytes {
			return nil
		}
		return walkFields(gen.bytes, func(usage protoField) error {
			if usage.num != agyUsageField || usage.wire != wireBytes {
				return nil
			}
			return walkFields(usage.bytes, func(c protoField) error {
				u = addAgyCounter(u, c)
				return nil
			})
		})
	})
	return u, err
}

func addAgyCounter(u cyclestate.TokenUsage, c protoField) cyclestate.TokenUsage {
	if c.wire != wireVarint {
		return u
	}
	switch c.num {
	case agyInputField:
		u.Input += int(c.varint)
	case agyOutputField:
		u.Output += int(c.varint)
	case agyCacheReadField:
		u.CacheRead += int(c.varint)
	}
	return u
}

func joinAgyWarn(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}
