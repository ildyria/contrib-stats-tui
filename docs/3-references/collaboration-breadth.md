# Collaboration breadth

**Scope:** per contributor · **Type:** classification

## What it means

How widely a contributor ranges across the codebase, based on the distinct
**files** and **directories** in which they added or removed lines:

- **generalist** — touches a broad slice of the tree.
- **specialist** — concentrates on a narrow set of directories.

The card shows the raw counts (files / dirs) alongside the label.

## How it is computed

During the retention replay, the tool records the set of files each contributor
touched and the set of directories those files live in. A contributor is a
**generalist** when the number of directories they touched is at least half of
the widest-ranging contributor's directory count:

$$\text{dirs} \times 2 \ge \text{maxDirs}$$

otherwise they are a **specialist**. The classification is therefore *relative*
to the broadest contributor in the repository.

## How to read it

- Breadth is about *spread*, not *volume*: a specialist can still be a top
  contributor within their area.
- It is combined with the [contribution profile](./contribution-profile.md) to
  pick a contributor's [title](./titles.md).
- Generalist and specialist each have their own color in the UI.
