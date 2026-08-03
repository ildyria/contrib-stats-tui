# Contribution calendar

**Scope:** repository-wide · **Type:** heatmap

## What it means

A GitHub-style calendar heatmap showing how many commits landed on each calendar
day. Each cell is one day; darker/warmer cells mean more commits. It answers
*when*, at a day granularity, work happened across the repository's lifetime.

## How it is computed

During aggregation every commit is bucketed by its local calendar day (midnight
in the commit's own time zone). The result is a map of day → commit count that
spans from the first to the last commit in history.

Cell intensity is bucketed relative to the **busiest single day**, so the scale
adapts to each repository rather than using absolute thresholds. A legend shows
the count range each shade represents.

The first weekday of each row is controlled by `--week-start` (`monday` or
`sunday`). Year and month labels flank the grid, and the view can be scrolled
horizontally through the repository's weeks.

## How to read it

- Colour encodes *volume per day*, normalised to the busiest day — two
  repositories' shades are not directly comparable.
- Long empty stretches indicate dormancy; dense bands indicate sprints.
- Times are local to each commit, so contributors in different time zones are
  each counted on their own calendar day.
