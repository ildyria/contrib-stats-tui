# Punch card

**Scope:** repository-wide · **Type:** heatmap (weekday × hour)

## What it means

A 7 × 24 matrix of commit times that reveals *when in the week* work happens.
Rows are weekdays, columns are hours of the day, and each cell's intensity is the
number of commits made in that weekday/hour slot.

## How it is computed

During aggregation every commit increments one cell of a `[7][24]` matrix,
indexed by the weekday and hour of the commit's **local** timestamp. The row
order respects `--week-start`.

From this matrix the view also derives:

- **Busiest slot** — the single weekday/hour cell with the most commits.
- **Most active hour** — the hour column with the highest total across all days.
- **Busiest day** — the weekday row with the highest total.
- **Weekend & after-hours ratios** — see
  [weekend-after-hours.md](./weekend-after-hours.md).

## How to read it

- Cell intensity is relative to the busiest slot in this repository.
- Times are local to each commit, so the punch card reflects contributors'
  own working hours rather than a single fixed zone.
- Clusters outside 08:00–18:00 or on Saturday/Sunday hint at work-life patterns,
  quantified by the weekend and after-hours ratios.
