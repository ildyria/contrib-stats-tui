# Active span

**Scope:** per contributor · **Unit:** days

## What it means

The elapsed time between a contributor's **first** and **last** commit — how long
they have been involved with the repository, regardless of how continuously.

## How it is computed

$$\text{active span} = \text{lastSeen} - \text{firstSeen}$$

measured in days. `firstSeen` and `lastSeen` are the timestamps of the
contributor's earliest and latest non-merge commits.

## How to read it

- Active span measures *tenure* (breadth of involvement over time), not
  *intensity*. A contributor with two commits a year apart has a large span but
  little activity.
- Pair it with [commits per active week](./commits-per-week.md) to distinguish
  steady contributors from occasional ones, and with the
  [active/dormant flag](./active-dormant.md) for recency.
