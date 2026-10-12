package practicewatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const defaultTimeout = 20 * time.Second

type Status string

const (
	StatusRecorded    Status = "recorded"
	StatusWouldRecord Status = "would-record"
	StatusUnchanged   Status = "unchanged"
	StatusFiled       Status = "filed"
	StatusWouldFile   Status = "would-file"
	StatusOpen        Status = "open"
	StatusDuplicate   Status = "duplicate"
	StatusSkipped     Status = "skipped"
	StatusOff         Status = "off"
	StatusError       Status = "error"
)

type Result struct {
	ID     string `json:"id"`
	Family string `json:"family"`
	Kind   Kind   `json:"kind"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Item   string `json:"item,omitempty"`
}

type Report struct {
	DryRun  bool     `json:"dry_run"`
	Results []Result `json:"results"`
}

func (r Report) count(status Status) int {
	n := 0
	for _, res := range r.Results {
		if res.Status == status {
			n++
		}
	}
	return n
}

func (r Report) Errors() int { return r.count(StatusError) }

func (r Report) Filed() int { return r.count(StatusFiled) }

type Options struct {
	Root    string
	Sources []Source
	Timeout time.Duration
	DryRun  bool
	Client  *http.Client
	Run     sysexec.RunFunc
	Now     func() time.Time
	Inbox   inboxmover.Options
}

func (o Options) withDefaults() Options {
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.Client == nil {
		o.Client = &http.Client{}
	}
	if o.Run == nil {
		o.Run = sysexec.DefaultRunner
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

func Watch(ctx context.Context, opts Options) Report {
	opts = opts.withDefaults()
	report := Report{DryRun: opts.DryRun}
	for _, src := range opts.Sources {
		report.Results = append(report.Results, opts.watchSource(ctx, src))
	}
	return report
}

func (o Options) watchSource(ctx context.Context, src Source) Result {
	res := Result{ID: src.ID, Family: src.Family, Kind: src.Kind}
	if !src.Enabled {
		return res.with(StatusOff, "")
	}
	prev, found, err := loadSnapshot(o.Root, src.ID)
	if err != nil {
		return res.with(StatusError, err.Error())
	}
	fctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	got, err := o.fetch(fctx, src, prev.ETag)
	switch {
	case err != nil:
		return res.with(StatusError, err.Error())
	case got.skip != "":
		return res.with(StatusSkipped, got.skip)
	case got.notModified:
		return res.with(StatusUnchanged, "the server answered 304")
	}
	text := Normalize(src.Kind, got.text)
	sum := sha256.Sum256([]byte(text))
	next := snapshot{Hash: hex.EncodeToString(sum[:]), FetchedAt: o.Now().UTC().Format(time.RFC3339), ETag: got.etag, Excerpt: capText(text, excerptLimit)}
	switch {
	case !found:
		return o.record(res, src, next)
	case prev.Hash == next.Hash:
		return res.with(StatusUnchanged, "")
	}
	return o.fileChange(res, src, prev, next)
}

func (r Result) with(status Status, detail string) Result {
	r.Status, r.Detail = status, detail
	return r
}

func (o Options) record(res Result, src Source, next snapshot) Result {
	if o.DryRun {
		return res.with(StatusWouldRecord, "the first run records the snapshot and files nothing")
	}
	if err := saveSnapshot(o.Root, src.ID, next); err != nil {
		return res.with(StatusError, err.Error())
	}
	return res.with(StatusRecorded, "the first run records the snapshot and files nothing")
}

func (o Options) fileChange(res Result, src Source, prev, next snapshot) Result {
	inboxDir := inboxDirOf(o.Root, o.Inbox.InboxDir)
	openID, open, err := openItemFor(inboxDir, src.ID)
	switch {
	case err != nil:
		return res.with(StatusError, err.Error())
	case open:
		res.Item = openID
		return res.with(StatusOpen, "the earlier item "+openID+" is still open: no second item")
	case o.DryRun:
		return res.with(StatusWouldFile, "")
	}
	raw, id := practiceItem(src, next.Hash, prev, addedLines(prev.Excerpt, next.Excerpt, addedLinesLimit), o.Now())
	if idFiled(inboxDir, id) {
		return o.advance(res, src, next, "the item id "+id+" is already filed (a consumed item of the same minute): the snapshot advanced")
	}
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		return res.with(StatusError, "create the inbox "+inboxDir+": "+err.Error())
	}
	if _, err := inboxmover.File(o.Inbox, raw); err != nil {
		return res.with(StatusError, "file the inbox item "+id+": "+err.Error())
	}
	res.Item = id
	if err := saveSnapshot(o.Root, src.ID, next); err != nil {
		return res.with(StatusError, "the item "+id+" is filed, but "+err.Error())
	}
	return res.with(StatusFiled, "")
}

func (o Options) advance(res Result, src Source, next snapshot, detail string) Result {
	if err := saveSnapshot(o.Root, src.ID, next); err != nil {
		return res.with(StatusError, err.Error())
	}
	return res.with(StatusDuplicate, detail)
}
