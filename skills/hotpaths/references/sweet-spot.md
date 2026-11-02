# The sweet spot — impact × complexity, with measured heat

How a hot entry becomes a placed candidate, and how `hotpaths` reports it. The
premise: **performance and simplicity are both assets, and most performance advice
fails by pricing only one of them.** This rubric prices both, every time.

Adapted from the generic part of the `performance` plugin, which this skill does
not need installed:

- `/srv/projects/trii/core/ai-marketplace/plugins/performance/references/sweet-spot.md`
  — the axes (lines 16-33), the quadrants (39-49), depth and target (55-71), the
  report format (79-126) and the over-engineering guide (130-146).
- `/srv/projects/trii/core/ai-marketplace/plugins/performance/skills/review/SKILL.md`
  — the Iron Law (lines 14-20).

Left out on purpose: the Trii catalog and its RTT sanity floors, the profile blocks
and the per-user pattern store. What changed is the impact axis. In the plugin it is
arithmetic read off the code and is measured only at its deepest level; here heat
and magnitude come from `measure`'s attribution, and arithmetic read off the code is
never reported as measured.

## The Iron Law

```
NO CANDIDATE WITHOUT A MEASURED SHARE + AN IMPACT × COMPLEXITY PRICE TAG
```

Two failure modes destroy trust in a performance review. **Premature
optimisation** flags waste nobody feels and pushes cleverness onto paths that run
once a day. **Vague alarmism** says "this could be faster" with no number attached.
Every candidate rests on two things actually established: (1) the code sits where
the probe spends its time, which here is its measured share, and (2) the impact is
estimable from that share and the probe's metric. Every candidate carries its
**complexity price**: the failure modes, abstraction and reading burden the faster
version adds. When the heat or the magnitude cannot be established, the candidate
is ⚪ needs measurement, not a finding.

## Axis 1 — Performance impact

`impact ≈ magnitude of waste × heat of path`.

- **Heat** is the entry's measured share: its median fraction of its probe's
  Postgres execution time, or of its probe's Go CPU samples, across the runs. It is
  a share of that source's total in that probe, not of the probe's wall time, and
  the report says which source it is.
- **Magnitude** is the entry's median raw `value` in its own unit, the time that
  source spent on the key per run, read against the probe's metric: the median of
  its latency or duration, with its spread across the runs. Quote the share, the
  value and the metric side by side. Do not turn them into a saving off the metric:
  the value is time spent in one source, and the metric is wall time that the source
  only partly fills.
- **Measured** means it came from the attribution. A candidate whose share was read
  off the ranking is measured. A candidate found by reading code next to a hot
  entry, whose own key is not in the ranking, is not: its heat is inferred from its
  neighbour, and it says so.

**High impact** is an entry carrying a leading part of its probe's source, on a
probe whose metric a user waits on or a job's duration depends on. As a guide, a
share of 0.20 or more is high and one under 0.05 is low; between them it is
judgment, and the report says it was. **Low impact** is a small share, or any share
on a probe whose metric is already far below anything a user would feel.

## Axis 2 — Complexity cost

What the *faster* version adds, honestly counted:

- **New failure modes**, the expensive kind: cache staleness and invalidation, race
  conditions, partial-batch failures, retry storms, memory growth. One new failure
  mode on a money path usually outweighs 100 ms.
- **New concepts**: layers, indirection, configuration knobs, a dependency, a
  background process. Each is something the next reader must load before
  understanding the code.
- **Testing burden**: whether proving it correct gets harder, as with
  time-dependent or concurrency tests.

**None or negligible** is the same or fewer concepts, using a pattern the codebase
already uses: `errgroup`, one query with `= ANY($1)` instead of one per item, an
index. **Real** is anything with a new failure mode or a new moving part. When
unsure, price it as real: complexity underpriced is how codebases rot.

The price is always judgment, read off the code by the agent, and the report labels
it that way.

## The quadrants

