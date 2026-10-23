# `replay outcome-record`: the evidence contract

Added 2026-10-22, before the code. It records the evidence an operator supplies as a run directory that `replay outcome` can score. It reads a transcript of nothing, runs no agent, calls no provider and changes nothing it reads.

## What a PASS from this recorder means, and what it does not

**A PASS from supplied inputs means only that the supplied answer is correct under the scorer's comparison conditions:** the reply's leading integer equals the count derived from a tree whose git tree id equals the pinned substrate's. It does not prove that an agent executed the task. It does not prove that the supplied answer came from that execution. It does not prove that the captured post-run tree was the tree the execution ran in.

**`--allow-modified-tree` permits recording a comparison against a tree that is not the substrate.** It does not establish who changed the tree or why, and a FAIL it leads to is not evidence that an agent caused a regression.

## Why supplied inputs and not a transcript

The audit before this design (2026-10-22) found that a Claude Code transcript records no process exit code, no commit and no tree id, only a `cwd` path and a branch name. A real transcript of the pinned prompt was not available to confirm the prompt and final-response shapes. Transcript ingestion is out of scope until it is. Every value here is therefore stated by the operator, and the sidecar says so.

## Command

```text
replay outcome-record --task <id> --reply-file <file> --exit-code <0-255>
    --usage-in <n> --usage-out <n> --arm <label> --tree <dir> --out <new-dir>
    [--allow-modified-tree]
```

Every flag but `--allow-modified-tree` is required, and none has a default. An exit code is never inferred. A nonzero exit code is recorded as given; the scorer treats it as inadmissible and reports the run `UNAVAILABLE` (`execution_failed`), not `FAIL`. Paths are cleaned first, so a trailing separator names the same directory.

## What it writes

`<out>` is created by the recorder and must not exist beforehand. It holds:

- `tree/`: a copy of `--tree`. Regular files keep their bytes and executable bit; symbolic links are recreated with the same target and never followed; a top-level `.git` is not copied.
- `provenance.json`, schema `replay.outcome.provenance.v1`: each field's source, the three tree ids, the reply's SHA-256 and the limitation above. It holds no path, host name or user name.
- `run.json`, schema `replay.outcome.run.v1`: exactly the fields the scorer's strict decoder accepts, and no verdict or truth.

`run.json` is written last, through a temporary file renamed into place. `Score` reads only `run.json` and `tree/`, so `provenance.json` cannot change a result, and a recording that stops before the rename leaves nothing `Score` can admit.

| Field | Source label |
|---|---|
| `reply`, `exit_code`, `usage`, `arm` | `operator_attested` |
| `task_id`, `manifest_sha256`, `substrate_commit` | `reconstructed_from_manifest`, after `--task` names the embedded manifest |
| `tree/` | `post_run_copy`, with `execution_tree` `not_established` |

## Refusals

Each refusal prints `replay outcome-record: refused: <code>` with a detail, exits 1, and removes everything it created. Codes are stable:

| Code | Cause |
|---|---|
| `flag_missing:<name>` | a required flag was not given |
| `unexpected_argument` | a positional argument was given |
| `task_unknown` | `--task` is not the embedded manifest's task id |
| `exit_code_invalid` | not an integer from 0 to 255 |
| `usage_invalid` | `--usage-in` or `--usage-out` is not a non-negative integer |
| `arm_invalid` | the arm is not 1 to 64 of `A-Z a-z 0-9 . _ -` |
| `reply_missing`, `reply_symlink`, `reply_not_regular` | the reply file is absent, a symbolic link, or not a regular file |
| `reply_unreadable` | the reply file exists and cannot be read |
| `reply_empty`, `reply_too_large`, `reply_not_utf8` | blank, over 1 MiB, or not valid UTF-8, which JSON would silently rewrite |
| `tree_missing`, `tree_symlink_root`, `tree_not_dir` | `--tree` is absent, a symbolic link, or not a directory |
| `tree_unreadable` | a directory in the tree cannot be listed |
| `tree_nested_repo`, `tree_unsupported_object` | a `.git` below the top or a tree whose root directory is itself named `.git`, which is a repository's store and never a working copy; or anything but a file, link or directory |
| `tree_unverifiable` | the tree id cannot be computed, for instance because a file cannot be read |
| `tree_differs_from_substrate` | the tree id is not the substrate's, and `--allow-modified-tree` was not given |
| `out_exists`, `out_parent_missing` | `--out` exists already, or its parent does not |
| `path_unresolvable` | the tree or the output's parent cannot be resolved to an absolute path |
| `out_inside_tree` | the output, once symbolic links in its parent are resolved, is inside the tree. The reverse cannot happen, since the output must not exist |
| `out_create_failed` | the output directory cannot be created |
| `tree_copy_failed` | a directory, link or file could not be copied |
| `tree_unstable_during_copy` | the source tree id before the copy, the copy's, and the source's after the copy are not all equal |
| `write_failed` | `provenance.json` or `run.json` could not be written |
| `platform_unsupported` | Windows, where `TreeID` cannot observe executable bits |

The stability check compares three tree ids. It detects a change that lands between them; it is not isolation, and a change made and reverted inside the copy can pass it.

## Test matrix

Each case is in `cmd/replay/outcome_record_test.go` unless named otherwise.

| Case | Expected |
|---|---|
| Valid input on a synthetic substrate, through `dispatch` | recorded; `replay outcome` scores it PASS |
| Each required flag omitted in turn | `flag_missing:<name>`, `--out` absent afterwards |
| Wrong `--task` | `task_unknown` |
| Exit code 1 | recorded; scored UNAVAILABLE `execution_failed`, because the scorer treats a failed execution as inadmissible, not as a wrong answer |
| Exit code 256, -1, `abc`; usage -1, `x` | `exit_code_invalid`, `usage_invalid` |
| Reply missing, a symbolic link, a directory, blank, over 1 MiB, invalid UTF-8 | the matching reply code |
| Reply `59` | scored FAIL `wrong_answer` |
| Reply `**3**` | scored FAIL `answer_not_a_number` |
| Modified tree, then the same with `--allow-modified-tree` | `tree_differs_from_substrate`; then scored FAIL `result_tree_modified` |
| `--out` exists, empty or holding a run | `out_exists`, its bytes unchanged |
| `--out` inside the tree, directly, through `..` or through a linked parent | `out_inside_tree` |
| `--tree` is a symbolic link | `tree_symlink_root` |
| A link inside the tree to a clean copy | copied as a link; scored FAIL `result_tree_modified` |
| A nested `.git` directory or file; a tree root named `.git`; a Unix socket | `tree_nested_repo`, `tree_nested_repo`, `tree_unsupported_object` |
| A top-level `.git` file and directory | not copied; PASS still possible |
| The source changed between the copy and the second hash, through a test hook | `tree_unstable_during_copy`, `--out` absent |
| `provenance.json` | exact keys and labels; no absolute path, home directory or user name anywhere under `--out` |
| `run.json` | decodes under `DisallowUnknownFields`; no verdict or truth key |
| Sources | reply file and tree unchanged: path, mode, size, mtime and SHA-256 |
| Every refusal | no `run.json` remains; `replay outcome` on `--out` is UNAVAILABLE, never PASS |
| A forged `provenance.json` beside a valid run | the score is unchanged |

The frozen mutants are listed in `internal/mutation/testdata/mutants.json` against the tests that kill them.
