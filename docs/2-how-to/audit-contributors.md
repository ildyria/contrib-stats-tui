# How to audit contributors with the `email` command

The TUI is great for exploring, but sometimes you just need a list. Maybe you
want to count unique contributors, pipe the output into another tool, or copy
a table into a Confluence page while sighing quietly to yourself. The `email`
subcommand is for that.

## Basic usage

```sh
# Scan the current directory
contributors email

# Scan a specific repo
contributors email /path/to/repo

# Use a config file (picks up repositories:, ignore:, users:, windows, etc.)
contributors email .contributors.yaml
contributors email resc.yaml
```

## Output format

Plain text, one line per unique contributor, sorted by commit count (descending):

```
Alice Smith:     alice@corp.com            (312 commits)
Bob Jones:       bob@example.com           ( 47 commits)
dependabot[bot]: 49699333+dependabot[bot]@users.noreply.github.com ( 23 commits)

3 unique contributors
```

The summary line at the bottom tells you the total count — which is the number
most people actually want.

## Respects all config settings

`contributors email` honours the same config as the TUI:

- **`repositories:`** — scan multiple repos and aggregate across them
- **`ignore:`** — excluded authors don't appear in the list
- **`users:`** — aggregated identities are merged (so "Alice from three email
  addresses" shows up as one line, not three)
- **`time-window:` / `commit-window:`** — limit the history before counting

```sh
# Who contributed in the last 3 months?
contributors email --time-window 3

# Same thing, via env var
CONTRIBUTORS_TIME_WINDOW=3 contributors email /path/to/repo
```

## Counting unique contributors

```sh
# Just the number
contributors email | tail -1
# → 42 unique contributors

# Or with grep for the paranoid
contributors email . | grep "unique contributors"
```

## Piping and grepping

Because it's plain text:

```sh
# Find all contributors from a specific domain
contributors email . | grep "@mycorp\.com"

# Sort differently (e.g. alphabetically)
contributors email . | grep -v "unique contributors" | sort

# Count lines (excluding the blank line and summary)
contributors email . | grep -c "commits"
```

## When to use `email` vs the TUI

| Task | Use |
| ---- | --- |
| Explore, search, drill into individuals | TUI (`contributors`) |
| Count unique contributors | `contributors email` |
| Copy a list into a doc / ticket | `contributors email` |
| Pipe into another script | `contributors email` |
| Find out if a specific person contributed | `contributors email \| grep "name"` |
| Present to management with pretty heatmaps | TUI |
