---
score_cap:
  - criterion: "The convention states the host-credential rule with every key of the one credential set"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2410_00[123]' ./acs/cycle2410"
  - criterion: "Sections 8.5 and 9 of llm-isolation.md state the built tool view, the secret list and the as-built status"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2410_00[4567]' ./acs/cycle2410"
  - criterion: "The I-5 row of section 16.2 guides the secret-read triage"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2410_008' ./acs/cycle2410"
  - criterion: "The spec and the package notes link each other and pass ste-lint --strict with resolving links"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2410_01[0123]' ./acs/cycle2410"
---

# Eval: AD1-9 update llm-isolation.md and the hermetic test rule

> Pins the document half of the AD1 credential isolation. The code of AD1-1 to AD1-6 landed, and
> the spec still called section 9.2 a design. Cycle 2410 brings sections 8.5, 9 and 16.2 and the
> hermetic-driver-tests convention in line with `ipcenv.CredentialIsolation`, `homeSecrets` and
> the manifest that sets `prStatusFooterEnabled`. The facts come from the code, not from the plan:
> only `claude-tmux.json` sets the footer key, although the plan names three manifests.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| convention-rule | the convention names the helper and each key | 6/10 | `go test -tags acs -run 'TestC2410_00[123]' ./acs/cycle2410` |
| spec-built-state | sections 8.5 and 9 match the code | 6/10 | `go test -tags acs -run 'TestC2410_00[4567]' ./acs/cycle2410` |
| runbook-row | the I-5 row guides the triage | 7/10 | `go test -tags acs -run 'TestC2410_008' ./acs/cycle2410` |
| lint-and-links | ste-lint, links, package-note cross links | 7/10 | `go test -tags acs -run 'TestC2410_01[0123]' ./acs/cycle2410` |
