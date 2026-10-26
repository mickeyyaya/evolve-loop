---
id: no-feature-flags
statement: "Add no feature flag. Select each behavior through configuration data and a design pattern."
layer: L0
order: 15
audience: [all]
enforced_by: []
source: "memory feedback_no_feature_flags_use_design_patterns and feedback_no_feature_flag_sprawl"
status: active
added: 2026-10-26
evidence: "the operator rule of 2026-06-19 (memory feedback_no_feature_flags_use_design_patterns)"
---
Why: A flag forks the code into paths that no test covers together. Data and a strategy keep one path.
