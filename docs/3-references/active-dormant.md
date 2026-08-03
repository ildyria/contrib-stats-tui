# Active / dormant

**Scope:** per contributor · **Type:** flag

## What it means

A recency flag showing whether a contributor is still contributing:

- **active** — committed recently.
- **dormant** — has not committed within the recent window.

It is shown as a colored dot before the contributor's name (green when active,
dark red when dormant) and as a word on the stats line.

## How it is computed

A contributor is **active** when their last commit falls within **26 weeks** of
the repository's most recent commit:

$$\text{lastSeen} \ge \text{repoLastCommit} - 26\ \text{weeks}$$

The window is measured against the dataset's most recent commit (not the wall
clock), so the flag is deterministic and stable across cached runs.

## How to read it

- "Recent" is relative to the repository's own timeline. A repo whose last commit
  was years ago can still show recent contributors as active if they committed in
  the final 26 weeks of its history.
- Use it to spot knowledge that may be walking out the door when combined with
  [alive lines](./alive-lines.md) and the [bus factor](./bus-factor.md).
