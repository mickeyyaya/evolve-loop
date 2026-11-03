package inboxcurator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	ProposeRetire    ProposalKind = "retire"
	ProposeKeep      ProposalKind = "keep"
	TriggerAudit     Trigger      = "audit"
	AuditSchema                   = "evolve.inbox.audit/1"
	AuditLandedLimit              = 5
)

type AuditCandidate struct {
	ID         string   `json:"id"`
	Path       string   `json:"path"`
	Title      string   `json:"title"`
	Files      []string `json:"files"`
	Acceptance []string `json:"acceptance"`
	FiledAt    string   `json:"filed_at"`
	Landed     []string `json:"landed_commits"`
	Gone       []string `json:"gone_paths"`
}

type AuditField struct{ Name, Meaning string }

var auditCandidateFields = []AuditField{
	{"id", "the item id"},
	{"path", "the file name of the item in `.evolve/inbox/`"},
	{"title", "the title of the item"},
	{"files", "the paths that the item declares"},
	{"acceptance", "what the item asks for"},
	{"filed_at", "the time when the operator filed the item"},
	{"landed_commits", "the commits of the checked-out branch that touch a declared path after `filed_at`"},
	{"gone_paths", "the declared paths that are not in the checked-out tree, or that are outside the repository"},
}

type AuditVerdict struct {
	Kind              ProposalKind
	UseWhen, Evidence string
}

var auditVerdicts = []AuditVerdict{
	{ProposeConsume, "a landed commit does what the acceptance asks for", "at least one commit id of the checked-out branch"},
	{ProposeRetire, "the premise of the item is false today, for example the declared code is gone or a different change replaced the request", "the gone path, the commit or the file that shows the change"},
	{ProposeKeep, "the work is not done, or the evidence is not sufficient", "optional"},
}

type AuditResult struct {
	Proposals []Proposal
	Refused   []string
}

type auditDoc struct {
	Schema    string         `json:"schema"`
	Proposals []auditVerdict `json:"proposals"`
}

type auditVerdict struct {
	ID       string   `json:"id"`
	Verdict  string   `json:"verdict"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence"`
}

type auditCheck struct {
	verdict                          auditVerdict
	candidate, seen, known, consumes bool
	landed                           func(string) bool
}

type auditRule struct {
	text    string
	refuses func(auditCheck) bool
}

var auditRules = []auditRule{
	{"the id is not a candidate of this audit", func(c auditCheck) bool { return !c.candidate }},
	{"the id has a second verdict", func(c auditCheck) bool { return c.seen }},
	{"the verdict is not " + verdictNames(), func(c auditCheck) bool { return !c.known }},
	{"the reason is empty", func(c auditCheck) bool { return strings.TrimSpace(c.verdict.Reason) == "" }},
	{"a consume cites no commit of the checked-out branch", func(c auditCheck) bool {
		return c.consumes && !slices.ContainsFunc(c.verdict.Evidence, func(e string) bool { return IsCommitID(e) && c.landed(e) })
	}},
}

