---
name: hotpaths
description: "Rank where a service spends its time and price what each hot path would cost to speed up, from a measurement rather than from reading code. Takes a local repository and one ref, any commit-ish; for uncommitted work it asks for the sha `git stash create` prints. It runs `measure` with base and head both set to that ref, so the spread between two identical sides is the measured noise, reads the attribution ranking the recipe left, Postgres time per query fingerprint and Go CPU per function, opens the code of the top entries at that ref and reports in chat: the hot set per probe with each entry's measured share, then candidates grouped as 🟢 free win, 🟠 trade-off, 🟡 micro, 🔴 over-engineering and ⚪ needs measurement, each with a `path:line` that exists at the ref, its impact from the measured share and the probe's metric, a complexity price stated as judgment, and whether its impact was measured. A run is slow: ten recipe runs by default, each with containers, a seed and a load. Needs a repository whose recipe writes attribution. Read-only over the repository: it fixes nothing, writes no file there and nothing in docs/performance. Use when somebody wants to know where optimising pays before touching code. Triggers on: 'dónde conviene optimizar', 'cuáles son los hot paths', 'qué optimizo primero', 'dónde se va el tiempo de este servicio', 'qué vale la pena optimizar acá', 'rankeá los hot paths', 'where should I optimise', 'where should I optimize', 'rank the hot paths', 'what are the hot paths', 'what should I optimise first', 'where does this service spend its time'. It reviews no change, which is `review`'s job, it compares no base against a head, which is `measure` used directly, and it never edits, never commits and never invokes `plan`, `work` or `debug`."
---

# hotpaths

Read [the host runtime](references/runtime.md) first for tool invocation and
delegation. Its memory preflight does not apply here: this skill runs no `memory`
command.

Say where a service spends its time, and what each place would cost to make
faster. Reading code tells you where waste *could* be; it does not tell you which
of it the service actually pays for. This skill takes that from a measurement: the
service's own recipe, run by `measure` on a local seeded stack, leaves attribution
— Postgres execution time per query fingerprint, Go CPU per function — and
`measure` ranks each probe's time by it. The skill reads the code behind the top of
that ranking and places each candidate on the impact × complexity rubric in
[the sweet spot](references/sweet-spot.md), which you read before step 7.

It evaluates the code as it is at one ref. It is not a review of a change.

## The Iron Law

```
NO CANDIDATE WITHOUT A MEASURED SHARE + AN IMPACT × COMPLEXITY PRICE TAG
```

Adapted from
`/srv/projects/trii/core/ai-marketplace/plugins/performance/skills/review/SKILL.md:14-20`.
Every candidate rests on where the probe measurably spends its time and on what the
faster version would add. A candidate whose heat or magnitude could not be
established is ⚪, never inflated to 🟢 to look useful.

## Hard rules

1. **Read-only over the measured repository.** Nothing reaches its working tree,
   its `.git` or its memory log. The git commands below are all this skill runs
   there, and each only reads. `measure` exports with `git archive` and writes
   only outside the repository. No `git stash create` from here either: it writes
   an object into `.git`, so the user runs it, not you.
2. **No `memory` command, against any repository.** Any `memory` read against a
   target creates `<target>/.ai-kit/` while it answers.
3. **It fixes nothing.** No edit, no patch, no benchmark written, no
   `docs/performance` record. The report is the whole output, and the decision of
   what to optimise is the person's.
4. **The report lives in the conversation.** No file, no record, no copy kept.
5. **Impact is "measured" only when it came from the attribution.** A share read
   off `measure`'s ranking is measured. Anything else — a caller inferred from a
   hot library function, a loop bound read off the code — says `Medido: no` and
   where it came from.
6. **The complexity price is judgment and the report says so**, on every
   candidate.
7. **Every `path:line` exists at the measured ref.** It is read back from the
   export in step 8; one that does not resolve is dropped and counted.
8. **The recipe's stack is the only one it touches.** `measure` gives the recipe
   only `PATH`, `HOME`, `USER` and `TMPDIR`, and the recipe starts its own
   containers. Do not set a DSN, a host or a URL for it, and do not run this over a
   recipe that reaches anything it did not start.
9. **Conversation in the user's language; everything written to disk in English.**

## Commands

Every value crosses from one command to the next by being read and pasted. No
block here captures one command's output for another, and none assigns a variable.

`<target-repo>` is the path step 1 prints, substituted literally and left unquoted.
`<work-dir>` is the path step 4 prints. Every other placeholder is quoted.