| Quadrant | Definition | Examples |
|---|---|---|
| 🟢 **Free win**: high impact, no added complexity | The fix is as simple as the current code, or simpler | a query per item in a loop → one query over the set; the missing index on a hot predicate; a client or a prepared statement built per call, hoisted out; two independent reads made sequentially, run together with `errgroup`; a missing `LIMIT`; a value recomputed per row that is constant per request |
| 🟠 **Trade-off**: high impact, real complexity | Worth it only at the right target, with the heat measured and the price said out loud | a TTL cache on a hot read (staleness, invalidation); a denormalised column (dual writes); a materialised aggregate (refresh policy); a batch endpoint on a sibling service (new surface) |
| 🟡 **Micro**: low impact, no complexity | Fine to do, never worth reporting loudly | `strings.Builder` in a warm loop; preallocating a slice; trimming a column from a small result |
| 🔴 **Over-engineering**: low impact, complexity present | The inverse finding: the fix is deletion or simplification | a hand-rolled cache in front of a query with a negligible share; a worker pool for a handful of items; `sync.Pool` on a cold path; tuning knobs nobody sets |
| ⚪ **Needs measurement** | The Iron Law is not met | heat or magnitude could not be established: a key with no code in the export, a candidate whose own share is not in the ranking, a probe whose metric did not come back |

Boundary rules:

- A 🟢 whose implementation quietly grows complexity, like the concurrent fan-out
  that needs a semaphore, an error policy and a retry, is an 🟠. Reclassify, do not
  squint.
- An 🟠 without measured heat is not a finding yet. It is ⚪ until the heat is
  established.
- A 🔴 is only a finding when the complexity is performance-motivated. Complexity
  that exists for correctness or product reasons is out of scope.

## Depth and target

This skill always runs at the plugin's deepest depth, `deep`: impact is backed by a
local measurement, the one `measure` takes. It never runs the plugin's
`quick-scan` or `standard`, which price impact by arithmetic alone, and it suggests
no production monitoring.

The **target** is how much complexity the user is buying, per flow:

| Target | Meaning | Quadrants in play |
|---|---|---|
| `baseline` | No obvious waste; zero complexity budget for speed | 🟢 fixes and 🔴 simplifications; 🟠 only reported |
| `interactive` | A user is waiting, and there is a p95 to respect | 🟢, 🔴 and small 🟠 with a well-understood price |
| `critical-path` | A measured bottleneck on a business-critical flow | every quadrant negotiable, each 🟠 with its measurement and its price stated |

The target never changes what is reported, only what would be fixed, and this skill
fixes nothing. So no target is asked for: each 🟠 names the target that would
justify it.

## Over-engineering — the 🔴 guide

The signals, in code:

- **A cache with no measured miss cost**: a hand-rolled TTL map, a cache layer in
  front of a query whose share is negligible.
- **Concurrency machinery below its break-even**: worker pools or goroutine fan-out
  where n is small or the path cold.
- **Premature data-structure cleverness**: `sync.Pool`, custom LRUs, bitsets,
  zero-allocation tricks no benchmark ever justified.
- **Knobs nobody turns**: `BATCH_SIZE` or `POOL_SIZE` style settings with the same
  value everywhere, guarding paths tests do not cover.
- **Readability traded for microseconds** on a path whose share is negligible.

Before placing one:

1. **Confirm the path is cold** with the same bar as hot: its share in the ranking,
   or its absence from a ranking that covers the probe, which is weaker and is said.
2. **Confirm the complexity is performance-motivated**: read names and comments on
   the block. A cache that exists for rate-limit compliance, or a pool that exists
   for correctness, is not a 🔴.
3. **State the deletion test**: "replace it with the naive version, same behaviour,
   nobody notices at this share". If it cannot be stated, the candidate is ⚪.

## Priority

| Group | Condition |
|---|---|
| 🟢 **Free win** | Measured hot entry, high impact, a fix that adds no complexity. The report's headline. |
| 🟠 **Trade-off** | Measured hot entry, high impact, a fix with a real price. |
| 🟡 **Micro** | Real but small, no complexity. |
| 🔴 **Over-engineering** | Cold by measurement, performance-motivated complexity with no payoff. The fix is deletion. |
| ⚪ **Needs measurement** | The Iron Law is not met. Name the missing fact; never inflate to 🟢 to look useful. |

Priority follows **measured impact**, not cleverness. A report whose top item is a
🟡 has failed. A report that is only "checked, no waste found" is a fine result.
