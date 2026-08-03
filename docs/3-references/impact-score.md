# Impact score

**Scope:** per contributor · **Range:** 0–100 · **Default sort key**

## What it means

A bias-resistant ranking score that rewards contributors who are strong on
*both* activity and volume, rather than one alone. It is designed so that a
single large vendored dependency (many lines, one commit) or a flurry of trivial
commits (many commits, few lines) does not top the chart.

## How it is computed

Each dimension is normalized to the busiest contributor in the repository, then
combined via a geometric mean:

$$\text{impact} = \sqrt{\frac{\text{commits}}{\text{maxCommits}} \times \frac{\text{changes}}{\text{maxChanges}}}$$

where `changes = additions + deletions`. The result (0–1) is scaled to 0–100 for
display.

## How to read it

- A high score requires doing well on **both** commits and lines changed.
- Because it is relative to the busiest contributor, scores are only comparable
  *within the same repository*, not across repos.
- Select it explicitly by cycling the sort order with `s`; it is the default.