## Steps

### 1 — Settle the repository

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
```

**Empty output stops the skill.**

> No puedo resolver el repo desde `<the path>`: o no existe, o no está dentro de un
> repositorio git. Decime la ruta absoluta y sigo desde ahí.

### 2 — Resolve the ref

The ref is what the user named: a branch, a tag, a sha. Nothing named means `HEAD`.

```bash
git -C <target-repo> rev-parse --verify --quiet "<ref>^{commit}"
```

It prints the commit, `<sha>` from here on. **Empty output stops the run.**

> No existe `<ref>` en `<target-repo>`. No hago fetch. Decime otro ref.

**Uncommitted work is measured through a loose commit the user makes.** `measure`
exports refs with `git archive`, so a change in the working tree is invisible to it.
When the user wants what is on disk rather than what is committed, stop and say:

> `measure` lee commits, no el working tree. Corré esto en `<target-repo>` y pasame
> el sha que imprime:
>
> `git stash create`
>
> Crea un commit suelto con tus cambios sin tocar ramas, índice ni archivos. Solo
> lleva archivos que git ya sigue: uno nuevo tiene que estar `git add`-eado antes,
> sin commitearlo. Si no imprime nada, no hay cambios y mido `HEAD`.

### 3 — Check `measure` and say what the run costs

```bash
command -v measure && measure --help
```

**Nothing printed stops the run**: `measure` is installed by `make install` from the
ai-kit clone. Read the help: it documents the attribution field this skill reads in
step 6.

A run is slow. By default `measure` runs the recipe five times per side, so ten
runs, each one starting containers, seeding them, running every probe and the load
generator, and tearing them down. The user may ask for fewer or more with `--runs`;
pass what they said through, and nothing else. A recipe directory other than the
default `bench` goes through as `--recipe` only when the user names one.

Say in one line what is about to run, then go on. It is a readback, not a question:

> Mido `<ref>` (`<sha corta>`) en `<target-repo>`: base y head son el mismo commit,
> `<N>` corridas por lado, `<2N>` corridas de la receta con contenedores, seed y
> carga. Tarda; aviso cuando termine.

### 4 — Make a temporary directory

The temporary directory is the only place this skill writes.

```bash
mktemp -d
```

If the path it printed sits under `<target-repo>`, stop: it has to be outside the
measured repository.

### 5 — Run `measure`

```bash
measure --repo <target-repo> --base "<sha>" --head "<sha>" --runs "<N>" --json > "<work-dir>/measure.json" 2> "<work-dir>/measure.log"; echo "measure exit $?"
```

Base and head are the same commit on purpose. `measure` has no single-ref mode, and
two identical sides are what make the spread between them the run-to-run noise of
this host, which the report states.

The run outlasts a foreground call's timeout. Run it in the background where the
host offers that, and wait for it to finish rather than polling. Where it does not,
give the call the longest timeout the host allows, and when that is shorter than the
run, say so and stop rather than read a half-written file.

**A non-zero exit stops the run.** `measure` printed no report: read the last lines
of `<work-dir>/measure.log`, repeat its error, go to step 9's cleanup and stop.

> `measure` no terminó: `<su error>`. No hay ranking que leer.

### 6 — Read the report

Read `<work-dir>/measure.json` whole. These fields are `measure`'s report as it
stands:

- `base.commit`, `head.commit`, `runs`, and `results`, the directory holding each
  run's `stdout.log`, `stderr.log` and `out`.
- `failed.base`, `failed.head` and `failed_runs`, each failed run with its
  `stderr_tail`.
- `metrics`: per `probe` and `metric`, its `unit`, its `better`, and for each side
  its `median`, `min`, `max` and `samples`.

The attribution ranking is the field this skill exists for. Read it from the JSON
and never from `measure`'s text output, which with base equal to head prints every
ranking twice and cuts each at fifteen keys:

- `attribution`: a list with one entry per probe, source and unit, each holding
  `probe`, `source` (`postgres` or `go-cpu`), `unit`, `base` and `head`.
- Each side is `null` when no completed run of it wrote attribution for that group,
  and otherwise `{runs, entries}`: `runs` is how many of the side's runs wrote it,
  and `entries` is the full ranking, sorted by median share descending, then by key.
- Each entry is `{key, share, value}`. `key` is a Postgres query fingerprint or a
  fully qualified Go function. `share` and `value` each carry `samples`, `median`,
  `min` and `max` across the side's runs.
- `share` is a fraction from 0 to 1: the key's value over the sum of every key's
  value in that probe, source and unit, in one run. It is a share of that source's
  total in that probe, not of the probe's wall time.
- `value` is the key's raw amount in `unit`, per run, with 0 in a run that did not
  report the key. It states the impact in the probe's own unit.
- `ignored_attribution_lines`: `{base, head}`, the lines of `attribution.jsonl`
  that were not attribution. It is present only when some run wrote the file.

`attribution` is left out of the JSON when no completed run wrote any.

**An absent or empty `attribution` stops the candidates.** The recipe left no
attribution. Report the probes and their noise from `metrics`, say that, and go to
step 9.

> La receta no dejó atribución: `measure` no tiene ranking para ningún probe. Sin
> ranking no hay hot set medido, así que no propongo candidatos.

Failed runs stop nothing: a probe that came back from the other runs is still
measured, and the report counts the failures and names `results`.

### 7 — Pick the hot set

Read [the sweet spot](references/sweet-spot.md) now.

Take the `head` ranking of each `attribution` entry, or its `base` ranking when
`head` is `null`. **Its hot set is the entries in rank order until their median
shares add up to 0.80, at most five.** Next to each, read its share's `min..max` and
its median share in the other side's ranking: the two sides ran the same code, so
the gap between them is the noise on that share.

**Pair each attribution probe with its metrics by rule, not by an exact name.** A
recipe can name a probe one way in `attribution.jsonl` and group its metrics under
another: in be-profits-stocks the attribution probe `positions/stocks` has its
metrics under probe `k6`, named `positions/stocks p50`, `positions/stocks p95` and
so on. In this order:

1. The `metrics` whose `probe` equals the attribution probe.
2. When there are none, the `metrics` whose `metric` starts with the attribution
   probe followed by whitespace or a colon. `positions/stocks p95` pairs with
   `positions/stocks`; it does not pair with `positions`.
3. When neither finds one, no metric is paired. The probe's impact is stated from
   its share and its raw `value` alone, and the report says no metric was paired.

From the paired metrics, take the one whose unit is a time and whose `better` is
`lower`: a p95 where there is one, otherwise the median duration. A paired set with
no time metric counts as no metric paired. When the set holds several time metrics,
like a benchmark probe with one `ns/op` per benchmark, take the one the entry's code
is exercised by when the code you read in step 8 says which, and otherwise the
largest. **The report names the metric it paired**, by its `probe` and `metric`, on
the probe's line.

### 8 — Open the code at the ref

```bash
mkdir "<work-dir>/code" &&
git -c filter.lfs.smudge= -c filter.lfs.process= -c filter.lfs.required=false -C <target-repo> archive --format=tar --output "<work-dir>/code.tar" "<sha>" &&
tar -xf "<work-dir>/code.tar" -C "<work-dir>/code" &&
rm "<work-dir>/code.tar"
```

`git archive` gives the code at the ref without touching `.git`, and the
`filter.lfs` settings keep an LFS smudge from downloading into it. **A failure here
goes to step 9's cleanup and stops**, repeating the error.

Locate each hot entry in `<work-dir>/code`:

- **A Postgres fingerprint** is query text with its constants replaced by `$1`,
  `$2`. Pick its most distinctive fragment — the table and the clause after it,
  never a placeholder — and grep for it. Queries live in Go strings, in `.sql` files
  or in generated code, and whitespace may differ from the fingerprint, so shorten
  the fragment until it hits.

  ```bash
  grep -rnF "<fragment>" "<work-dir>/code" --include=*.go --include=*.sql
  ```

  The query's line is the anchor. Then find who runs it, by grepping the name of
  the function or the generated method that holds it, because the waste is usually
  in the caller: a query run once per item of a loop.

- **A Go function** is `<import path>.<Func>` or `<import path>.(*<Type>).<Method>`.
  Read the module path off `<work-dir>/code/go.mod`; what follows it is the package
  directory.

  ```bash
  grep -rn "^func \(([^)]*<Type>) \)\?<Func>(" "<work-dir>/code/<package dir>"
  ```

  For a plain function `<Type>` is left empty. A key ending in `.func1`, `.func2`
  is a closure: locate the function that encloses it. Type parameters in brackets
  are dropped from the name before grepping.

  A key outside the module — the runtime, the standard library, a dependency — has
  no code in the export. It stays in the hot set with its share. A candidate for it
  is the module code that calls it, found by reading, and that candidate's impact is
  `Medido: no`: the measured share belongs to the library function, and which caller
  pays it is inferred.

Read each hot entry's code and what calls it, enough to say what the time is spent
on. Every candidate gets a quadrant from the rubric, a `path:line`, its impact, its
price and whether it was measured. A hot entry with no waste in it goes under
`Revisado, sin desperdicio` with what was checked.

**Every anchor is read back before it is reported**, one line per candidate in one
block:

```bash
awk 'NR==<line>{print FILENAME":"FNR": "$0}' "<work-dir>/code/<path>"
```

`awk` printing nothing, or a line with nothing on it, means the anchor does not
exist at the ref. That candidate is dropped and the report says how many were.

### 9 — Clean up

```bash
rm -rf "<work-dir>"
```

This runs on every path that reached step 4, before the report goes out. If it
fails, say so and name the directory. `measure`'s own `results` directory is
outside the repository and is left where it is: the report names it.

### 10 — Print the report

In the user's language, and then stop. The template is the shape:

```
Hot paths de `<ref>` (`<sha corta>`), <N> corridas por lado. Fallaron: base <n>/<N>, head <n>/<N>.
Base y head son el mismo commit: la diferencia entre los dos lados es el ruido medido en este host, no un cambio.
Resultados de measure: <results>

