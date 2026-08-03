# Retention

**Scope:** per contributor · **Two ponderations:** by-commit and by-day

## What it means

How long a contributor's **added** lines survive in the codebase. Lines that are
still present at `HEAD` count with full weight; lines that were removed count in
proportion to how long they lived before being deleted. It rewards durable
contributions over churn.

Two ponderations are produced:

- **by-commit** — a line's lifespan is measured in commits (how many commits it
  survived, relative to how many it could have).
- **by-day** — a line's lifespan is measured in days. A line removed on the same
  day it was added contributes zero.

## How it is computed

A separate replay of first-parent history

```
git log --first-parent --reverse -p --unified=0 -M
```

reconstructs per-line ownership: each current line records the commit that
introduced it. When a line is removed, the distance between the removing commit
and the birth commit gives its lifespan. The per-contributor score is the sum of
their added lines weighted by survival, reported as a rate against their tracked
additions. Retention is a lasting per-contributor quantity — it does not slide
with time.

## How to read it

- High retention means a contributor's code *stuck*; low retention means much of
  it was later rewritten or removed.
- `--first-parent` keeps reconstruction consistent on branchy histories, at the
  cost of attributing merged lines to the merging commit in old merge-heavy
  workflows.
- See also [alive lines](./alive-lines.md) for the count still present at `HEAD`.
