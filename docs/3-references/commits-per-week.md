# Commits per active week

**Scope:** per contributor · **Unit:** commits / week

## What it means

A contributor's commit *frequency* while they were actually contributing. It
divides total commits by the number of weeks in which they made at least one
commit — so long idle gaps do not dilute the figure.

## How it is computed

$$\text{commits per active week} = \frac{\text{commits}}{\text{active weeks}}$$

where *active weeks* is the count of week buckets in the contributor's weekly
series that contain one or more commits. If a contributor has no active weeks,
the value is 0.

## How to read it

- Unlike [active span](./active-span.md), this measures *intensity* rather than
  tenure: it answers "when they were working, how often did they commit?"
- A high value indicates bursty, frequent work within active periods; a low value
  indicates sparse commits even during active weeks.
