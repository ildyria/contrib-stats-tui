# Commit size distribution

**Scope:** repository-wide · **Type:** histogram + summary

## What it means

How large commits tend to be, measured in **lines changed** (additions +
deletions) per commit. It distinguishes a codebase built from many small,
reviewable commits from one dominated by occasional huge ones.

## How it is computed

During aggregation each commit's total lines changed is placed into a histogram
(`CommitSizeHist`) bucketed by `CommitSizeBuckets`. Two summary figures are
derived from the histogram without buffering every commit:

- **Median commit size** — the median lines changed per commit, interpolated
  within the bucket that contains the middle commit (`MedianCommitSize`).
- **Large commits** — the count of commits that changed **more than 500 lines**
  (`LargeCommitThreshold = 500`), also shown as a percentage of all commits.

## How to read it

- A low median with few large commits suggests small, incremental changes.
- A high share of large commits can indicate vendored dependencies, generated
  code, squashed merges, or big-bang rewrites.
- Because it uses a histogram, the median is an interpolated estimate, not an
  exact per-commit sort.
