package inboxbatch

import (
	"maps"
	"slices"
)

type FieldOwner string

const (
	AuthorOwned        FieldOwner = "author"
	CuratorOwned       FieldOwner = "curator"
	LoopStampOwned     FieldOwner = "loop-stamp"
	OperatorStampOwned FieldOwner = "operator-stamp"
)

type ValueShape string

const (
	TextValue    ValueShape = "text"
	NumberValue  ValueShape = "number"
	ListValue    ValueShape = "list"
	CounterValue ValueShape = "counter"
)

type FieldRole struct {
	Owner FieldOwner
	Shape ValueShape
}

func (r FieldRole) IsCounter() bool {
	return r.Shape == CounterValue
}

func (r FieldRole) IsStamp() bool {
	return r.Owner == LoopStampOwned || r.Owner == OperatorStampOwned
}

const (
	RouteField                   = "route"
	RoutedReasonField            = "routed_reason"
	RoutedCycleField             = "routed_cycle"
	RoutedAtField                = "routed_at"
	FailureCountField            = "failure_count"
	LastFailureReasonField       = "last_failure_reason"
	ContinuationField            = "continuation"
	ReleasedContinuationsField   = "released_continuations"
	ConsumedField                = "consumed"
	UnbackedField                = "unbacked"
	RetiredReasonField           = "retired_reason"
	RetiredCycleField            = "retired_cycle"
	GitSHAField                  = "git_sha"
	PremiseVerifiedAtField       = "premise_verified_at"
	PremiseVerifiedSHAField      = "premise_verified_sha"
	PremiseVerifiedEvidenceField = "premise_verified_evidence"
)

var (
	curated       = func(shape ValueShape) FieldRole { return FieldRole{Owner: CuratorOwned, Shape: shape} }
	loopStamp     = FieldRole{Owner: LoopStampOwned}
	loopCounter   = FieldRole{Owner: LoopStampOwned, Shape: CounterValue}
	operatorStamp = FieldRole{Owner: OperatorStampOwned}
)

var fieldRoles = map[string]FieldRole{
	"id":             curated(TextValue),
	"title":          curated(TextValue),
	"summary":        curated(TextValue),
	"fix":            curated(TextValue),
	"priority_class": curated(TextValue),
	"weight":         curated(NumberValue),
	"acceptance":     curated(ListValue),
	"files":          curated(ListValue),
	"deps":           curated(ListValue),
	"connects_to":    curated(ListValue),

	RouteField:                 loopStamp,
	RoutedReasonField:          loopStamp,
	RoutedCycleField:           loopStamp,
	RoutedAtField:              loopStamp,
	FailureCountField:          loopCounter,
	LastFailureReasonField:     loopStamp,
	ContinuationField:          loopStamp,
	ReleasedContinuationsField: loopStamp,
	ConsumedField:              loopStamp,
	UnbackedField:              loopStamp,
	RetiredReasonField:         loopStamp,
	RetiredCycleField:          loopStamp,
	GitSHAField:                loopStamp,

	PremiseVerifiedAtField:       operatorStamp,
	PremiseVerifiedSHAField:      operatorStamp,
	PremiseVerifiedEvidenceField: operatorStamp,
}

func RoleOf(key string) FieldRole {
	if role, listed := fieldRoles[key]; listed {
		return role
	}
	return FieldRole{Owner: AuthorOwned}
}

func FieldsOwnedBy(owner FieldOwner) []string {
	var keys []string
	for _, key := range slices.Sorted(maps.Keys(fieldRoles)) {
		if fieldRoles[key].Owner == owner {
			keys = append(keys, key)
		}
	}
	return keys
}
