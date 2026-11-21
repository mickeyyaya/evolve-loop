---
score_cap:
  - criterion: "Every role that a cmd/evolve call site passes is one of publisher.Roles()"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRootSignalCenter_EveryCallSiteRoleIsAPublisherRole$' ./cmd/evolve"
  - criterion: "publisher.Roles() is the closed set of the 20 spec roots"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRoles_IsTheClosedSetOfTheSpecRoots$' ./internal/events/publisher"
  - criterion: "event-channels.md lists every role that publisher.Roles() accepts"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRoles_AreTheRolesTheEventChannelsSpecLists$' ./internal/events/publisher"
  - criterion: "An integration-root emit reaches the root bus under source integration"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2596_003_' ./acs/cycle2596"
---

# Eval: signal roots publish under registered roles

> Pins the closed role set of the event publisher against the roles that the CLI call sites pass. Source incident: cycle 2586 inbox item signal-roots-under-unregistered-roles-publish-nothing. Five roots (integration, auth, queue, latency, retire) ran under roles that Config.check refused, so rootwire.Attach dropped every event of them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| call-site-derived-guard | every call-site role is a publisher role | 4/10 | `go test -run TestRootSignalCenter_EveryCallSiteRoleIsAPublisherRole ./cmd/evolve` |
| closed-set | Roles() lists the 20 roots | 5/10 | `go test -run TestRoles_IsTheClosedSetOfTheSpecRoots ./internal/events/publisher` |
| bus-reach | integration emit is on the bus | 4/10 | `go test -tags acs -run TestC2596_003_ ./acs/cycle2596` |