<probe> — métrica emparejada `<probe de la métrica>` `<métrica>`: mediana <v> <unidad>; ruido: base <min>..<max>, head <min>..<max>
<probe> — sin métrica emparejada: el impacto sale solo de la participación y del valor crudo
  Postgres (participación en el tiempo de ejecución Postgres del probe, <runs> corridas):
  1. <share> (<min>..<max>; base <share base>) <value mediano> <unit> <fingerprint acortado>
  Go CPU (participación en la CPU Go del probe, <runs> corridas):
  1. <share> (<min>..<max>; base <share base>) <value mediano> <unit> <función>

🟢 Free win
1. <título>
   `<path>:<line>` — <el desperdicio, en términos de código>
   Impacto: <share> del tiempo <Postgres | CPU Go> de <probe>, <value mediano> <unit> por corrida; `<métrica emparejada>` mediana <v> <unidad>, o "sin métrica emparejada"
   Precio (juicio): <lo que agrega la versión más rápida, o "ninguno: <por qué>">
   Medido: sí, atribución de measure | no, <de dónde sale>

🟠 Trade-off
1. <título>
   <la misma forma>
   Se justifica en: <interactive | critical-path>

🟡 Micro
- `<path>:<line>` <una línea>

🔴 Over-engineering
1. <título>
   <la misma forma>
   Prueba de borrado: <la versión ingenua, y por qué nadie lo notaría>

