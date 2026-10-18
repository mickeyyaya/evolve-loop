// Package outbox ingests the drafts that one dispatch writes into its own outbox, sends them through the broker and writes the answer into the mailbox of the sender.
// The path of a draft names the dispatch; the draft never does. A dispatch id is unique when folded to lower case, because a case-folding host maps two keys that differ only in case to one directory. A wait arms a kernel watch and never polls.
package outbox
