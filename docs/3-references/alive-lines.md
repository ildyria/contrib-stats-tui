# Alive lines

**Scope:** per contributor · **Unit:** lines

## What it means

The number of lines a contributor authored that are **still present** in the
current `HEAD`. It is the "living legacy" of a contributor: code that survived
every subsequent edit.

## How it is computed

Derived from the same first-parent line-lifetime reconstruction used for
[retention](./retention.md). Every added line whose ownership record still points
to the contributor at the final commit is counted as alive.

## How to read it

- Alive lines measure *lasting footprint*, distinct from total additions (which
  include lines later removed).
- It is the basis of the [bus factor](./bus-factor.md): concentration of alive
  lines in few people signals project risk.
- Sort contributors by alive lines by cycling the sort order with `s`.
