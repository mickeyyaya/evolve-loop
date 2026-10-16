package signalcenter

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

const (
	CodeUnknownModule    Code = "SIGNALCENTER_UNKNOWN_MODULE"
	CodeUnknownKind      Code = "SIGNALCENTER_UNKNOWN_KIND"
	CodeUnknownSeverity  Code = "SIGNALCENTER_UNKNOWN_SEVERITY"
	CodeMissingCode      Code = "SIGNALCENTER_MISSING_CODE"
	CodeUnregisteredCode Code = "SIGNALCENTER_UNREGISTERED_CODE"
	CodeMissingReason    Code = "SIGNALCENTER_MISSING_REASON"
	CodeBadOrigin        Code = "SIGNALCENTER_BAD_ORIGIN"
	CodeListenerPanicked Code = "SIGNALCENTER_LISTENER_PANICKED"
	CodeSinkDropped      Code = "SIGNALCENTER_SINK_DROPPED"
)

var fieldKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func Normalize(e Event) (Event, []Code) {
	var drift []Code
	fields := boundFields(e.Fields)
	keep := func(k, v string) {
		if fields == nil {
			fields = map[string]string{}
		}
		fields[k] = v
	}
	origModule := e.Module
	drift = checkVocabulary(&e, drift, keep)
	if origModule.Known() {
		drift = checkCode(&e, origModule, drift, keep)
	}
	drift = checkText(&e, drift, keep)
	if len(drift) > 0 {
		if e.Code != "" && e.Code != drift[0] {
			keep("raw_code", string(e.Code))
		}
		e.Code = drift[0]
		keep("drift", joinCodes(drift))
		if !e.Severity.AtLeast(SeverityWarn) {
			e.Severity = SeverityWarn
		}
	}
	e.Fields = fields
	return capLine(e), drift
}

func checkCode(e *Event, origModule Module, drift []Code, keep func(k, v string)) []Code {
	if e.Code == "" {
		if e.Severity.AtLeast(SeverityWarn) {
			drift = append(drift, CodeMissingCode)
		}
		return drift
	}
	owner, ok := IsRegistered(e.Code)
	if ok && owner == origModule {
		return drift
	}
	drift = append(drift, CodeUnregisteredCode)
	if ok {
		keep("registered_module", string(owner))
	}
	return drift
}

func joinCodes(codes []Code) string {
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = string(c)
	}
	return strings.Join(parts, ",")
}

func boundFields(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(in))
	dropped := 0
	for _, k := range keys {
		if !fieldKeyRE.MatchString(k) || len(out) >= MaxFields {
			dropped++
			continue
		}
		out[k] = log.SanitizeField(in[k])
	}
	return recordTruncated(out, keys, dropped)
}

func recordTruncated(out map[string]string, sorted []string, dropped int) map[string]string {
	if dropped == 0 {
		return out
	}
	if len(out) >= MaxFields {
		for i := len(sorted) - 1; i >= 0; i-- {
			if _, kept := out[sorted[i]]; kept {
				delete(out, sorted[i])
				dropped++
				break
			}
		}
	}
	out["truncated"] = strconv.Itoa(dropped)
	return out
}

func capLine(e Event) Event {
	for lineBytes(e) > MaxLineBytes && largestField(e.Fields) != "" {
		delete(e.Fields, largestField(e.Fields))
		n, _ := strconv.Atoi(e.Fields["truncated"])
		e.Fields["truncated"] = strconv.Itoa(n + 1)
	}
	for over := lineBytes(e) - MaxLineBytes; over > 0 && e.Reason != ""; over = lineBytes(e) - MaxLineBytes {
		e.Reason = cutRunes(e.Reason, over)
	}
	return e
}

const maxJSONBytesPerRune = 6

func cutRunes(s string, n int) string {
	r := []rune(s)
	keep := len(r) - n/maxJSONBytesPerRune - 2
	if keep <= 0 {
		return ""
	}
	return string(r[:keep]) + "…"
}

func capRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func largestField(fields map[string]string) string {
	largest, size := "", -1
	for k, v := range fields {
		if k != "truncated" && len(k)+len(v) > size {
			largest, size = k, len(k)+len(v)
		}
	}
	return largest
}

func lineBytes(e Event) int {
	raw, _ := json.Marshal(e)
	return len(raw)
}

func checkVocabulary(e *Event, drift []Code, keep func(k, v string)) []Code {
	if !e.Module.Known() {
		drift = append(drift, CodeUnknownModule)
		keep("raw_module", string(e.Module))
		e.Module = ModuleSignalCenter
	}
	if !e.Kind.Known() {
		drift = append(drift, CodeUnknownKind)
		keep("raw_kind", string(e.Kind))
		e.Kind = KindRegistryDrift
	}
	if !e.Severity.Valid() {
		drift = append(drift, CodeUnknownSeverity)
		keep("raw_severity", string(e.Severity))
		e.Severity = SeverityWarn
	}
	return drift
}

func checkText(e *Event, drift []Code, keep func(k, v string)) []Code {
	e.Reason = log.SanitizeField(e.Reason)
	e.Origin = capRunes(log.SanitizeField(e.Origin), MaxIdentRunes)
	e.Phase = capRunes(log.SanitizeField(e.Phase), MaxIdentRunes)
	e.RunID = capRunes(log.SanitizeField(e.RunID), MaxIdentRunes)
	if strings.TrimSpace(e.Reason) == "" {
		drift = append(drift, CodeMissingReason)
		e.Reason = "(no reason given)"
	}
	if !ValidOrigin(e.Origin) {
		drift = append(drift, CodeBadOrigin)
		keep("raw_origin", e.Origin)
		e.Origin = "unknown"
	}
	return drift
}
