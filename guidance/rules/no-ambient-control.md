---
id: no-ambient-control
statement: "Read no control value from the environment. An operator setting lives in policy.json, and a bypass is a CLI parameter."
layer: L0
order: 14
audience: [all]
enforced_by: []
source: "ADR-0156 I-16; memory feedback_cli_params_over_env_flags"
status: active
added: 2026-10-26
evidence: "ADR-0156 I-16 of 2026-10-25"
---
Why: A setting in policy.json is reviewed and decoded strictly. An environment value is invisible.
