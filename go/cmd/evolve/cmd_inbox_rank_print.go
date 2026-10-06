package main

import (
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func rankedCounts(lists []rankedList) map[string]int {
	counts := make(map[string]int, len(lists))
	for _, l := range lists {
		counts[l.List] = len(l.Items)
	}
	return counts
}

func printRankedLists(w io.Writer, lists []rankedList, counts map[string]int) {
	for i, l := range lists {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s: %d item(s)", l.List, counts[l.List])
		if len(l.Items) < counts[l.List] {
			fmt.Fprintf(w, ", top %d shown", len(l.Items))
		}
		fmt.Fprintf(w, "\n%6s  %6s", "rank", "score")
		for _, f := range inboxrank.Factors() {
			fmt.Fprintf(w, "  %10s", f)
		}
		fmt.Fprintln(w, "  id  title")
		for _, row := range l.Items {
			fmt.Fprintf(w, "%6d  %6.4f", row.Rank, row.Score)
			for _, term := range row.Terms {
				fmt.Fprintf(w, "  %10.4f", term.Contribution)
			}
			fmt.Fprintf(w, "  %s  %s\n", row.ID, row.Title)
		}
	}
}

func explainRanked(ranked rankedInbox, req inboxRankRequest, stdout, stderr io.Writer) int {
	for _, l := range ranked.doc.Lists {
		at := slices.IndexFunc(l.Items, func(row rankedRow) bool { return row.ID == req.explain })
		if at < 0 {
			continue
		}
		doc := explainedItemDoc{List: l.List, Of: len(l.Items), rankedRow: l.Items[at]}
		if req.asJSON {
			return encodeInboxJSON("rank", doc, stdout, stderr)
		}
		printExplained(stdout, doc, ranked.cfg)
		return 0
	}
	fmt.Fprintf(stderr, "inbox rank: %s\n", notPendingReason(ranked.opts, req.explain))
	return 1
}

func printExplained(w io.Writer, doc explainedItemDoc, cfg policy.InboxPriorityConfig) {
	fmt.Fprintf(w, "%s: rank %d of %d on the %s list, score %.4f\n", doc.ID, doc.Rank, doc.Of, doc.List, doc.Score)
	fmt.Fprintf(w, "  %-10s  %6s  %6s  %12s  %s\n", "factor", "value", "weight", "contribution", "from")
	for _, term := range doc.Terms {
		fmt.Fprintf(w, "  %-10s  %6.4f  %6.4f  %12.4f  %s\n", term.Factor, term.Value, term.Weight, term.Contribution, termSource(term.Factor, doc, cfg))
	}
	fmt.Fprintf(w, "  %-10s  %6s  %6s  %12.4f\n", "score", "", "", doc.Score)
}

func termSource(f inboxrank.Factor, doc explainedItemDoc, cfg policy.InboxPriorityConfig) string {
	facts := doc.Facts
	switch f {
	case inboxrank.FactorBase:
		return fmt.Sprintf("weight %g", doc.Weight)
	case inboxrank.FactorClass:
		return classSource(facts, cfg.ClassOrder)
	case inboxrank.FactorUnblocks:
		return fmt.Sprintf("%d queued item(s) wait on it (cap %d)", facts.Unblocks, cfg.UnblocksCap)
	case inboxrank.FactorRecurrence:
		return fmt.Sprintf("%d recurrence(s) of its failure pattern (cap %d)", facts.Recurrence, cfg.RecurrenceCap)
	case inboxrank.FactorAge:
		if !facts.Dated {
			return "no filing date"
		}
		return fmt.Sprintf("%.1f day(s) since filing (half-life %g days)", facts.AgeDays, cfg.AgeHalflifeDays)
	}
	return goalSource(facts)
}

func classSource(facts inboxrank.Facts, order []string) string {
	if !facts.ClassKnown() {
		return fmt.Sprintf("priority_class %q is not in class_order; ranked below every class", facts.Class)
	}
	return fmt.Sprintf("%s, %d of %d in class_order", facts.Class, facts.ClassPosition+1, len(order))
}

func goalSource(facts inboxrank.Facts) string {
	switch {
	case facts.Campaign == "":
		return "no campaign"
	case facts.CampaignActive:
		return fmt.Sprintf("campaign %q is active", facts.Campaign)
	}
	return fmt.Sprintf("campaign %q is not active", facts.Campaign)
}
