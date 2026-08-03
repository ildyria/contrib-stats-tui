# Contribution profile

**Scope:** per contributor · **Type:** classification

## What it means

A label describing *how* a contributor changes code, derived from the balance
between the lines they add and the lines they remove:

- **builder** — a net-adder who mostly writes new code.
- **balanced** — adds and removes in comparable measure.
- **refactorer** — removes heavily relative to additions.
- **demolisher** — deletions vastly outweigh additions.

## How it is computed

Let `r = additions / deletions`. The profile is assigned as:

| Condition | Profile |
| --------- | ------- |
| `deletions == 0` (and additions > 0) | builder |
| `r ≥ 2` | builder |
| `r ≤ 0.2` | demolisher |
| `r ≤ 0.5` | refactorer |
| otherwise | balanced |
| `additions == 0` and `deletions == 0` | idle |

The additions:deletions ratio is also shown in human-readable form (e.g.
`3.4:1`).

## How to read it

- The profile describes *style*, not quality — a refactorer or demolisher removing
  dead code is often doing valuable work.
- It is combined with [collaboration breadth](./collaboration-breadth.md) to pick
  a contributor's [title](./titles.md).
- Each profile has its own color in the UI.
