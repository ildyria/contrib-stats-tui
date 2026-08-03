# How to limit the history window

By default `contributors` scans every commit back to the very first one. For
most repositories that's fine. For repositories with years of history, hundreds
of contributors, or a long tail of one-off drive-by commits, you probably care
more about *recent* activity. Here's how to tell it to stop caring about the
past.

## Time window: last N months

`time-window` limits the scan to commits authored within the last N months,
measured from the HEAD commit's date (not today's date, so the result is stable
regardless of when you run it).

```sh
# Only look at the last 6 months
contributors --time-window 6

# Or in the config
echo "time-window: 6" >> .contributors.yaml
```

Via environment variable:

```sh
CONTRIBUTORS_TIME_WINDOW=6 contributors
```

## Commit window: last N commits

`commit-window` limits the scan to the most recent N commits, regardless of
when they were made.

```sh
# Only look at the last 500 commits
contributors --commit-window 500
```

```yaml
# .contributors.yaml
commit-window: 500
```

## Combining both

Both constraints can be active at the same time. Git applies them together, so
the result is whichever cuts off first — the time boundary or the commit count.

```yaml
time-window: 6      # go back at most 6 months
commit-window: 1000 # but never more than 1000 commits
```

## Cache invalidation

The window values are part of the cache key. Changing `time-window` or
`commit-window` (or removing them) automatically triggers a rescan — you don't
need to pass `--no-cache` manually.

## Auditing who contributed recently with `email`

The `email` command (plain text, no TUI) is especially useful with a window when
you want to answer "who has touched this in the last quarter?":

```sh
contributors email --time-window 3 /path/to/repo
```

Output:
```
Alice Smith: alice@corp.com (42 commits)
Bob Jones:   bob@example.com (17 commits)
...

3 unique contributors
```

Pipe it, count it, grep it — it's just text.

## When to use a window

| Situation | Suggestion |
| --------- | ---------- |
| Large monorepo with years of history | `time-window: 12` to focus on the last year |
| Want to know who's *actively* contributing | `time-window: 3` (last quarter) |
| Repo with a huge noisy import commit | `commit-window` starting just after that commit count |
| Fast scan for CI / dashboards | `commit-window: 200` keeps it snappy |
| Full historical record | Omit both — full history is the default |
