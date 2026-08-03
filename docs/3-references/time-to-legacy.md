# Time-to-legacy

**Scope:** repository · **Unit:** days (median)

## What it means

The **median age at which a line gets overwritten** — how long, typically, a line
of code lives before it is removed or replaced. It is a measure of code churn and
turnover: short times mean code is rewritten quickly; long times mean code tends
to stick.

## How it is computed

During the first-parent line-lifetime reconstruction (see
[retention](./retention.md)), every line that is removed contributes its age (in
days) to a histogram. Time-to-legacy is the median of that distribution,
interpolated within the bucket that contains the middle observation. Lines still
alive at `HEAD` are, by definition, not yet "legacy" and are excluded.

## How to read it

- A short time-to-legacy indicates high churn — code is frequently reworked.
- A long time-to-legacy indicates stability — changes tend to persist.
- Because it only considers lines that *died*, it describes the turnover of
  replaced code, not the whole codebase.
