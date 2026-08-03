# How to merge contributor identities

Sometimes the same human shows up as several people. Different email at work,
different email at home, a GitHub noreply address, a bot account that's actually
just a script they wrote. By default `contributors` counts each unique email
separately. Here's how to fold them together.

## The problem

```
Alice Smith: alice@corp.com        (312 commits)
alice:       alice@personal.dev    ( 47 commits)
asmith:      49134512+asmith@users.noreply.github.com (11 commits)
```

Three entries, one human, and the leaderboard is lying to everyone.

## The solution: `users:` in your config

Add a `users:` section to `.contributors.yaml`. Each entry has a `display-name`
(required), an optional `display-email` shown in the UI, and lists of `emails`
and `usernames` to match. Any commit whose author email or author name matches
one of those patterns gets attributed to that entry.

```yaml
users:
  - display-name: Alice Smith
    display-email: alice@corp.com
    emails:
      - alice@corp\.com
      - alice@personal\.dev
      - .*\+asmith@users\.noreply\.github\.com
    usernames:
      - asmith
```

Now you get one entry:

```
Alice Smith: alice@corp.com  (370 commits)
```

## Patterns are regular expressions

`emails` and `usernames` are **case-insensitive regular expressions**, matched
anywhere in the value (not anchored). That means:

```yaml
# Matches any noreply GitHub address, regardless of username
emails:
  - .*@users\.noreply\.github\.com

# Matches any name ending in [bot]
usernames:
  - .*\[bot\]
```

Dot `.` is a regex metacharacter — if you want a literal dot, escape it: `\.`.

## Folding bots into one entry

A common use-case is grouping all bot accounts into a single "Bots" contributor
so they don't pollute the leaderboard:

```yaml
users:
  - display-name: Bots
    display-email: bots@example.com
    emails:
      - .*@users\.noreply\.github\.com
    usernames:
      - .*\[bot\]
      - dependabot
      - renovate
```

## Validating your patterns

Invalid regular expressions are caught before scanning starts. Run
`contributors email` (the plain-text command) to get a fast error without
launching the full TUI:

```sh
contributors email .
# Error: parsing users: users "Bots": invalid pattern "([unterminated": ...
```

## Tips

- The **first entry that matches** wins; order matters when patterns could
  overlap.
- `display-email` is purely cosmetic. The actual grouping key is the first
  email pattern (sorted alphabetically after normalization), which stays stable
  across repositories so multi-repo aggregation works correctly.
- You can use `usernames` alone (no `emails`) for accounts that always use a
  consistent name but chaotic email addresses.
- Run `contributors config` to generate a starter `.contributors.yaml` with
  commented examples.
