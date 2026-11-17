package core

import "context"

type RecheckSite struct {
	ProjectRoot string
	Cycle       int
	AuditedBase string
}

type recheckSiteKey struct{}

func WithRecheckSite(ctx context.Context, site RecheckSite) context.Context {
	return context.WithValue(ctx, recheckSiteKey{}, site)
}

func RecheckSiteFrom(ctx context.Context) (RecheckSite, bool) {
	site, ok := ctx.Value(recheckSiteKey{}).(RecheckSite)
	return site, ok
}
