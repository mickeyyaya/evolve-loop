// Package cliupdate updates each subscribed CLI family at a loop boundary and smoke-tests a changed version.
// See docs/architecture/packages/internal-cliupdate.md.
package cliupdate

import (
	"context"
	"strings"
)

type Status string

const (
	StatusUnchanged    Status = "unchanged"
	StatusUpdated      Status = "updated"
	StatusSelfUpdated  Status = "self-updated"
	StatusUpdateFailed Status = "update-failed"
	StatusSmokeFailed  Status = "smoke-failed"
	StatusNoUpdater    Status = "no-updater"
	StatusSkipped      Status = "skipped"
	StatusPlanned      Status = "planned"
)

const noUpdaterDetail = "the family manifest declares no update_argv"

type Family struct {
	Name             string
	UpdateArgv       []string
	AutoUpdateOffEnv string
}

type Seams struct {
	Eligible func(family string) (bool, string)
	Probe    func(ctx context.Context, family string) error
	Version  func(bin string) (string, error)
	Run      func(ctx context.Context, f Family) error
	Smoke    func(ctx context.Context, family string) error
}

type Result struct {
	Family          string   `json:"family"`
	Status          Status   `json:"status"`
	SelfUpdatedFrom string   `json:"self_updated_from,omitempty"`
	OldVersion      string   `json:"old_version,omitempty"`
	NewVersion      string   `json:"new_version,omitempty"`
	Argv            []string `json:"argv,omitempty"`
	Detail          string   `json:"detail,omitempty"`
}

type Report struct {
	DryRun  bool     `json:"dry_run"`
	Results []Result `json:"results"`
}

func Plan(families []Family) Report {
	rep := Report{DryRun: true, Results: make([]Result, 0, len(families))}
	for _, f := range families {
		res := Result{Family: f.Name, Status: StatusNoUpdater, Detail: noUpdaterDetail}
		if len(f.UpdateArgv) > 0 {
			res = Result{Family: f.Name, Status: StatusPlanned, Argv: f.UpdateArgv,
				Detail: "would check the subscription, smoke-test a version the last boundary did not record, run `" + strings.Join(f.UpdateArgv, " ") + "` and smoke-test a changed version"}
		}
		rep.Results = append(rep.Results, res)
	}
	return rep
}

func Update(ctx context.Context, families []Family, s Seams, history []Record) Report {
	rep := Report{Results: make([]Result, 0, len(families))}
	for _, f := range families {
		rep.Results = append(rep.Results, updateOne(ctx, f, s, lastSeen(history, f.Name)))
	}
	return rep
}

func updateOne(ctx context.Context, f Family, s Seams, last string) Result {
	res := Result{Family: f.Name, Argv: f.UpdateArgv}
	if len(f.UpdateArgv) == 0 {
		return res.with(StatusNoUpdater, noUpdaterDetail)
	}
	if ctx.Err() != nil {
		return res.interrupted(ctx)
	}
	if ok, why := s.Eligible(f.Name); !ok {
		return res.with(StatusSkipped, why)
	}
	old, err := s.Version(f.Name)
	if err != nil {
		return smokeUnreadableStart(ctx, res, s, err)
	}
	res.OldVersion = old
	res, ready := checkStart(ctx, res, s, last)
	if !ready {
		return res
	}
	return runUpdater(ctx, res, f, s)
}

func smokeUnreadableStart(ctx context.Context, res Result, s Seams, versionErr error) Result {
	notes := []string{"version before the update: " + versionErr.Error()}
	if err := s.Smoke(ctx, res.Family); err != nil {
		return res.failedSmoke(ctx, notes, err)
	}
	return res.with(StatusUpdateFailed, strings.Join(append(notes, "smoke OK; the updater did not run"), "; "))
}

