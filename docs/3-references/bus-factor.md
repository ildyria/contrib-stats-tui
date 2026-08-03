# Bus factor

**Scope:** repository · **Unit:** number of contributors

## What it means

The fewest contributors whose surviving code together makes up at least half of
all living lines. A **low** bus factor flags concentration risk: much of the code
that is still in use was written by very few people.

## How it is computed

Using [alive lines](./alive-lines.md) (lines still present at `HEAD`):

1. Sort contributors by alive lines, descending.
2. Accumulate until the running total reaches **50%** of all alive lines.
3. The number of contributors needed is the bus factor; their combined share is
   also reported.

If no surviving lines can be attributed, the bus factor is 0.

## How to read it

- A bus factor of 1–2 means the living codebase depends heavily on a couple of
  people — risky if they leave.
- It is based on *surviving* code, so it reflects who currently matters, not who
  was historically busiest.
- Cross-reference with [active/dormant](./active-dormant.md): a low bus factor
  concentrated in dormant contributors is a stronger warning.
