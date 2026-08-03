# Commits

**Scope:** per contributor (and repository total)

## What it means

The number of commits authored by a contributor. Merge commits are excluded, so
this counts only commits that carry actual changes.

## How it is computed

Collection runs `git log --no-merges`, so merge commits never reach the counter.
Each commit is attributed to its author, identified by **email (lowercased)** so
that casing differences do not split one person into two identities.

## How to read it

- A high commit count reflects *frequency* of contribution, not *volume* — pair
  it with [lines changed](./lines-changed.md) or the [impact score](./impact-score.md)
  for a fuller picture.
- Commits from contributors on the `--ignore` list are dropped before counting.
- Sort contributors by commits by cycling the sort order with `s`.
