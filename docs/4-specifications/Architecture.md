# Architecture — Service Snapshot

> This file is the living architecture document for this service/repository.
> It is maintained alongside code — updated by the Implementation Agent when changes affect architecture.
> Agents load this file to understand the system before making changes.

---

## System Context

`contributors` is a standalone, offline command-line application. It reads a
local git repository by shelling out to the `git` binary, computes statistics,
caches them on the local filesystem, and renders an interactive terminal UI. It
has no server component, no external dependencies at runtime beyond `git`, and
makes no network calls.

**Service name**: contributors (Go module `github.com/ildyria/contrib-stats-tui`)
**Domain**: Developer tooling / repository analytics
**Owning team**: Nobody
**Classification**: Supporting (local developer tool)

### System Context Diagram

```
[User in terminal] <--> [contributors TUI]
                              |
                              +--> git binary (git log / rev-list / rev-parse)
                              +--> local git repository (.git history)
                              +--> on-disk cache (keyed by HEAD)
```

All interactions are local. There are no upstream consumers, downstream services,
message brokers, or external APIs.

---

## Component Map

| Component | Responsibility | Key Files/Packages |
|---|---|---|
| CLI / entry point | Parses flags, loads config (Cobra + Viper), launches the UI program | `main.go` |
| Collection layer | Runs `git log` and parses output into a stream of `Commit` values | `internal/gitstats/collector.go` |
| Aggregation layer | Folds the commit stream into a `Summary` (per-contributor + repo totals) | `internal/gitstats/aggregator.go` |
| Shared types & wiring | Defines `Commit`, `Contributor`, `Summary`, sort keys; wires collect → aggregate | `internal/gitstats/gitstats.go` |
| Retention analysis | Replays first-parent history to compute line survival / retention scores, per-contributor collaboration breadth, and repo-wide line-lifetime metrics | `internal/gitstats/retention.go` |
| Cache | Serializes/deserializes the `Summary` to disk, keyed by `HEAD` + inputs (ignore list, `--exclude-docs`, build) | `internal/gitstats/cache.go` |
| UI (root model) | Bubble Tea model, tab switching, keybindings, search/filter/sort, loading state | `internal/ui/ui.go`, `internal/ui/messages.go` |
| UI views | Renders the leaderboard (with profile / breadth / title / active-status), calendar, punch card/heatmap, and sparklines | `internal/ui/contributors.go`, `internal/ui/calendar.go`, `internal/ui/heatmap.go`, `internal/ui/activity.go` |
| UI support | Shared styles and helpers | `internal/ui/styles.go`, `internal/ui/helpers.go` |

---

## Data Flows

### Primary Flow: Analyze a repository and render the UI

```
1. [CLI] Resolve path, week-start, ignore list, cache and exclude-docs settings (flags/env/config)
2. [Cache] Look up a cached Summary keyed by HEAD + ignore list + exclude-docs + build
     └─ hit  → skip collection, go straight to render
     └─ miss → continue
3. [Collector] Run `git log --no-merges --numstat`, parse each commit,
     send Commit values on a channel (in a goroutine); skip doc files when
     --exclude-docs is set
4. [Aggregator] Consume the channel; drop ignored authors; fold each Commit
     into per-contributor stats and the repo-wide Summary
5. [Retention] Concurrently replay first-parent history (own goroutine) to weight
     added lines by how long they survive, compute per-contributor collaboration
     breadth, and derive repo-wide metrics (time-to-legacy, onboarding); merged
     into the Summary once both walks finish
6. [Cache] Persist the finished Summary to disk
7. [UI] Render the Contributors and Activity tabs; handle key input
```

### Error / Exception Flow

```
1. Invalid --week-start value        → error returned before the UI starts
2. Not a git repo / git not on PATH  → git command fails; scan error surfaced
                                        via the model and returned as exit error
3. Corrupt or stale cache            → treated as a miss; a full rescan runs
4. Config file not found             → ignored (defaults + flags/env are used)
```

