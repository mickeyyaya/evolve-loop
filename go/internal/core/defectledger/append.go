package defectledger

import "github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"

type rowKey struct{ source, text string }

func keyOf(e Entry) rowKey { return rowKey{e.Source, e.Text} }

func Append(doc Doc, rows []Entry, cycle int) (Doc, bool, int) {
	out := Doc{OriginCycle: doc.OriginCycle, Entries: append([]Entry(nil), doc.Entries...)}
	known := make(map[rowKey]bool, len(out.Entries)+len(rows))
	held := make(map[string]int)
	for _, e := range out.Entries {
		known[keyOf(e)] = true
		held[e.Source]++
	}
	var cut []Entry
	added := false
	for _, row := range rows {
		row.Text = Truncate(row.Text, TextMaxRunes)
		if known[keyOf(row)] {
			continue
		}
		if held[row.Source] >= MaxEntries {
			cut = append(cut, row)
			continue
		}
		known[keyOf(row)] = true
		held[row.Source]++
		row.ID = rowID(row)
		out.Entries = append(out.Entries, row)
		added = true
	}
	for _, group := range bySource(cut) {
		if tail := standInFor(group, cycle); !known[keyOf(tail)] {
			out.Entries = append(out.Entries, tail)
			added = true
		}
	}
	return out, added, len(cut)
}

const sourceSeparator = "\xff"

func rowID(e Entry) string {
	if e.Source == "" {
		return ID(e.Text)
	}
	return ID(e.Source + sourceSeparator + e.Text)
}

func qualified(source, text string) string {
	if source == "" {
		return text
	}
	return source + ": " + text
}

func bySource(rows []Entry) [][]Entry {
	at := make(map[string]int)
	var groups [][]Entry
	for _, r := range rows {
		i, seen := at[r.Source]
		if !seen {
			i = len(groups)
			at[r.Source] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], r)
	}
	return groups
}

func standInFor(cut []Entry, cycle int) Entry {
	tail := overflowRow(len(cut), cycle)
	tail.Status, tail.Reason, tail.Source, tail.Round = cut[0].Status, cut[0].Reason, cut[0].Source, cut[0].Round
	tail.Text = qualified(tail.Source, tail.Text)
	tail.ID = rowID(tail)
	tail.Severity = highestSeverity(cut)
	return tail
}

func highestSeverity(rows []Entry) string {
	highest := ""
	for _, r := range rows {
		if r.Severity != "" && (highest == "" || reportdoc.SeverityRank(r.Severity) < reportdoc.SeverityRank(highest)) {
			highest = r.Severity
		}
	}
	return highest
}
