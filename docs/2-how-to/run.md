# How to run

Run `contributors` against a git repository and explore its terminal UI.

## Prerequisites

- A built binary (see [build.md](./build.md)), or use `make run` / `go run` to
  build and run in one step.
- A **`git`** binary on your `PATH`.
- The target directory must be inside a git repository.

## Run the binary

Analyze the current directory:

```sh
./bin/contributors
```

Analyze another repository by passing its path:

```sh
./bin/contributors /path/to/some/repo
```

If you ran `make install`, drop the `./bin/` prefix and just call
`contributors`.

## Run without building first

Use the `make run` target, passing any flags via `ARGS`:

```sh
make run                          # runs against the current directory
make run ARGS="/path/to/repo"     # runs against another repo
make run ARGS="--week-start sunday --no-cache"
```

Or invoke Go directly:

```sh
go run . /path/to/repo
```

## Common options

```sh
# Start the calendar week on Sunday
contributors --week-start sunday

# Exclude bot or noisy accounts (repeatable or comma-separated)
contributors --ignore "dependabot[bot]" --ignore ci@example.com

# Ignore all documentation changes (Markdown files and docs/ folders)
contributors --exclude-docs

# Force a full rescan, ignoring the on-disk cache
contributors --no-cache
```

| Flag            | Default  | What it does                                              |
| --------------- | -------- | -------------------------------------------------------- |
| `--week-start`  | `monday` | First day of the calendar week (`monday` or `sunday`).   |
| `--no-cache`    | `false`  | Ignore and overwrite the on-disk cache (force a rescan). |
| `--ignore`      | *(none)* | Names/emails to exclude. Repeatable or comma-separated.  |
| `--exclude-docs`| `false`  | Ignore changes to Markdown files (`*.md`) and `docs/` folders. |

Flags can also be supplied through a `.contributors.yaml` config file (in the
current directory or your home directory) or via `CONTRIBUTORS_`-prefixed
environment variables.

## Navigating the UI

The first run scans the repository (showing a progress bar); subsequent runs are
instant thanks to the on-disk cache. Once loaded:

| Key                   | Action                                              |
| --------------------- | --------------------------------------------------- |
| `tab` / `shift+tab`   | Switch between the Contributors and Activity tabs.  |
| `1` / `2`             | Jump straight to a tab.                             |
| `←` / `→` (`h` / `l`) | Switch tabs, or scroll weeks on the Activity tab.   |
| `↑` / `↓`             | Scroll the contributors list.                       |
| `m`                   | Toggle the per-contributor graph (commits ↔ lines). |
| `s`                   | Cycle sort: impact → commits → changes → recent.    |
| `/`                   | Search contributors by name or email.               |
| `f`                   | Filter by last-commit date (`YYYY-MM-DD`).          |
| `g` / `G`             | Jump the calendar to the start / end.               |
| `esc`                 | Clear the current search/filter.                    |
| `q` / `ctrl+c`        | Quit.                                               |
