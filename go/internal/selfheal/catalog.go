package selfheal

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	PreCatalog         = "catalog"
	PreTierLimit       = "tier-limit"
	PreItemKnown       = "item-known"
	PreItemOfIncident  = "item-of-incident"
	PreFamilyKnown     = "family-known"
	PreVariationNew    = "variation-new"
	PreMainRed         = "main-red"
	PreLandingInWindow = "landing-in-window"
	PreFilesExist      = "files-exist"
	PreConverge        = "converge"
	PreBreaker         = "breaker"
	PreBudget          = "budget"
	PreDiagnosis       = "diagnosis"
)

type Landing struct {
	SHA string
	At  time.Time
}

type Inventory struct {
	Items      map[string]bool
	Families   []string
	Landings   []Landing
	MainRed    bool
	FileExists func(path string) bool
}

type check struct {
	inc     Incident
	params  phasecontract.DiagnosisParams
	inv     Inventory
	history History
}

type precondition struct {
	name   string
	refuse func(check) string
}

type catalogAction struct {
	kind   string
	tiers  []string
	repair bool
	pre    []precondition
}

var (
	loopTiers = []string{TierPipeline, TierItem}
	allTiers  = []string{TierKernel, TierPipeline, TierItem}

	itemKnown       = precondition{PreItemKnown, refuseUnknownItem}
	itemOfIncident  = precondition{PreItemOfIncident, refuseItemOfAnotherIncident}
	familyKnown     = precondition{PreFamilyKnown, refuseUnknownFamily}
	variationNew    = precondition{PreVariationNew, refuseTriedVariation}
	mainRed         = precondition{PreMainRed, refuseGreenMain}
	landingInWindow = precondition{PreLandingInWindow, refuseUnknownLanding}
	filesExist      = precondition{PreFilesExist, refuseMissingFile}
)

var catalog = []catalogAction{
	{phasecontract.ActionRetryWithVariation, loopTiers, true, []precondition{itemKnown, variationNew}},
	{phasecontract.ActionRerouteFamily, loopTiers, true, []precondition{itemKnown, familyKnown}},
	{phasecontract.ActionQuarantineItem, allTiers, false, []precondition{itemKnown, itemOfIncident}},
	{phasecontract.ActionRevertLastLanding, loopTiers, true, []precondition{mainRed, landingInWindow}},
	{phasecontract.ActionOpenRepairItem, allTiers, true, []precondition{filesExist}},
	{phasecontract.ActionHaltAndEscalate, allTiers, false, nil},
}

func CatalogActions() []string {
	out := make([]string, 0, len(catalog))
	for _, a := range catalog {
		out = append(out, a.kind)
	}
	return out
}

func catalogEntry(kind string) (catalogAction, bool) {
	i := slices.IndexFunc(catalog, func(a catalogAction) bool { return a.kind == kind })
	if i < 0 {
		return catalogAction{}, false
	}
	return catalog[i], true
}

func (a catalogAction) firstRefusal(c check) (string, string) {
	if !slices.Contains(a.tiers, c.inc.Tier) {
		return PreTierLimit, fmt.Sprintf("%s runs only at the tiers %v, and the incident is %s", a.kind, a.tiers, c.inc.Tier)
	}
	for _, p := range a.pre {
		if reason := p.refuse(c); reason != "" {
			return p.name, reason
		}
	}
	return "", ""
}

func refuseUnknownItem(c check) string {
	if c.inv.Items[c.params.Item] {
		return ""
	}
	return fmt.Sprintf("the item %q is not a pending inbox item", c.params.Item)
}

func refuseItemOfAnotherIncident(c check) string {
	if c.params.Item == c.inc.Subject {
		return ""
	}
	return fmt.Sprintf("the item %q is not the item of the incident, %q", c.params.Item, c.inc.Subject)
}

func refuseUnknownFamily(c check) string {
	if slices.Contains(c.inv.Families, c.params.Family) {
		return ""
	}
	return fmt.Sprintf("the family %q is not an installed CLI family %v", c.params.Family, c.inv.Families)
}

func refuseTriedVariation(c check) string {
	for _, d := range c.history.Decisions {
		if d.IncidentID == c.inc.ID && d.Verdict == VerdictAccepted && d.Action != nil &&
			d.Action.Kind == phasecontract.ActionRetryWithVariation && d.Action.Params.Variation == c.params.Variation {
			return fmt.Sprintf("an earlier retry of the incident used the variation %q", c.params.Variation)
		}
	}
	return ""
}

func refuseGreenMain(c check) string {
	if c.inv.MainRed {
		return ""
	}
	return "main is not red: no open main-red incident"
}

func refuseUnknownLanding(c check) string {
	for _, l := range c.inv.Landings {
		if strings.HasPrefix(l.SHA, c.params.SHA) {
			return ""
		}
	}
	return fmt.Sprintf("the commit %q is not a landing of the loop in the window", c.params.SHA)
}

func refuseMissingFile(c check) string {
	for _, f := range c.params.Files {
		if !c.inv.FileExists(f) {
			return fmt.Sprintf("the file %q is not in the repository", f)
		}
	}
	return ""
}
