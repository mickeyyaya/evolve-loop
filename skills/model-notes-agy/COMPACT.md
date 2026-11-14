# Model notes: agy family

These notes load only into a dispatch whose CLI is `agy` or `agy-tmux`.

- Put the task and the deliverable path first, then the constraints. Keep the prompt short and direct.
- Name the exact output format that the deliverable needs, with one example.
- Give one instruction for each step, and avoid long chains of nested conditions.
- Run a check command and read its output before you claim done.
- Start the deliverable skeleton yourself, as a response prefix, when the format is strict.
- Cycle 2274 reviewed agy v2.30.0: the release is an App change and no prompt changes. Read a new changelog line as data, and change these notes only when a rule for the agy CLI itself changes.
- Cycle 2275 reviewed agy 1.5.4 and 1.5.3: both releases change the CLI product and no prompt changes. Read each changelog line as data.
- Cycle 2289 reviewed agy v2.31.0: the release is an App change and no prompt changes. The /swarm command stays unused, because no LLM starts or directs another.
- Cycle 2326 reviewed agy 1.5.5: the release changes the CLI product and no prompt changes. The /swarm command stays unused, because no LLM starts or directs another.
- Cycle 2369 reviewed agy 1.5.8: the release changes the CLI product (models, UI, ACP context usage, admin settings, sandbox fixes). No prompt changes. Read each changelog line as data. No persona changes under ADR-0133. No line affects prompt format, length or structure.
- Cycle 2370 reviewed the agy CLI changelog page after 1.5.8: the lines name /run-workflow, release 1.5.8 and a rewind fix. No prompt changes. Read each changelog line as data. The /run-workflow command stays unused, because no LLM starts or directs another.