---

## Integration Points

| Integration | Type | Direction | Protocol | Auth | SLA |
|---|---|---|---|---|---|
| `git` binary | Synchronous (subprocess) | Outbound | Process exec (`os/exec`) | Local user | — |
| Local git repository | Read-only | Inbound | Filesystem / git history | Local user | — |
| On-disk cache | Read/write | Local | Filesystem | Local user | — |

There are no network integrations. The tool makes zero network calls.

---

## Technology Stack

| Layer | Technology | Version |
|---|---|---|
| **Language / Runtime** | Go | 1.26.1 |
| **TUI framework** | Bubble Tea | v1.3.10 |
| **TUI widgets** | Bubbles | v1.0.0 |
| **Styling** | Lip Gloss | v1.1.0 |
| **Charts / sparklines** | ntcharts | v0.5.1 |
| **CLI** | Cobra | v1.10.2 |
| **Configuration** | Viper | v1.21.0 |
| **Utilities** | samber/lo | v1.53.0 |
| **External dependency** | `git` binary (on `PATH`) | — |
| **Database** | None (on-disk cache only) | — |
| **Message broker** | None | — |
| **Container / Orchestration / IaC** | None (distributed as a single binary) | — |

---

## Non-Functional Requirements

| Attribute | Target | Measurement |
|---|---|---|
| **Memory footprint** | Bounded by #contributors × #weeks, not #commits | Streaming aggregation over a channel |
| **Repeat-run latency** | Near-instant on unchanged history | On-disk cache keyed by `HEAD` |
| **Privacy** | No data leaves the machine | Zero network calls (git + math only) |
| **Portability** | Single static binary | `go build` / `make build` |

<!-- Formal availability/latency/throughput SLAs are not applicable to a local CLI tool. -->

---

## Security Considerations

- **Authentication / Authorization**: None. The tool runs with the invoking
  user's local permissions and only reads repositories that user can already
  access.
- **Encryption**: Not applicable — no data in transit (no network I/O). The
  on-disk cache is stored unencrypted on the local filesystem.
- **Secrets management**: None required; the tool handles no credentials or
  tokens.
- **PII handling**: Git history contains author names and email addresses. These
  are read from the local repository and displayed in the UI and stored in the
  local cache; they are never transmitted off-machine. The `--ignore` option can
  exclude specific names/emails from the output.
- **Input handling**: All external input comes from `git` output, which is parsed
  using an ASCII record separator to remain unambiguous even with unusual author
  names.

---

## Decisions Log

<!-- Record architectural decisions here or reference ADR files. Dates below are inferred from the design and are not backed by explicit ADRs in the repository. -->

| Date | Decision | Rationale | Status |
|---|---|---|---|
| — | Channel-based collect → aggregate pipeline | Keeps memory bounded by contributors/weeks instead of buffering all commits | Accepted |
| — | Rank by Impact (geometric mean of commits × lines) by default | Resists gaming by raw line count (e.g. vendored dependencies) | Accepted |
| — | On-disk cache keyed by `HEAD` + ignore list + `--exclude-docs` + build | Makes repeat runs instant while staying correct when history or inputs change | Accepted |
| — | Offline-only, shell out to `git` | Privacy and portability; no network or auth required | Accepted |
| — | Contributors keyed by lowercased email | Avoids splitting one person across casing/name variations | Accepted |
| — | Retention pass runs concurrently with collection in its own goroutine | Overlaps the two full-history git walks to roughly halve collection time | Accepted |
| — | Derive per-contributor profile / breadth / title and repo-wide line-lifetime metrics from the retention replay | Reuses the single line-ownership reconstruction instead of extra git walks | Accepted |
| — | `--exclude-docs` drops Markdown and `docs/` changes from every statistic | Lets code-focused analysis ignore documentation churn | Accepted |
