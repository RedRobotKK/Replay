// Command replay reads Claude Code and Codex transcripts from disk and finds
// the turn on which a prompt cache expired, and what that turn cost.
//
// Every figure it prints carries a tier, the population it was measured on
// and the date it was read; where a figure cannot be measured, it prints a
// refusal in the place the number would have gone. The binary makes no
// network request except from four commands that say so when typed:
// rules --check-prices, probe --execute, upgrade and rules --update.
// The documentation is at https://replay.doctor.
package main
