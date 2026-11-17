# Option 3: a conflict rule that reads the edited symbols

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

**What changes:** admission parses the symbols that each item edits. Two items conflict only on a shared symbol.

**For whom:** all lanes.

**Causal chain:** a symbol-level rule is the most precise rule, so more items run together.

**Expected effect:** not quantifiable. An item names files, not symbols, before its build (A2), so the symbols are unknown at admission time.

**Cost and time to effect:** a parser, a guess or an LLM inside admission. This breaks A1.

**Top risks:**

1. A wrong guess admits two lanes that conflict. Detect: integration reds.
2. Admission stops being deterministic (A1). Detect: the property `TestAdmit_TwoAdmittedTodosNeverConflict` fails.
