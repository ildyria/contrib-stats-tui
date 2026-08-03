# contributors

> A terminal UI that turns any git repository into a GitHub-style contributors page, contribution calendar, and activity heatmap — entirely offline.

## What it is

`contributors` is a single-binary command-line tool, written in Go, that renders
an interactive terminal UI (TUI) for a git repository. Point it at a repo and it
reads the git history, computes per-contributor statistics, and paints a
GitHub-style contributors leaderboard, a contribution calendar, and a
weekday × hour activity punch card — all inside your terminal.

## What it does

- Ranks contributors by a bias-resistant **Impact** score (as well as by
  commits, lines changed, or most-recently-active).
- Classifies each contributor with a **profile** (builder, refactorer,
  balanced, or demolisher) and a **collaboration breadth** (generalist vs.
  specialist), surfaced as a cheeky per-person title.
- Flags whether each contributor is **still active** or **dormant** at a glance.
- Renders a **contribution calendar** heatmap of daily commit activity.
- Renders an **activity punch card** — a weekday × hour matrix of commit times.
- Draws **per-contributor sparklines** (commits per week, or lines added/removed).
- Derives repository-wide signals such as **bus factor**, **Gini coefficient**,
  **time-to-legacy**, **onboarding time**, and **weekend / after-hours ratios**.
- Supports **search, filtering, and ignore lists** to focus on the people that
  matter and mute bots, plus **`--exclude-docs`** to drop Markdown and `docs/`
  changes.
- Caches results **on disk**, keyed by `HEAD`, so repeat runs are instant until
  the history changes.
- Makes **zero network calls** — it is just `git` and math.

## Who it's for

Developers, team leads, and maintainers who live in the terminal and want to
understand *who* built a codebase and *how* activity is distributed over time —
without opening a browser, logging into a hosting provider, or sending any data
off-machine.

## Why it exists

Contribution insights normally live behind a web UI tied to a specific hosting
provider (GitHub, Azure DevOps, etc.), require network access and authentication,
and often reward raw line counts over genuine impact. `contributors` brings the
same visual insights to any local git repository, offline, and ranks people with
an Impact score designed so that a single large vendored dependency doesn't crown
someone the "top contributor."

## How it fits together

The tool is a short pipeline:

1. **Collector** streams `git log --numstat` and parses each commit.
2. **Aggregator** folds that stream into a `Summary` as commits arrive, keeping
   memory bounded by the number of contributors and weeks rather than commits.
3. **Cache** stores the finished `Summary` on disk, keyed by `HEAD`, the ignore
   list, the `--exclude-docs` setting, and the build.
4. **UI** (Bubble Tea + Lip Gloss) renders the leaderboard, calendar, punch card,
   and sparklines.

Configuration is layered: **flags → environment variables → config file
(`.contributors.yaml`) → defaults**.

## Getting started

Requires **Go 1.26+** and a `git` binary on your `PATH`.

```sh
make build        # builds ./bin/contributors
./bin/contributors                 # analyze the current directory
./bin/contributors /path/to/repo   # analyze another repo
```

Press `tab` to switch views, `s` to cycle the sort order, `/` to search, and `q`
to quit. See the root [`README.md`](../../README.md) for the full flag and
keybinding reference.
