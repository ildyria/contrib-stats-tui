# Contributor titles

**Scope:** per contributor · **Type:** flavour label (no spoilers below)

## What it means

Every contributor is awarded a small, tongue-in-cheek **title** shown next to
their name. It is purely for fun — a friendly nickname distilled from two real
metrics — and carries no ranking weight of its own.

## How it is derived

A title is chosen from the pairing of a contributor's two classifications:

- their [contribution profile](./contribution-profile.md) — builder, refactorer,
  balanced, or demolisher; and
- their [collaboration breadth](./collaboration-breadth.md) — generalist or
  specialist.

Each combination maps to its own honorific, coloured to match the profile. When
breadth cannot be determined (no tracked files), a simpler profile-only title is
used instead. The raw labels are always shown in parentheses after the title, so
the underlying metrics are never hidden.

## How to read it

- The title is a *summary* of the profile and breadth, not an extra metric —
  read the two labels in parentheses for the facts.
- Titles change as a contributor's balance of additions/deletions and their
  spread across the tree change.

## No spoilers

The specific titles are intentionally **not listed here** — discovering which one
you (or your colleagues) earned is half the fun. Point the tool at a repository
and look next to each name. If you truly must know every combination, they live
in the `contribTitle` function in
[`internal/ui/contributors.go`](../../internal/ui/contributors.go).
