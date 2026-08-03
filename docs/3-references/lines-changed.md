# Lines changed (additions / deletions)

**Scope:** per contributor (and repository total)

## What it means

The number of lines a contributor added and removed across all their commits:

- **Additions** — lines added.
- **Deletions** — lines removed.
- **Changes** — the sum, `additions + deletions`, used by other metrics.

## How it is computed

Collection runs `git log --no-merges --numstat` and sums the per-file additions
and deletions for every non-merge commit. Binary files report `-` in numstat and
contribute zero. When `--exclude-docs` is set, changes to Markdown files (`*.md`)
and files under `docs/` folders are excluded from these totals.

## How to read it

- Raw line counts are easy to inflate (generated code, vendored dependencies),
  which is why ranking defaults to the [impact score](./impact-score.md) rather
  than lines alone.
- The additions/deletions balance feeds the [contribution profile](./contribution-profile.md).
- The per-contributor sparkline can display lines added/removed per week; toggle
  it with `m`.
