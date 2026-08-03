# Gini coefficient

**Scope:** repository · **Range:** 0–1

## What it means

A single number describing how *evenly* contribution is distributed across
contributors, measured by total lines changed (`additions + deletions`):

- **0** — perfectly equal; everyone contributed the same amount.
- **near 1** — highly unequal; a few people did almost all the work.

It is the classic "is this a one-person show?" indicator.

## How it is computed

The standard sorted-rank formula over each contributor's total lines changed:

$$G = \frac{2 \sum_{i=1}^{n} i \cdot x_i}{n \sum_{i=1}^{n} x_i} - \frac{n+1}{n}$$

where values `x_i` are sorted ascending and `n` is the number of contributors.
The result is clamped to be non-negative; it is 0 when there is no contribution.

## How to read it

- High Gini is normal and healthy for many projects (a core maintainer plus
  occasional contributors); interpret it in context.
- It measures distribution of *volume*, not durability — combine with the
  [bus factor](./bus-factor.md), which is based on surviving code.