func checkStart(ctx context.Context, res Result, s Seams, last string) (Result, bool) {
	if last != "" && last != res.OldVersion {
		res.SelfUpdatedFrom = last
		if err := s.Smoke(ctx, res.Family); err != nil {
			return res.failedSmoke(ctx, res.selfUpdateNote(), err), false
		}
		return res, true
	}
	if err := s.Probe(ctx, res.Family); err != nil {
		if ctx.Err() != nil {
			return res.interrupted(ctx), false
		}
		return res.with(StatusSkipped, "doctor live did not answer, so the family counts as unsubscribed: "+err.Error()), false
	}
	return res, true
}

func runUpdater(ctx context.Context, res Result, f Family, s Seams) Result {
	runErr := s.Run(ctx, f)
	if ctx.Err() != nil {
		return res.interrupted(ctx)
	}
	newVersion, verErr := s.Version(f.Name)
	res.NewVersion = newVersion
	if runErr == nil && verErr == nil && newVersion == res.OldVersion {
		return res.unchanged()
	}
	return smoked(ctx, res, s, failures(runErr, verErr))
}

func failures(runErr, verErr error) []string {
	var out []string
	if runErr != nil {
		out = append(out, "updater: "+runErr.Error())
	}
	if verErr != nil {
		out = append(out, "version after the update: "+verErr.Error())
	}
	return out
}

func smoked(ctx context.Context, res Result, s Seams, problems []string) Result {
	notes := append(res.selfUpdateNote(), problems...)
	if err := s.Smoke(ctx, res.Family); err != nil {
		return res.failedSmoke(ctx, notes, err)
	}
	switch {
	case len(problems) > 0:
		return res.with(StatusUpdateFailed, strings.Join(append(notes, "smoke OK"), "; "))
	case len(notes) > 0:
		return res.with(StatusUpdated, strings.Join(append(notes, "smoke OK"), "; "))
	}
	return res.with(StatusUpdated, "")
}

func (r Result) failedSmoke(ctx context.Context, notes []string, err error) Result {
	if ctx.Err() != nil {
		return r.interrupted(ctx)
	}
	return r.with(StatusSmokeFailed, strings.Join(append(notes, "smoke: "+err.Error()), "; "))
}

func (r Result) interrupted(ctx context.Context) Result {
	return r.with(StatusSkipped, "interrupted: "+ctx.Err().Error())
}

func (r Result) unchanged() Result {
	if r.SelfUpdatedFrom != "" {
		return r.with(StatusSelfUpdated, "outside a boundary; smoke OK")
	}
	return r.with(StatusUnchanged, "")
}

func (r Result) selfUpdateNote() []string {
	if r.SelfUpdatedFrom == "" {
		return nil
	}
	return []string{"self-updated to " + r.OldVersion + " outside a boundary"}
}

func (r Result) with(status Status, detail string) Result {
	r.Status, r.Detail = status, detail
	return r
}

func (r Result) Line() string {
	line := r.Family + " " + string(r.Status)
	if v := r.Versions(); v != "" {
		line += " " + v
	}
	if r.Detail != "" {
		line += ": " + r.Detail
	}
	return line
}

func (r Result) Versions() string {
	from, to := r.OldVersion, r.NewVersion
	if r.SelfUpdatedFrom != "" {
		from = r.SelfUpdatedFrom
	}
	if to == "" {
		to = r.OldVersion
	}
	switch {
	case from == "":
		return ""
	case to == from:
		return from
	default:
		return from + " → " + to
	}
}

func (r Report) Failed() []Result {
	return r.filter(func(res Result) bool {
		return res.Status == StatusUpdateFailed || res.Status == StatusSmokeFailed
	})
}

func (r Report) SmokeFailed() []Result {
	return r.filter(func(res Result) bool { return res.Status == StatusSmokeFailed })
}

func (r Report) filter(keep func(Result) bool) []Result {
	var out []Result
	for _, res := range r.Results {
		if keep(res) {
			out = append(out, res)
		}
	}
	return out
}
