# Weekend & after-hours ratios

**Scope:** repository · **Unit:** percentage of commits

## What it means

Two "work-life" indicators derived from *when* commits happen:

- **Weekend ratio** — the share of commits made on Saturday or Sunday.
- **After-hours ratio** — the share of commits made outside 08:00–18:00.

## How it is computed

Both come from the [punch card](./punch-card.md) (the weekday × hour matrix of
commit times):

- **Weekend** — commits whose weekday is Saturday or Sunday, divided by all
  commits.
- **After-hours** — commits whose hour is before 08:00 or at/after 18:00, divided
  by all commits.

Commit times use each commit's own local time zone as recorded in git.

## How to read it

- These are lighthearted indicators, not judgments — time zones, flexible
  schedules, and automation can all skew them.
- A high after-hours or weekend ratio can nonetheless hint at delivery pressure or
  a globally distributed team.