⚪ Necesita medición
- <candidato> — <el dato que falta> — <qué lo mediría>

Revisado, sin desperdicio
- <entrada del hot set> — <qué se revisó>

Descarté <n> candidatos con un `path:line` que no existe en este ref.
```

- The probe's line is one of the two shapes above: the metric it paired in step 7,
  named by its `probe` and `metric`, or the line saying none was paired. The noise
  is that metric's spread on both sides: what two runs of the same code differ by,
  stated and not used to grade anything.
- Shares print as percentages of the JSON's fractions. The source is named on every
  impact line, because a share of Postgres time and a share of Go CPU are different
  things.
- The impact line quotes the share, the key's median `value` in its own `unit` and
  the probe's metric side by side, and never turns them into milliseconds saved off
  the metric: the value is time spent in one source, which the probe's wall time
  only partly overlaps.
- A side that is `null` prints `sin atribución` in place of its share.
- Every group heading appears, with `ninguno` under it when it is empty, so the
  reader sees each was considered.
- A `metrics` probe that no attribution probe paired with still gets one line per
  time metric, followed by `sin atribución`.
- The dropped-anchor line appears only when it is not zero.
- **No candidate is a real report.** A hot set where every entry is under
  `Revisado, sin desperdicio` says where the time goes and that nothing in it is
  waste.

Add nothing after it: no fix, no patch, no offer to apply anything.

## Key principles

- **The measurement picks the code, not the other way round.** Only what the
  ranking puts at the top is read.
- **Measured and inferred are never blended.** The `Medido` line is what keeps a
  guess from reading like a number.
- **Both assets are priced.** A faster version that adds a failure mode is not free,
  and complexity on a path with a negligible share is a finding too.
- **The noise is stated, not hidden.** Two identical sides that disagree say how much
  of any share can be trusted.
- **The measured repository is read and never touched**, and that includes the
  index, the refs and the log.
