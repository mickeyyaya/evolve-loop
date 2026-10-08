# Simplified Technical English (ASD-STE100): the house rules

All documentation in this repository uses Simplified Technical English. The source is ASD-STE100, Issue 9 (January 2025). The operator gave this rule on 2026-10-07: "improve the readability by strictly follow ASD-STE100 policy and rewrite all docs wording by strictly follow ASD-STE100 format".

This document gives the house rules. The rules apply to every author: people, the console and every loop phase. They apply to this text:

- every file in `docs/`, and `README.md`, `CLAUDE.md`, `AGENTS.md` and `CHANGELOG.md`;
- log and error text in Go code;
- generated text: generator templates, phase reports, inbox items, dossiers, release notes, commit messages and pull request bodies;
- every other document that an author writes.

Machine-read parts keep their exact form: signal codes, `key=value` fields, parser sentinels and the headings that a grammar requires. The rewrite plan is [ste100-docs-rewrite-2026-10.md](../plans/ste100-docs-rewrite-2026-10.md).

ASD-STE100 is a copyrighted specification. This document does not copy its dictionary. It gives the writing rules in short form and a house list of words to replace. For the full approved dictionary, use the specification from [asd-ste100.org](https://www.asd-ste100.org/).

## Why

- A short sentence with one meaning is easy to read and easy to translate.
- An agent reads these documents as instructions. One word with one meaning gives one interpretation.
- A deterministic lint can check the measurable rules. A reviewer then checks only the meaning.

## The rules

### Words

1. Use approved words, with their approved meaning and part of speech only.
2. Use one word for one meaning. Do not use synonyms for variety.
3. You can use technical names. Put a code name in backticks. A technical name is one of these:
   - a code identifier, a file path, a command or a flag;
   - a signal code, a config key or an ADR number;
   - a product name;
   - a term that this repository defines.
4. You can use technical verbs. A technical verb is part of a technical name or of a command. Examples: `evolve ship`, "commit", "push", "merge" and "rebase".
5. Do not make a noun cluster of more than three words. Break a longer cluster with a preposition: write "the rule for the retry budget", not "the retry budget rule table".

### Verbs and tenses

1. Use only these verb forms:
   - the imperative and the infinitive;
   - the simple present, the simple past and the simple future;
   - the past participle as an adjective.
2. Do not use the "-ing" form, except in a technical name (for example, `--force-fresh` or a quoted error message).
3. Use the active voice. In a description, you can use the passive voice only if the agent is not known.
4. Do not use "should", "may", "might", "could" or "would". Write "must" for a requirement and "can" for a possibility.

### Sentences

1. An instruction has 20 words or fewer.
2. A description or an explanation has 25 words or fewer.
3. Write one instruction in one sentence. Write two actions in one sentence only when you do them at the same time.
4. Write one topic in one sentence.
5. Do not leave out words to make a sentence shorter. Keep "the", "a" and "that".
6. Start a condition with "If" or "When", and put the condition first: "If the push fails, the ship retries."
7. Code names, paths, backtick spans and quotations count as one word each.
8. A numbered item has 20 words or fewer. A numbered item is a step, or a rule or a decision that other text cites by its number.

### Paragraphs and structure

1. A paragraph has six sentences or fewer, and one topic only.
2. Use a vertical list for a sequence of steps or for three or more items.
3. Number the steps of a procedure. Give each step one instruction.
4. Put a warning or a caution before the step that it applies to. Start it with a command or a condition.
5. Use a table for data that has two or more attributes.
6. A heading is short and tells the topic. A heading is not a sentence.

### Technical terms that keep the -ing form

These terms are technical names in this repository. Use them as nouns, with the meaning that the repository gives them. To add a term, add a row.

| Term | Meaning in this repository |
|---|---|
| routing | the choice of CLI, model and tier for a dispatch |
| binding | the record that ties a verdict or a seal to a tree or a digest (for example, the audit binding) |
| audit-binding | the binding of a ship to its audit |
| pinning | a fixed choice of CLI or model in the policy |
| wiring | the connection of a component to its production caller |
| provisioning | the creation of a worktree or a workspace for a phase |
| reasoning | the class of finding that a stronger judgment can fix |
| finding | one defect or observation that a review reports |
| landing | the merge of a change into `main`, and the steps that do it |
| billing | the cost of an account or a subscription |
| plumbing | low-level code that moves data between components |
| anti-gaming | the controls that stop an agent from passing a gate without the work |
| error handling | the code that detects and reports errors |
| mutation testing | a test of the tests: change the code and make sure that a test fails |

### Words that are also code names

A house-list word that is also a code name or a term with a definition here stays as it is. Put a code name in backticks: for example, "`attempt` 2/2" in a log line, or the `--resume` flag. In other prose, use the approved word.

## What the rules do not change

- **Quotations.** Text in quotation marks that records the operator's words, an error message or tool output stays exactly as it was said. The lint counts a quotation as one word and does not check its words.
- **Code.** Fenced code blocks, inline code spans and URLs stay as they are.
- **Meaning.** A rewrite keeps every fact, number, path, link and decision. When a rule and the meaning conflict, keep the meaning and split the sentence.
- **Generated documents.** A generator writes some files (for example, `docs/architecture/signal-codes.md`) and some rows (for example, the README version history that `versionbump` writes). Change the generator's templates, not the output.
- **Headings that something reads.** A heading that a test, a persona, a predicate or a link reads keeps its text. Before you change a heading, search the repository for its text and its anchor. If the lint finds a house-list word in such a heading, the batch lists the finding as an accepted exception.

## House list of words to replace

The lint reads this table, and it does not check the words of this table in this document. The left column is a word or phrase that this repository often uses and that STE does not approve. The right column gives the approved alternatives. The lint matches the left column as whole words, without regard to case, outside code, links and quotations.

| Do not write | Write |
|---|---|
| utilize | use |
| utilise | use |
| leverage | use |
| ensure | make sure |
| in order to | to |
| prior to | before |
| subsequent to | after |
| subsequently | then, after that |
| via | through, with, by |
| approximately | about |
| additional | more, other |
| numerous | many |
| sufficient | enough |
| commence | start |
| terminate | stop |
| facilitate | help |
| assist | help |
| demonstrate | show |
| indicate | show |
| obtain | get |
| attempt | try |
| modify | change |
| initial | first |
| regarding | about |
| with regard to | about |
| whether | if |
| should | must, can |
| might | can |
| could | can |
| would | will, can |
| shall | must |
| e.g. | for example |
| i.e. | that is |
| etc. | (list all the items) |
| vs. | against, compared with |
| et al. | and others |
| incl. | with |

To add a word, add a row and a test in the lint package. Remove a word only when the specification approves it.

## The lint

`evolve docs ste-lint` checks the measurable rules:
- sentence length (20 words in a numbered item, 25 words in other text);
- paragraph length (6 sentences);
- the house list of words.

The lint reads the text in this way:
- A code span, a path, a URL and a quotation count as one word each. The lint does not check the words of a quotation.
- Link text counts for sentence length. The lint does not check link text against the house list, because link text often names another document.
- A heading has no length rule. The lint checks a heading against the house list.
- A blockquote is prose, and the lint checks it. If the text of a blockquote starts with a quotation mark, the blockquote is a quoted passage, and the lint does not check it.
- The lint does not read fenced code, HTML comments, HTML blocks, YAML frontmatter or generated text.
- Some tokens are technical. The lint does not check them against the house list. They are a `key=value` token, a URL, a bracket tag (for example, `[runner]`) and a word with an underscore.

The lint reports findings as a WARN. It does not block a commit or a ship. The build handoff floor runs it on the changed documents and prints the result beside the documentation floor's line. The lint cannot check the meaning, the voice or the tense of a sentence: a reviewer checks those.

## References

- ASD-STE100 Simplified Technical English, Issue 9, January 2025: [asd-ste100.org](https://www.asd-ste100.org/)
- [Simplified Technical English](https://en.wikipedia.org/wiki/Simplified_Technical_English) (a summary of the rule categories)
- [markdown-structure.md](markdown-structure.md): the house structure for Markdown files
- [code-comments.md](code-comments.md): code carries no comments
