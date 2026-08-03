# Onboarding

**Scope:** repository

## What it means

Two signals about how newcomers join the project:

- **Onboarding time** — the median time from a contributor's *first commit* to
  their *first surviving change* (a line they added that is still alive). It
  approximates how long it takes a new contributor to land something that sticks.
- **New contributors per month** — the average rate at which first-time
  contributors appear.

## How it is computed

- **Onboarding time** comes from the line-lifetime reconstruction (see
  [retention](./retention.md)): for each contributor it takes the gap between
  their earliest commit and the earliest commit whose added lines survive, then
  reports the median across contributors.
- **New contributors per month** divides the total number of contributors by the
  repository's active span in months (at least one month).

## How to read it

- A short onboarding time suggests newcomers can make durable contributions
  quickly; a long one may indicate a steep ramp-up.
- The per-month rate is a coarse average over the whole history, not a recent
  trend.
