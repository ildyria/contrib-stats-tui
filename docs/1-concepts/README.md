# Concepts

Background and explanation to help you understand `contributors` before working
with it.

## Overview

`contributors` answers two questions about a git repository: *who* contributed,
and *when* activity happened. It does this purely from local git history — there
is no server, no account, and no network call. The core idea is to transform a
linear stream of commits into a small set of aggregated statistics, then render
those statistics as familiar visualizations (a leaderboard, a contribution
calendar, and a punch card).

A guiding principle is **fair ranking**. Raw line counts are easy to game (add a
vendored dependency, top the chart), so the default ranking uses an *Impact*
score that rewards being consistently active *and* changing meaningful amounts of
code — not just one of the two.

## Key terms

- **Contributor** — a single author, identified by name and (lowercased) email.
  Holds aggregated counts, temporal bounds, weekly activity series, and optional
  retention scores.
- **Summary** — the repository-wide aggregate: totals, the first/last commit
  range, the daily commit map, and the punch-card matrix.
- **Impact score** — the default ranking metric. A normalized geometric mean of a
  contributor's commit count and lines changed, so both dimensions matter:
  `sqrt((commits / maxCommits) × (changes / maxChanges))`, scaled to 0–100.
- **Contribution calendar** — a GitHub-style heatmap of commits per calendar day,
  with intensity buckets scaled to the busiest day.
- **Punch card** — a 7 × 24 matrix (weekday × hour) of commit times, exposing
  *when* work happens across the week.
- **Retention** — how long a contributor's added lines survive in the codebase.
  Lines still present at `HEAD` count with full weight; lines removed quickly
  count for less. Measured both in commits and in days.
- **Bus factor / alive lines** — retention-derived signals: how many lines a
  contributor authored are *still* present in the current `HEAD`.
- **Contribution profile** — a classification derived from a contributor's
  additions/deletions ratio: *builder* (net-adder), *refactorer* (heavy
  deleter), *demolisher* (deletions vastly outweigh additions), or *balanced*.
- **Collaboration breadth** — how widely a contributor ranges across the tree,
  based on the distinct files and directories they touch: *generalist* (broad)
  vs. *specialist* (concentrated). Combined with the profile it yields a cheeky
  per-person title.
- **Active / dormant** — a recency flag: a contributor is *active* if their last
  commit falls within a recent window of the repository's latest commit, else
  *dormant*.
- **Repository-wide metrics** — aggregate health signals such as the *Gini
  coefficient* of contribution, *time-to-legacy* (median age at which lines get
  overwritten), *onboarding time* (first commit to first surviving change), and
  *weekend / after-hours ratios* derived from the punch card.
- **Ignore list** — names or emails whose commits are dropped before aggregation
  (useful for bots and CI accounts).
- **Exclude docs** — an optional mode (`--exclude-docs`) that drops all changes
  to Markdown files (`*.md`) and files under `docs/` folders from every
  statistic.

## How the system works

The tool runs a channel-based pipeline so that history is never fully buffered in
memory:

1. **Collect** — the collection layer runs `git log --no-merges --numstat` and
   parses each commit into a `Commit` value, sending it on a channel. Git-specific
   parsing is isolated here.
2. **Aggregate** — the aggregation layer consumes the channel and folds each
   commit into a `Summary` on the fly. Commits from ignored authors are dropped
   before they touch any counter. Because aggregation happens as commits arrive,
   memory is bounded by the number of contributors and weeks, not the number of
   commits.
3. **Retention (optional)** — a separate replay of first-parent history
   (`git log --first-parent -p --unified=0 -M`) tracks per-line ownership to
   compute how long each author's lines survive, plus each contributor's
   collaboration breadth and repository-wide metrics such as time-to-legacy and
   onboarding time.
4. **Cache** — the finished `Summary` is serialized to disk, keyed by `HEAD` (plus
   the ignore list and the build), so subsequent runs are instant until the
   history changes.
5. **Render** — the UI layer (Bubble Tea + Lip Gloss) turns the `Summary` into the
   contributors leaderboard, the contribution calendar, the punch card, and the
   per-contributor sparklines.

## Architecture

At a high level the code is split into two Go packages under `internal/`:

- **`gitstats`** — the data layer: collection, aggregation, retention, and
  caching. It defines the shared `Commit`, `Contributor`, and `Summary` types and
  the available sort keys.
- **`ui`** — the presentation layer: a Bubble Tea model that renders the tabs
  (Contributors and Activity), handles keybindings, search, filtering, and
  sorting.

`main.go` wires configuration (Cobra + Viper) and launches the UI. For the
detailed, living architecture — component map, data flows, and technology stack —
see [`../4-specifications/Architecture.md`](../4-specifications/Architecture.md).

## Important rules and conventions

- **Identity is by email (lowercased).** Contributors are keyed on their email
  address, normalized to lowercase, so casing differences don't split one person
  into two.
- **Merge commits are excluded.** Collection uses `--no-merges`, so merge commits
  never inflate anyone's counts.
- **Ranking is deliberately not just lines.** The default Impact score is a
  geometric mean, so being strong on only one axis (commits *or* lines) is not
  enough to top the chart.
- **Streaming, not buffering.** The collector and aggregator communicate over a
  channel; the full commit history is never held in memory at once.
- **Cache is keyed by `HEAD` + inputs.** The on-disk cache is invalidated when
  `HEAD`, the ignore list, the `--exclude-docs` setting, or the build changes;
  `--no-cache` forces a rescan.
- **Offline by design.** The tool shells out only to `git` and performs no network
  I/O.