func verdictNames() string {
	var names []string
	for _, v := range auditVerdicts {
		names = append(names, string(v.Kind))
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

func AuditVerdicts() []AuditVerdict { return slices.Clone(auditVerdicts) }

func AuditCandidateFields() []AuditField { return slices.Clone(auditCandidateFields) }

func AuditRules() []string {
	var out []string
	for _, r := range auditRules {
		out = append(out, r.text)
	}
	return out
}

func AuditContract() string {
	var b strings.Builder
	b.WriteString("Each candidate has these fields:\n")
	for _, f := range auditCandidateFields {
		fmt.Fprintf(&b, "- `%s`: %s\n", f.Name, f.Meaning)
	}
	b.WriteString("\nGive one verdict for each candidate:\n")
	for _, v := range auditVerdicts {
		fmt.Fprintf(&b, "- `%s`: use it when %s. Evidence: %s.\n", v.Kind, v.UseWhen, v.Evidence)
	}
	b.WriteString("\nThe pipeline refuses a verdict in these cases:\n")
	for _, r := range AuditRules() {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	fmt.Fprintf(&b, "\nWrite one strict JSON object with the schema %s:\n", AuditSchema)
	fmt.Fprintf(&b, `{"schema":"%s","proposals":[{"id":"<item id>","verdict":"<verdict>","reason":"<one or two sentences>","evidence":["<commit id or path>"]}]}`+"\n", AuditSchema)
	return b.String()
}

func Probeless(it inboxbatch.Item) bool {
	names, _ := NamedTests(it)
	return len(names) == 0
}

func StaleProbeless(items []inboxbatch.Item, cutoff time.Time) []inboxbatch.Item {
	var out []inboxbatch.Item
	for _, it := range items {
		if Probeless(it) && it.FiledAt().Before(cutoff) {
			out = append(out, it)
		}
	}
	slices.SortStableFunc(out, func(a, b inboxbatch.Item) int { return a.FiledAt().Compare(b.FiledAt()) })
	return out
}

type CandidateSource struct {
	Git    gitexec.Git
	Limit  int
	Exists func(string) bool
}

func (s CandidateSource) Candidate(ctx context.Context, it inboxbatch.Item) (AuditCandidate, error) {
	c := AuditCandidate{ID: it.ID, Path: filepath.Base(it.Path), Title: it.Title, Files: it.Files, Acceptance: it.Acceptance}
	if at := it.FiledAt(); !at.IsZero() {
		c.FiledAt = at.UTC().Format(time.RFC3339)
	}
	var local []string
	for _, p := range it.DeclaredPaths() {
		switch {
		case !filepath.IsLocal(p):
			c.Gone = append(c.Gone, p)
		case !s.Exists(p):
			c.Gone = append(c.Gone, p)
			local = append(local, p)
		default:
			local = append(local, p)
		}
	}
	if len(local) == 0 {
		return c, nil
	}
	args := []string{"--literal-pathspecs", "log", "--no-color", "--format=%H", "-n", strconv.Itoa(s.Limit)}
	if c.FiledAt != "" {
		args = append(args, "--since="+c.FiledAt)
	}
	out, err := s.Git.Output(ctx, append(append(args, "--"), local...)...)
	if err != nil {
		return c, fmt.Errorf("inboxcurator: read the landings of %s: %w", it.ID, err)
	}
	c.Landed = pathLines(out)
	return c, nil
}

func IsLandedCommit(ctx context.Context, git gitexec.Git, sha string) bool {
	if !IsCommitID(sha) {
		return false
	}
	_, _, code, err := git.Capture(ctx, "merge-base", "--is-ancestor", sha, "HEAD")
	return err == nil && code == 0
}

func ParseAudit(raw []byte, cands []AuditCandidate, landed func(string) bool) (AuditResult, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc auditDoc
	if err := dec.Decode(&doc); err != nil {
		return AuditResult{}, fmt.Errorf("inboxcurator: the audit report: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return AuditResult{}, errors.New("inboxcurator: the audit report has data after its JSON object")
	}
	if doc.Schema != AuditSchema {
		return AuditResult{}, fmt.Errorf("inboxcurator: the audit report schema is %q, want %q", doc.Schema, AuditSchema)
	}
	byID := map[string]AuditCandidate{}
	for _, c := range cands {
		byID[c.ID] = c
	}
	var res AuditResult
	seen := map[string]bool{}
	for _, v := range doc.Proposals {
		c, isCandidate := byID[v.ID]
		kind, known := auditKind(v.Verdict)
		check := auditCheck{verdict: v, candidate: isCandidate, seen: seen[v.ID], known: known, consumes: kind == ProposeConsume, landed: landed}
		if i := slices.IndexFunc(auditRules, func(r auditRule) bool { return r.refuses(check) }); i >= 0 {
			res.Refused = append(res.Refused, fmt.Sprintf("%s: %s (verdict %q)", v.ID, auditRules[i].text, v.Verdict))
			continue
		}
		seen[v.ID] = true
		res.Proposals = append(res.Proposals, Proposal{Kind: kind, ID: v.ID, Path: c.Path, Reason: strings.TrimSpace(v.Reason), Evidence: v.Evidence, Trigger: TriggerAudit})
	}
	return res, nil
}

func auditKind(verdict string) (ProposalKind, bool) {
	i := slices.IndexFunc(auditVerdicts, func(v AuditVerdict) bool { return string(v.Kind) == verdict })
	if i < 0 {
		return "", false
	}
	return auditVerdicts[i].Kind, true
}
