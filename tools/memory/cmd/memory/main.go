// Command memory is the append-only knowledge log behind the ai-kit workflow.
//
// It knows nothing about skills, plans, phases or work loops. It stores typed
// records and the edges between them, validates them, and answers queries
// without flooding the caller's context. Everything about *how* the workflow
// uses it lives in the skills, not here — that boundary is what lets this tool
// be extracted to its own repository later without rewriting anything.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/alanv/ai-kit/tools/memory/cmd/internal/gitcmd"
	"github.com/alanv/ai-kit/tools/memory/internal/memory"
)

const usage = `memory — append-only knowledge log

Usage:
  memory init      [--scope repo|user]
  memory add       --kind KIND --title TEXT --about TAG[,TAG...] [--plan SLUG] [--body TEXT] [--slug SLUG] [--anchor PATH:LINE] [--repo-root PATH]
  memory add       --batch PATH|- [--plan SLUG] [--repo-root PATH]
  memory link      --type TYPE --from ID --to ID
  memory query     [--plan SLUG] [--about TAGS] [--kind KINDS] [--slug SLUG] [--text S] [--id IDS] [--since TS] [--until TS] [--current] [--count] [--fields KEYS] [--body-lines N] [--limit N]
  memory search    TEXT [--plan SLUG] [--current] [--limit N]
  memory areas     [--kind KINDS] [--plan SLUG]
  memory anchors   [--plan SLUG] [--about TAGS] [--kind KINDS] [--slug SLUG] [--id IDS] [--verdict V] [--path S] [--current] [--all] [--repo-root PATH]
  memory edges     [--type TYPES] [--from ID] [--to ID] [--since TS] [--until TS] [--count] [--limit N]
  memory state     [--id ID] [--text] [--here] [--all]
  memory promote   [--ids ID[,ID...]] [--kind KIND[,KIND...]] --to user [--dry-run]
  memory validate  [--all]
  memory kinds

Common flags:
  --scope repo|user        which log to act on (default: repo). 'query' alone also
                           takes 'both'. 'areas' takes no --scope: it answers for
                           both logs, reported apart.
  --plan SLUG              which plan a record belongs to: stamped by 'add',
                           bounds 'query' and 'search'. Defaults to AI_KIT_PLAN.
                           Other commands accept it and read the whole log, bar
                           'edges', which refuses it: an edge carries no plan.
  --dir PATH               start directory for repo discovery (default: cwd)
  --store PATH             the store every log lives in (default:
                           /srv/projects/.ai-kit/memory.db). Every command takes
                           it, after the subcommand name like any other flag.

There is one log per repo, at <repo>/.ai-kit/memory.jsonl. The plan a record
belongs to lives in the record, as origin.plan, so any two records can be linked
whatever plan produced them.

A record written with no --plan belongs to the repository rather than to any plan
in it, and every plan-bounded read returns it — that is what 'preference' is for.
A record that does name a plan stays out of a query naming a different one.

The first command that resolves a location creates it, read or write, and writes
a .gitignore containing '*' inside it — so nothing the loop leaves behind reaches
a commit, and no repository's own .gitignore is ever edited. A read that cannot
create it answers anyway; a write refuses with code 'init_failed'.

'add --batch' takes one JSON object per line — '-' reads stdin — carrying the
fields the flags carry (kind, title, body, slug, about, anchor, plan) and never
the ones the tool stamps. A line may declare an 'edges' array, each edge naming
exactly one endpoint by an id already in the store: the entry on that line is the
other end, on the side the line left out, and the id may belong to another
repository's log exactly as 'link' allows. Every entry in the file is appended in
one write, or nothing is — one bad line refuses all of it.

An argument in flag position that begins with '-' and contains a space is refused
before any flag is parsed, whatever the subcommand. It is one argument where the
command line meant two, and an unquoted variable that failed to split is what
produces it. A flag's *value* is never inspected, so a body may say anything.

'query --id ID[,ID...]' answers with those records and nothing else, and names
under 'not_found' every id this query did not match — one no record in the log
carries, and one another bound excluded — an id that matches nothing is an
answer, not a refusal, so the exit code is still 0. An '--id' that names no id
at all is the one refusal on this bound: a flag that parsed to nothing is an
unset variable rather than a query for the whole log. '--since' and
'--until' bound a query by the record's 'at' stamp and both bounds are inclusive:
a record stamped exactly at either one is returned. The stamp is compared as
written rather than parsed as a date, so a bare '--until 2026-08-01' stops just
before that day; pass a full timestamp to bound one. '--count' reports how many
records match and returns none of them, which is the same number the same query
without it reports.

'query --fields KEY[,KEY...]' returns those keys of each matched record and no
others — any subset of version, id, kind, slug, scope, at, about, title, body,
anchors, origin, with nothing forced in — and refuses a name outside them rather
than dropping it. '--body-lines N' returns the first N lines of each body; 0
returns no body at all and -1, the default, returns the whole of it. Both are on
'query' alone, both change the shape of an answer rather than which records are
in it, and neither is a way past the count-before-dump guard.

'query --scope both' reads this repository's log and the user log in one
invocation and answers with one 'records' list, each record carrying the 'scope'
it was written under. Each log is filtered on its own; 'count', '--count',
'--limit', the guard and 'not_found' are computed over the two together, and
'current_objective' is the repository log's. The answer names both logs under
'paths', repository first, and has no 'path'. A record promoted into the user log
comes back twice, once from each. A missing user log adds nothing, and outside a
git repository 'both' refuses as 'repo' does. No other command takes 'both', and
it is not a scope a record can carry.

'edges' lists the relations themselves, and it is the one read that needs no id
to start from: every edge in the log, or the ones narrowed by '--type', by either
endpoint, or by time. It filters on what an edge carries and on nothing else — an
edge has no plan, so '--plan' is refused here rather than accepted and ignored,
and the kind or the areas of the records at either end are not bounds it takes.
Naming '--from' or '--to' reads every log of the store, since a record is where
it is and an edge drawn next door still names it; naming neither reads this log.
It carries the same count-before-dump guard 'query' carries: above the threshold
it returns no edges at all, says so with 'truncated', and names in the hint the
flags that get past it — a narrower '--type', endpoint or time bound, '--limit N'
for the most recent, '--count' for the number alone.

'state' reads one log or every log, and its default follows the output mode:
'state' lists this repository's objectives as JSON, 'state --text' prints what is
waiting on you across every log in the store. Which objectives are listed is one
log's answer; what is read off them — parentage, state, the current objective —
comes from every edge in the store, so a parent or a child in another repository
is named as one, and an anomaly is reported by the log holding the record it is
about. '--here' narrows the text to this repository and '--all' widens the JSON
to every log the store holds. No mode of 'state' creates anything and none walks
the filesystem: the logs are the ones the store already carries, so one whose
repository has since been moved or deleted is reported like any other. An
objective two logs both carry at one scope is reported once, from the first of
them.

Every command prints JSON on stdout, on success and on failure alike, except
'state --text' which prints the summary. A failure is JSON in every mode,
'--text' included. Exit 0 means the command ran; exit 1 means it refused, and
the JSON says why. 'validate' is the one command where exit 1 is a verdict
rather than a refusal: it ran, and either a log failed validation (ok: false) or
a location it was asked to sweep could not be created (init_failed). Those two
are reported separately, because a directory nobody can write to says nothing
about whether the records in it are well-formed.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "link":
		err = cmdLink(os.Args[2:])
	case "query":
		err = cmdQuery(os.Args[2:])
	case "search":
		err = cmdSearch(os.Args[2:])
	case "areas":
		err = cmdAreas(os.Args[2:])
	case "anchors":
		err = cmdAnchors(os.Args[2:])
	case "edges":
		err = cmdEdges(os.Args[2:])
	case "state":
		err = cmdState(os.Args[2:])
	case "promote":
		err = cmdPromote(os.Args[2:])
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "kinds":
		err = cmdKinds(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		emitError(err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------- output ---

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

type cliError struct {
	Code string
	Msg  string
}

func (e *cliError) Error() string { return e.Msg }

func fail(code, format string, args ...any) error {
	return &cliError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

func emitError(err error) {
	code := "error"
	var ce *cliError
	if errors.As(err, &ce) {
		code = ce.Code
	}
	emit(map[string]string{"error": err.Error(), "code": code})
}

// ----------------------------------------------------------- shared flags ---

type common struct {
	scope string
	plan  string
	dir   string
}

// --plan is bound on every command, not only the three that use it. It is a
// filter on 'query' and 'search' and a stamp on 'add'; elsewhere it is accepted
// and ignored — bar 'edges', which refuses it, since an edge carries no plan at
// all — because a caller that has a plan slug in hand will pass it, and refusing
// the flag would break every invocation that names one out of habit.
func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.scope, "scope", "repo", "repo|user")
	fs.StringVar(&c.plan, "plan", "", "plan slug: stamped by add, bounds query and search")
	fs.StringVar(&c.dir, "dir", "", "start directory for repo discovery")
}

// parseFlags is fs.Parse plus one refusal and the store, and every subcommand
// calls it instead of calling fs.Parse itself.
//
// The refusal is for an argument in flag position that begins with `-` and
// contains a space. That is one argument where the command line meant two, and
// what produces it is an unquoted variable that failed to split — a shell handed
// `--kind decision` as a single word, which Go's flag package reports as an
// undefined flag named `kind decision`. The tool then looks like it has invented
// a flag, and the write that was refused looks like the tool's fault rather than
// the shell's.
//
// **In flag position, and that is why this cannot be a scan over os.Args.**
// `--body "--about persistence is a legitimate body"` is a body this repository's
// own log contains, and a bare scan would refuse it. Telling a flag from a flag's
// value needs to know which flags take a value, which is knowledge only the
// FlagSet has — so the guard sits here, where the FlagSet is, and covers every
// subcommand by being the one thing they all call.
//
// There is no escape hatch. A caller who genuinely means an argument of that
// shape is passing it as a flag's value, which this never looks at.
func parseFlags(fs *flag.FlagSet, args []string) error {
	store := fs.String("store", memory.DefaultStorePath, "the store every log lives in")
	for i := 0; i < len(args); i++ {
		a := args[i]
		// Where the flag package stops parsing, this stops looking: `--` ends the
		// flags, and so does the first argument that is not one — `search`'s text,
		// for instance, which may say anything.
		if a == "--" {
			break
		}
		if len(a) < 2 || a[0] != '-' {
			break
		}
		if strings.ContainsAny(a, " \t") {
			return fail("unsplit_argument",
				"argument %q is in flag position, begins with - and contains a space: it is one argument "+
					"where the command line meant two. Did an unquoted variable fail to split? Pass the flag "+
					"and its value as separate arguments.", a)
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.Index(name, "="); eq >= 0 {
			continue // --flag=value carries its value inline, so the next argument is a flag again
		}
		f := fs.Lookup(name)
		if f == nil {
			continue // an unknown flag: fs.Parse refuses it, and guessing whether it takes a value would be inventing one
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		i++ // the next argument is this flag's value, and a value is never inspected
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := memory.UseStore(*store); err != nil {
		return fail("bad_store", "%s", err.Error())
	}
	return nil
}

// locate resolves a location without creating it. Nearly everything goes through
// resolve or resolveForWrite instead; the two commands that call this directly,
// `init` and `validate`, each have something of their own to say about the
// creation.
func (c *common) locate() (memory.Location, error) {
	loc, err := memory.Resolve(memory.Scope(c.scope), c.dir)
	if err != nil {
		return loc, fail("bad_scope", "%s", err.Error())
	}
	return loc, nil
}

// Resolving a location also creates it, on a read as much as on a write.
//
// A repository that has never run this loop is the case that has to work on
// first contact, and a read used to leave nothing behind: `query` reported
// `count: 0` against a path that did not exist, so the .gitignore that keeps the
// directory out of git only arrived with the first record. Whatever command
// touches the repository first is the one that gets to make it ignorable.
//
// **Creating it is best-effort here, and that asymmetry with resolveForWrite is
// deliberate.** The accepted cost of initializing on a read was a stray directory
// in somebody else's repository — not a read that refuses to answer. A repository
// checked out read-only, or an .ai-kit/ somebody chmod-ed, still answers `query`
// with whatever its log holds, exactly as it did before initialization moved onto
// the read path; turning that into exit 1 would be this change breaking the
// command it was meant to make work on first contact.
//
// A scope that cannot be resolved creates nothing — the refusal comes first.
func (c *common) resolve() (memory.Location, error) {
	loc, err := c.locate()
	if err != nil {
		return loc, err
	}
	_ = materialize(loc) // discarded on purpose: see above
	return loc, nil
}

// resolveForWrite is resolve for a command that is about to append, where a
// location that cannot be created is a refusal rather than a shrug: the write
// would fail on the next line anyway, and `init_failed` names which of the two
// went wrong instead of leaving the caller with a dangling-edge or write error to
// interpret.
func (c *common) resolveForWrite() (memory.Location, error) {
	loc, err := c.locate()
	if err != nil {
		return loc, err
	}
	if err := materialize(loc); err != nil {
		return loc, err
	}
	return loc, nil
}

func materialize(loc memory.Location) error {
	if _, err := memory.Open(loc.Path).Init(); err != nil {
		return fail("init_failed", "%s", err.Error())
	}
	return nil
}

func (c *common) checkScopeFlags() error {
	if err := memory.ValidScope(memory.Scope(c.scope)); err != nil {
		return fail("bad_scope", "%s", err.Error())
	}
	if c.dir == "" {
		return nil
	}
	if _, err := memory.RepoRoot(c.dir); err != nil {
		return fail("bad_scope", "%s", err.Error())
	}
	return nil
}

// planFilter is the plan a read command bounds itself to, empty when none was
// named — an unbounded read over one repo's whole log, which the count-before-dump
// threshold still guards.
func (c *common) planFilter() (string, error) {
	slug, err := memory.PlanSlug(c.plan)
	if err != nil {
		return "", fail("bad_plan", "%s", err.Error())
	}
	return slug, nil
}

// planStamp is the plan a write records itself under, empty when none was named.
// A malformed slug is still a refusal, since `origin.plan` is what every bounded
// query matches on.
//
// It reads the same two sources planFilter reads and today produces the same
// answer, and it derives that answer itself rather than calling planFilter,
// because the two are separate questions about the same empty string. Absent on a
// read means "do not bound", which a later change could reasonably redefine —
// widen to a default, narrow to the current tree. Absent on a write means "this
// belongs to the repository rather than to a tree in it", which is the claim a
// `preference` exists to make, and it is stamped into an append-only log where a
// wrong answer cannot be taken back. Under the alias, moving the first would have
// silently moved the second.
func (c *common) planStamp() (string, error) {
	slug, err := memory.PlanSlug(c.plan)
	if err != nil {
		return "", fail("bad_plan", "%s", err.Error())
	}
	return slug, nil
}

func (c *common) load() (memory.Location, []memory.Line, error) {
	loc, err := c.resolve()
	if err != nil {
		return loc, nil, err
	}
	lines, err := readLog(loc)
	return loc, lines, err
}

func (c *common) loadExisting() (memory.Location, []memory.Line, error) {
	loc, err := c.locate()
	if err != nil {
		return loc, nil, err
	}
	lines, err := readLog(loc)
	return loc, lines, err
}

func readLog(loc memory.Location) ([]memory.Line, error) {
	lines, err := memory.Open(loc.Path).Read()
	if err != nil {
		return nil, fail("read_failed", "%s", err.Error())
	}
	return lines, nil
}

func writeLog(loc memory.Location) *memory.Store {
	s := memory.Open(loc.Path)
	s.Scope = loc.Scope
	s.RepoID = gitcmd.RepoIdentity(loc.Repo)
	return s
}

func logAt(l memory.Log) memory.Location {
	loc := memory.Location{Scope: l.Scope, Path: l.Path}
	if l.Scope == memory.ScopeRepo {
		loc.Repo = filepath.Dir(filepath.Dir(l.Path))
	}
	return loc
}

// ------------------------------------------------------------- subcommands ---

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	var c common
	c.bind(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	// c.locate() and not c.resolve(): resolve initializes as a side effect, which
	// would leave this command reporting created=false on the run that did the
	// creating. Here the creation is the point, so it happens where its answer can
	// be read — and a failure to create is this command's whole subject, so it is
	// never best-effort.
	loc, err := c.locate()
	if err != nil {
		return err
	}
	created, err := memory.Open(loc.Path).Init()
	if err != nil {
		return fail("init_failed", "%s", err.Error())
	}
	emit(map[string]any{"path": loc.Path, "scope": loc.Scope, "created": created})
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	var c common
	c.bind(fs)
	kind := fs.String("kind", "", "record kind (see `memory kinds`)")
	title := fs.String("title", "", "one-line title")
	body := fs.String("body", "", "free text")
	slug := fs.String("slug", "", "kebab-case label; derived from --title when omitted")
	about := fs.String("about", "", "comma-separated tags: areas, paths, topics")
	var anchors anchorSpecs
	fs.Var(&anchors, "anchor", "code this record rests on, as repo-relative path:line or path:first-last; repeat for more than one")
	anchorRoot := fs.String("repo-root", "", "repository the --anchor paths are relative to (default: the repo the log lives in)")
	unit := fs.String("unit", "", "id of the unit of work this came out of")
	author := fs.String("author", "", "who recorded it; defaults to git config user.name")
	repo := fs.String("repo", "", "repo label; defaults to the repo directory name")
	batch := fs.String("batch", "", "JSONL file of entries to append in one write, or - for stdin")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	loc, err := c.resolveForWrite()
	if err != nil {
		return err
	}
	// Before the id, so that a plan slug that is not a slug refuses the write
	// rather than reaching a log that cannot take it back. An *absent* plan is not
	// a refusal: the record then belongs to the repository. See planStamp.
	plan, err := c.planStamp()
	if err != nil {
		return err
	}

	st := stamp{anchorRoot: *anchorRoot, repo: *repo, author: *author, unit: *unit}

	if *batch != "" {
		// The flags that describe one record are refused alongside --batch rather
		// than ignored. Each of them has a per-line spelling, so a caller who passed
		// both meant one of the two to win, and silently picking the file would put a
		// title nobody sees into a log that cannot take it back.
		if named := namedFlags(fs, "kind", "title", "body", "slug", "about", "anchor"); len(named) > 0 {
			return fail("batch_and_record_flags",
				"--batch takes those fields from each line of the file, so %s cannot also be passed on the "+
					"command line: put them in the batch, or drop --batch", strings.Join(named, ", "))
		}
		return runBatch(&c, loc, *batch, plan, st)
	}

	id, err := memory.NewULID(time.Now())
	if err != nil {
		return fail("id_failed", "%s", err.Error())
	}

	s := *slug
	if s == "" {
		s = deriveSlug(*title)
	}

	// The anchors are resolved against the working tree before the record exists,
	// so a citation that names nothing refuses the whole write. The alternative is
	// an anchor nobody can check sitting in an append-only log.
	built, err := buildAnchors(st.anchorRepo(loc), anchors)
	if err != nil {
		return err
	}

	// The schema version is stamped, never passed in. The tool knows which
	// schema it writes; a caller that could choose would eventually choose
	// wrong, and a record that lies about its version is worse than one with
	// no version at all.
	e := &memory.Entry{
		Version: memory.CurrentSchemaVersion,
		ID:      id,
		Kind:    memory.Kind(*kind),
		Slug:    s,
		Scope:   memory.Scope(c.scope),
		At:      time.Now().UTC().Format(time.RFC3339),
		About:   splitList(*about),
		Title:   strings.TrimSpace(*title),
		Body:    strings.TrimSpace(*body),
		Anchors: built,
		Origin:  st.origin(loc, plan),
	}

	// Validate before writing. The log is append-only, so a bad record cannot
	// be taken back — the only place to stop it is here.
	if ps := memory.ValidateEntry(e); len(ps) > 0 {
		emit(map[string]any{"error": "record rejected", "code": "invalid_record", "problems": ps})
		os.Exit(1)
	}

	if err := writeLog(loc).Append(e); err != nil {
		return fail("write_failed", "%s", err.Error())
	}
	emit(map[string]any{"path": loc.Path, "record": e})
	return nil
}

// stamp is what a write takes from the command line about where it came from,
// as opposed to what it says. One record or two hundred, the answers are the same:
// the plan is the only part of an origin a batch line may speak for itself, so
// everything else is resolved once per run and copied. See originBase for what
// makes that a requirement rather than a tidiness.
type stamp struct {
	anchorRoot string
	repo       string
	author     string
	unit       string
}

// anchorRepo is the working tree the --anchor paths are read out of.
//
// --repo-root is what makes an anchored record writable at user scope, where the
// log resolves to a home directory and loc.Repo is empty. The requirement itself
// never bends — a `gotcha` is refused without an anchor in every scope — so what
// the flag changes is that the root can be supplied rather than only inferred.
func (s stamp) anchorRepo(loc memory.Location) string {
	return firstNonEmpty(s.anchorRoot, loc.Repo)
}

// origin is the whole origin for one record: the base, plus the plan that record
// belongs to.
func (s stamp) origin(loc memory.Location, plan string) *memory.Origin {
	o := s.originBase(loc)
	o.Plan = plan
	return &o
}

// originBase is every part of an origin except the plan, and it is built once per
// run because building it spawns two processes: `git remote get-url origin` for the
// identity and `git config user.name` for the author.
//
// **Neither is skipped by a value already in hand.** firstNonEmpty evaluates every
// argument it is given, so `--author` or AI_KIT_AUTHOR set does not stop
// gitcmd.UserName from running — the call happens while the arguments are being
// assembled, before anything chooses between them. Measured with a counting wrapper
// on PATH before this was hoisted out of the per-line path: 2 spawns for a 1-line
// batch, 20 for 10 lines, 400 for 200 lines, at which point a batch spent 2.3s
// almost entirely waiting on subprocesses. It returns a value rather than a pointer
// so that a caller copying it per line cannot alias the plan of the one before.
func (s stamp) originBase(loc memory.Location) memory.Origin {
	anchorRepo := s.anchorRepo(loc)
	return memory.Origin{
		// The repository the anchors were read out of is the repository this
		// record came from, and at user scope it is the only one there is:
		// loc.Repo is empty there, and filepath.Base("") is ".", which names
		// nothing. An anchor's path says where in a repository and never which
		// one, so a user-scope record whose origin.repo read "." would carry a
		// citation nobody could resolve.
		Repo: firstNonEmpty(s.repo, repoLabel(loc.Repo), repoLabel(anchorRepo)),
		// The label and the identity are read off the same root but answer
		// different questions, so --repo overrides the first and not the second: a
		// caller may rename what a record calls its repository, and nothing it can
		// type makes two repositories the same one.
		RepoID: gitcmd.RepoIdentity(firstNonEmpty(loc.Repo, anchorRepo)),
		Author: firstNonEmpty(s.author, os.Getenv("AI_KIT_AUTHOR"), gitcmd.UserName(loc.Repo)),
		Unit:   s.unit,
	}
}

// namedFlags returns which of the given flags the caller actually passed. Set
// reports what was set rather than what differs from a default, so a flag passed
// with its default value still counts — the question here is what the command
// line said, not what it changed.
func namedFlags(fs *flag.FlagSet, names ...string) []string {
	passed := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
	out := []string{}
	for _, n := range names {
		if passed[n] {
			out = append(out, "--"+n)
		}
	}
	return out
}

func refuseNamedScope(fs *flag.FlagSet, c *common, subject, alternative string) error {
	if len(namedFlags(fs, "scope")) == 0 {
		return nil
	}
	drop := "Drop --scope."
	if alternative != "" {
		drop = "Drop --scope, or " + alternative + "."
	}
	return fail("scope_not_on_store_wide_read", "--scope %q bounds nothing here: %s spans every log in %s, "+
		"so naming one would claim the answer came from a log that need not hold it. %s",
		c.scope, subject, memory.CurrentStore(), drop)
}

// runBatch is `add` over a JSONL file: every entry in it appended in one write,
// or nothing appended at all.
//
// The atomicity is the point and it is not new machinery: Store.Append is
// variadic and takes the advisory lock once for the whole call, so the whole
// batch is one write to the file. What this function adds is that every line is
// judged before that call happens. A batch that appended its good lines and
// reported the rest would leave a caller holding an append-only log and no way to
// tell which half landed.
//
// The edges are judged here too, against the same three store-wide checks `link`
// applies — dangling endpoints, state-edge justification, and the objective tree.
// A second write path for edges that skipped them would be a hole in all three.
func runBatch(c *common, loc memory.Location, path, plan string, st stamp) error {
	entries, problems, err := readBatch(path)
	if err != nil {
		return err
	}

	lines, err := memory.ReadStore("")
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}
	// Both views of the store, taken once: the kinds every edge check needs, and the
	// edges the tree check walks. Each batch entry joins the first as it is built
	// and each batch edge joins the second, so a batch that gives one objective two
	// parents is refused on its own second edge rather than only against the store.
	known := map[string]memory.Kind{}
	for _, e := range memory.Entries(lines) {
		known[e.ID] = e.Kind
	}
	edges := memory.Edges(lines)

	// Everything the batch will write, in the order it will be written: each entry
	// followed by the edges that entry declared.
	records := []any{}
	written := []*memory.Entry{}
	newEdges := []*memory.Edge{}
	// Where each edge came from, so a store-wide refusal can name the line to fix.
	edgeLine := map[*memory.Edge]int{}
	// And which line each stamped id belongs to, so a refusal that would otherwise
	// quote one can name the line instead. See lineNames.
	entryLine := map[string]int{}

	// The origin every entry shares, built once. See originBase: building one spawns
	// two git processes, and doing it per line is how a 200-line batch spent most of
	// its time in subprocesses.
	base := st.originBase(loc)

	for _, bl := range entries {
		e, ps := buildBatchEntry(c, base, st.anchorRepo(loc), plan, bl)
		problems = append(problems, ps...)
		if e == nil {
			continue
		}
		known[e.ID] = e.Kind
		entryLine[e.ID] = bl.Num
		records = append(records, e)
		written = append(written, e)

		for i, be := range bl.Entry.Edges {
			// A line that named both endpoints or neither was already reported by the
			// parser; there is nothing left here to build an edge out of.
			if (be.From == "") == (be.To == "") {
				continue
			}
			edge := &memory.Edge{
				Version: memory.CurrentSchemaVersion,
				Kind:    memory.KindEdge,
				Type:    be.Type,
				From:    firstNonEmpty(be.From, e.ID),
				To:      firstNonEmpty(be.To, e.ID),
				At:      time.Now().UTC().Format(time.RFC3339),
			}
			for _, p := range memory.ValidateEdge(edge) {
				p.Line = bl.Num
				p.Field = fmt.Sprintf("edges[%d].%s", i, p.Field)
				problems = append(problems, p)
			}
			records = append(records, edge)
			newEdges = append(newEdges, edge)
			edgeLine[edge] = bl.Num
		}
	}

	if len(problems) > 0 {
		emit(map[string]any{"error": "batch rejected", "code": "invalid_batch",
			"path": loc.Path, "problems": problems})
		os.Exit(1)
	}
	if len(written) == 0 {
		return fail("empty_batch", "%s holds no entries: a batch that writes nothing is a batch whose "+
			"lines went somewhere else", batchName(path))
	}

	// The three store-wide checks, in `link`'s order and with `link`'s codes, one
	// refusal at a time: each of them is a single sentence about one edge, and the
	// line number is what turns it back into the line to fix.
	//
	// Each edge is judged against the store plus the batch edges *before* it and
	// never against itself: the tree check reads the parent relation out of the
	// edges it is given, so an edge included in its own input would report that its
	// source already has the parent this edge is about to give it.
	prior := make([]*memory.Edge, 0, len(edges)+len(newEdges))
	prior = append(prior, edges...)
	for _, edge := range newEdges {
		var missing []string
		if _, ok := known[edge.From]; !ok {
			missing = append(missing, "from="+edge.From)
		}
		if _, ok := known[edge.To]; !ok {
			missing = append(missing, "to="+edge.To)
		}
		if len(missing) > 0 {
			return fail("dangling_edge", "line %d: no entry in any log of %s for %s — an edge in a batch "+
				"names one endpoint by an id already in the store",
				edgeLine[edge], memory.CurrentStore(), strings.Join(missing, ", "))
		}
		if err := memory.CheckJustification(edge.Type, known[edge.To]); err != nil {
			return fail("unjustified_state", "line %d: %s", edgeLine[edge], err.Error())
		}
		if err := memory.CheckParent(edge.Type, edge.From, edge.To, known, prior); err != nil {
			return fail("not_a_tree", "line %d: %s", edgeLine[edge], lineNames(err.Error(), entryLine))
		}
		prior = append(prior, edge)
	}

	if err := writeLog(loc).Append(records...); err != nil {
		return fail("write_failed", "%s", err.Error())
	}

	// The ids and nothing else of what was written. `add` prints the whole record
	// because there is one; twenty-five bodies printed back would spend as much of
	// the caller's context as writing them did, and the reason to declare an edge on
	// the line rather than link afterwards is precisely that nobody has to read
	// these ids back to draw one.
	report := make([]map[string]string, 0, len(written))
	for _, e := range written {
		report = append(report, map[string]string{
			"id": e.ID, "kind": string(e.Kind), "slug": e.Slug, "title": e.Title,
		})
	}
	emit(map[string]any{
		"path": loc.Path, "count": len(written), "edges": len(newEdges), "records": report,
	})
	return nil
}

// readBatch reads the JSONL from a path or, for `-`, from stdin.
//
// A path is the shape that removes the shell entirely: the file is written with
// whatever writes files and then named here, where a pipeline would have put a
// heredoc or an echo back into the command line. `-` is for the caller that
// already has the bytes.
func readBatch(path string) ([]memory.BatchLine, []memory.Problem, error) {
	if path == "-" {
		entries, ps := memory.ParseBatch(os.Stdin)
		return entries, ps, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fail("batch_unreadable", "--batch %s: %s", path, err.Error())
	}
	defer f.Close()
	entries, ps := memory.ParseBatch(f)
	return entries, ps, nil
}

func batchName(path string) string {
	if path == "-" {
		return "the batch on stdin"
	}
	return path
}

// lineNames rewrites a batch entry's own id, wherever a refusal quotes one, as the
// line that was going to carry it.
//
// The tree check states its refusal in ids, which is right for `link`: both of them
// are on disk and either can be looked up. In a batch one of them is the id this run
// stamped for an entry it is about to write — and a refusal means it never writes,
// so the id names nothing that exists and nothing that ever will. A reader who takes
// it for a record goes looking through the log for it. The line number is the only
// handle they actually have.
func lineNames(msg string, entryLine map[string]int) string {
	for id, num := range entryLine {
		msg = strings.ReplaceAll(msg, id, fmt.Sprintf("the record on line %d", num))
	}
	return msg
}

// buildBatchEntry stamps one batch line into the entry that will be written, and
// returns the problems that stop it. A nil entry means the line cannot be built
// at all, and the problems say why.
//
// It stamps id, at, version and scope exactly as the flagged path does, because
// they are the fields whose whole point is that no caller chooses them. `base` and
// `anchorRepo` arrive already resolved rather than being derived here, so that
// nothing in this function can be made to run per line that should run once.
func buildBatchEntry(c *common, base memory.Origin, anchorRepo, plan string, bl memory.BatchLine) (*memory.Entry, []memory.Problem) {
	var ps []memory.Problem
	at := func(field, format string, args ...any) {
		ps = append(ps, memory.Problem{Line: bl.Num, Field: field, Message: fmt.Sprintf(format, args...)})
	}

	// A line's own plan wins over the batch's, and an absent one is not an empty
	// one: it means this line is stamped with whatever the batch was. PlanSlug is
	// not reached for the absent case on purpose — it would fall back to
	// AI_KIT_PLAN, which the batch's own stamp has already consulted.
	linePlan := plan
	if bl.Entry.Plan != "" {
		slug, err := memory.PlanSlug(bl.Entry.Plan)
		if err != nil {
			at("plan", "%s", err.Error())
		}
		linePlan = slug
	}

	// An anchor that names nothing stops the line here rather than falling through
	// to the rest of the rules. The entry cannot be built without it, and the
	// anchored-kind rule would otherwise add "at least one anchor is required" under
	// the line that did pass one — sending the reader to add a second citation when
	// what is wrong is the first.
	built, err := buildAnchors(anchorRepo, anchorSpecs(bl.Entry.Anchor))
	if err != nil {
		at("anchor", "%s", err.Error())
		return nil, ps
	}

	id, err := memory.NewULID(time.Now())
	if err != nil {
		at("", "%s", err.Error())
		return nil, ps
	}

	slug := bl.Entry.Slug
	if slug == "" {
		slug = deriveSlug(bl.Entry.Title)
	}

	// A copy of the shared origin, with this line's plan written into it. The copy
	// is what keeps the batch's own stamp intact for the lines that follow: base is
	// taken by value, so writing Plan here cannot reach the line after this one.
	origin := base
	origin.Plan = linePlan

	e := &memory.Entry{
		Version: memory.CurrentSchemaVersion,
		ID:      id,
		Kind:    bl.Entry.Kind,
		Slug:    slug,
		Scope:   memory.Scope(c.scope),
		At:      time.Now().UTC().Format(time.RFC3339),
		About:   bl.Entry.About,
		Title:   strings.TrimSpace(bl.Entry.Title),
		Body:    strings.TrimSpace(bl.Entry.Body),
		Anchors: built,
		Origin:  &origin,
	}
	for _, p := range memory.ValidateEntry(e) {
		p.Line = bl.Num
		ps = append(ps, p)
	}
	return e, ps
}

func cmdLink(args []string) error {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	var c common
	c.bind(fs)
	typ := fs.String("type", "", "edge type (see `memory kinds`)")
	from := fs.String("from", "", "source entry id")
	to := fs.String("to", "", "target entry id")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	loc, err := c.resolveForWrite()
	if err != nil {
		return err
	}
	lines, err := memory.ReadStore("")
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}

	edge := &memory.Edge{
		Version: memory.CurrentSchemaVersion,
		Kind:    memory.KindEdge,
		Type:    memory.EdgeType(*typ),
		From:    *from,
		To:      *to,
		At:      time.Now().UTC().Format(time.RFC3339),
	}
	if ps := memory.ValidateEdge(edge); len(ps) > 0 {
		emit(map[string]any{"error": "edge rejected", "code": "invalid_edge", "problems": ps})
		os.Exit(1)
	}

	// Refuse a dangling edge at write time. A dangling edge fails silently
	// later — a graph walk just returns less than it should, with nothing
	// announcing the answer was incomplete.
	//
	// The map carries each entry's kind rather than a bare bool, because the
	// justification rule below needs the target's kind and this is the pass that
	// already reads every entry to find it.
	known := map[string]memory.Kind{}
	for _, e := range memory.Entries(lines) {
		known[e.ID] = e.Kind
	}
	var missing []string
	if _, ok := known[edge.From]; !ok {
		missing = append(missing, "from="+edge.From)
	}
	if _, ok := known[edge.To]; !ok {
		missing = append(missing, "to="+edge.To)
	}
	if len(missing) > 0 {
		return fail("dangling_edge", "no entry in any log of %s for %s",
			memory.CurrentStore(), strings.Join(missing, ", "))
	}

	// A state edge has to point at the record that justifies the transition.
	// The rule lives on the write path and nowhere else: the version rule sets
	// judge one record in isolation and cannot see the target's kind, and the
	// read sweep stays tolerant so that an edge already on disk is never
	// condemned by a requirement that arrived after it. Refusing here is the one
	// moment it can be caught, since the log is append-only.
	if err := memory.CheckJustification(edge.Type, known[edge.To]); err != nil {
		return fail("unjustified_state", "%s", err.Error())
	}

	// And the objective tree: one parent per objective, no cycles. Same place
	// and same reasoning as the rule above — a graph-wide question the version
	// rule sets cannot ask, answered where every entry is already in hand, at the
	// one moment it can still be answered at all.
	if err := memory.CheckParent(edge.Type, edge.From, edge.To, known, memory.Edges(lines)); err != nil {
		return fail("not_a_tree", "%s", err.Error())
	}

	if err := writeLog(loc).Append(edge); err != nil {
		return fail("write_failed", "%s", err.Error())
	}
	emit(map[string]any{"path": loc.Path, "edge": edge})
	return nil
}

var queryHintAdvice = []string{
	"narrow with --about/--kind/--current/--id/--since/--until",
	"pass --limit N to take the most recent",
	"ask for --count alone",
	"take less of each record with --fields id,title or --body-lines 5",
}

// scopeBoth is a value of query's --scope and never a memory.Scope. `memory kinds`
// publishes memory.Scopes() as the scopes a record can carry, and widening that
// set is a schema-version change, so it is resolved here into the two scopes that
// do exist before anything calls Resolve.
const scopeBoth = "both"

func cmdQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	var c common
	c.bind(fs)
	fs.Lookup("scope").Usage = "repo|user|both; both reads the repo log and the user log as one answer"
	about := fs.String("about", "", "comma-separated tags; a tag ending in / matches by path prefix")
	kinds := fs.String("kind", "", "comma-separated kinds")
	slug := fs.String("slug", "", "exact slug")
	text := fs.String("text", "", "case-insensitive substring over title and body")
	ids := fs.String("id", "", "comma-separated record ids; an id this query did not match is listed under not_found, whether no record carries it or another bound excluded it")
	since := fs.String("since", "", "records stamped at this RFC3339 timestamp or later; the bound itself is included, and compared as written, so a bare date starts that day")
	until := fs.String("until", "", "records stamped at this RFC3339 timestamp or earlier; the bound itself is included, and compared as written, so a bare date stops just before that day")
	current := fs.Bool("current", false, "drop entries something else supersedes")
	count := fs.Bool("count", false, "report how many match and return no records")
	fields := fs.String("fields", "", "comma-separated record fields to return — "+
		strings.Join(memory.ProjectableFields(), ", ")+" — and no others; a name outside that set is refused")
	bodyLines := fs.Int("body-lines", memory.WholeBody, "return the first N lines of each body; 0 returns no body at all, -1 (the default) returns the whole of it")
	limit := fs.Int("limit", 0, "return at most N, most recent first")
	threshold := fs.Int("threshold", 0, "override the count-before-dump threshold; -1 disables it")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	idList := splitList(*ids)
	if len(idList) == 0 && len(namedFlags(fs, "id")) > 0 {
		return fail("empty_id", "--id %q names no id: a flag that parsed to nothing is what an unset "+
			"variable looks like, and reading the whole log there would answer a question nobody asked. "+
			"Name at least one id, or drop --id.", *ids)
	}
	proj, err := memory.NewProjection(splitList(*fields), *bodyLines)
	if err != nil {
		return fail("unknown_field", "--fields: %s", err.Error())
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	f := memory.Filter{
		Plan:        plan,
		ID:          idList,
		About:       splitList(*about),
		Kinds:       toKinds(splitList(*kinds)),
		Slug:        *slug,
		Text:        *text,
		Since:       strings.TrimSpace(*since),
		Until:       strings.TrimSpace(*until),
		CurrentOnly: *current,
		CountOnly:   *count,
		HintAdvice:  queryHintAdvice,
		Limit:       *limit,
		Threshold:   *threshold,
	}
	if c.scope == scopeBoth {
		return runQueryBoth(&c, f, proj)
	}
	return runQuery(&c, f, proj)
}

var searchHintAdvice = []string{
	"narrow with --current",
	"pass --limit N to take the most recent",
}

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	var c common
	c.bind(fs)
	current := fs.Bool("current", false, "drop entries something else supersedes")
	limit := fs.Int("limit", 0, "return at most N, most recent first")
	threshold := fs.Int("threshold", 0, "override the count-before-dump threshold; -1 disables it")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	text := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(text) == "" {
		return fail("missing_text", "search needs text: memory search \"key rotation\"")
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	return runQuery(&c, memory.Filter{
		Plan: plan, Text: text, CurrentOnly: *current, HintAdvice: searchHintAdvice,
		Limit: *limit, Threshold: *threshold,
	})
}

// areaReport is one scope's enumeration. Count is how many distinct areas the
// log answers for; each row carries its own count of records.
type areaReport struct {
	Path  string        `json:"path"`
	Scope memory.Scope  `json:"scope"`
	Count int           `json:"count"`
	Areas []memory.Area `json:"areas"`
}

// cmdAreas answers what a log can be asked about, which is the one question the
// other reads cannot be asked.
//
// `--about` is freeform and nothing validates its contents, so a reader who
// guesses a tag wrong gets `count: 0` — the same answer they would get if nobody
// had ever written one. This lists the tags that exist, so those two stop being
// the same answer.
//
// It is the first command whose default reads two logs, and they are kept apart
// the way `validate --all` keeps its reports apart: each scope resolved on its
// own, each with the path it resolved to. Merging them would produce a list no
// row of which says which log it came from — and the whole point of the user log
// is that what is in it came from somewhere else. So there is no `--scope` here:
// this command has nothing to choose between.
func cmdAreas(args []string) error {
	fs := flag.NewFlagSet("areas", flag.ExitOnError)
	var c common
	// Bound by hand rather than through common.bind, which would also bind a
	// --scope this command does not honour.
	fs.StringVar(&c.plan, "plan", "", "plan slug: bounds the enumeration exactly as it bounds query")
	fs.StringVar(&c.dir, "dir", "", "start directory for repo discovery")
	kinds := fs.String("kind", "", "comma-separated kinds")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	f := memory.Filter{Plan: plan, Kinds: toKinds(splitList(*kinds))}

	stored, err := memory.ReadStore("")
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}

	reports := make([]areaReport, 0, 2)
	for _, s := range []memory.Scope{memory.ScopeRepo, memory.ScopeUser} {
		// Through load, so each scope goes through the same best-effort resolve
		// every other read uses: a machine that has never promoted anything has no
		// user log, and that has to come back as a user scope with zero areas
		// rather than as a refusal. An omitted section would leave "I never
		// promoted anything" and "I did not look there" indistinguishable, which
		// is the failure this command exists to stop making.
		sc := common{scope: string(s), plan: c.plan, dir: c.dir}
		loc, lines, err := sc.load()
		if err != nil {
			return err
		}
		areas := memory.Areas(lines, stored, f)
		reports = append(reports, areaReport{Path: loc.Path, Scope: loc.Scope, Count: len(areas), Areas: areas})
	}

	emit(map[string]any{"plan": plan, "kinds": nonNil(splitList(*kinds)), "scopes": reports})
	return nil
}

// cmdAnchors is `gotcha`'s reader, and the only mechanical check in this tool of
// whether a record's claim about code still stands.
//
// It takes the same filter the read commands take, because the question "is what
// we wrote about this area still true" is asked by area, by kind and by plan the
// same way every other question here is. What it returns is one verdict per
// anchored record its filters admit and never the record's body: this answers
// whether to go and look, not what the record said.
//
// Exit 0 whatever the verdicts are. `changed` is an answer, not a refusal, and
// the exit code in this CLI says whether the command ran.
func cmdAnchors(args []string) error {
	fs := flag.NewFlagSet("anchors", flag.ExitOnError)
	var c common
	c.bind(fs)
	about := fs.String("about", "", "comma-separated tags; a tag ending in / matches by path prefix")
	kinds := fs.String("kind", "", "comma-separated kinds")
	slug := fs.String("slug", "", "exact slug")
	text := fs.String("text", "", "case-insensitive substring over title and body")
	ids := fs.String("id", "", "comma-separated record ids; an id this check did not return is listed under not_found, whether no record carries it, it carries no anchors, or another bound excluded it")
	verdict := fs.String("verdict", "", "return only the records whose own verdict is this one — "+
		joinVerdicts(memory.Verdicts())+" — where a record's verdict is its worst anchor's")
	path := fs.String("path", "", "return only the records carrying an anchor whose path contains this, case-insensitively")
	current := fs.Bool("current", false, "drop entries something else supersedes")
	limit := fs.Int("limit", 0, "check at most N, most recent first")
	all := fs.Bool("all", false, "check every record the store holds, each against the repository its own log lives in")
	repoRoot := fs.String("repo-root", "", "repository to check every record against (default: the repo the log lives in; under --scope user and under --all, the repository each record's own log lives in)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	idList := splitList(*ids)
	if len(idList) == 0 && len(namedFlags(fs, "id")) > 0 {
		return fail("empty_id", "--id %q names no id: a flag that parsed to nothing is what an unset "+
			"variable looks like, and checking the whole log there would answer a question nobody asked. "+
			"Name at least one id, or drop --id.", *ids)
	}
	var wantVerdict memory.Verdict
	if len(namedFlags(fs, "verdict")) > 0 {
		v, err := toVerdict(*verdict)
		if err != nil {
			return err
		}
		wantVerdict = v
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	var loc memory.Location
	var locs []memory.Location
	var held [][]memory.Line
	var stored []memory.Line
	if *all {
		if err := c.checkScopeFlags(); err != nil {
			return err
		}
		if err := refuseNamedScope(fs, &c, "what --all checks", "drop --all to check one log"); err != nil {
			return err
		}
		if locs, held, stored, err = everyLog(); err != nil {
			return err
		}
	} else {
		if loc, err = c.resolve(); err != nil {
			return err
		}
		locs = []memory.Location{loc}
		if *current {
			if held, stored, err = memory.ReadStoreEach("", loc.Path); err != nil {
				return fail("read_failed", "%s", err.Error())
			}
		} else {
			lines, err := readLog(loc)
			if err != nil {
				return err
			}
			held, stored = [][]memory.Line{lines}, lines
		}
	}

	var abs string
	var repos repoLocator
	if named := strings.TrimSpace(*repoRoot); named != "" || (!*all && loc.Scope == memory.ScopeRepo) {
		if abs, err = resolveRepoRoot(firstNonEmpty(named, loc.Repo)); err != nil {
			return err
		}
	} else if repos, err = locateRepos(); err != nil {
		return err
	}

	// The count-before-dump threshold is off here and only here. It exists to stop
	// a broad query from spending the caller's context on record bodies, and this
	// command returns no bodies — withholding the verdicts would leave the check
	// unrunnable over a whole log, which is the one way anybody wants to run it.
	filter := memory.Filter{
		Plan:        plan,
		ID:          idList,
		About:       splitList(*about),
		Kinds:       toKinds(splitList(*kinds)),
		Slug:        *slug,
		Text:        *text,
		CurrentOnly: *current,
		Limit:       *limit,
		Threshold:   -1,
	}
	res := memory.QueryEachIn(held, stored, filter)

	rooted := map[*memory.Entry]string{}
	if abs == "" {
		for i, lines := range held {
			if locs[i].Repo == "" {
				continue
			}
			for _, e := range memory.Entries(lines) {
				rooted[e] = locs[i].Repo
			}
		}
	}

	var checks []memory.RecordCheck
	if abs != "" {
		checks = memory.CheckEntries(abs, res.Records)
	} else {
		checks = memory.CheckEntriesAt(func(e *memory.Entry) (string, string) {
			if root, ok := rooted[e]; ok {
				return root, ""
			}
			return repos.locate(e)
		}, res.Records)
	}
	counts := map[memory.Verdict]int{}
	for _, v := range memory.Verdicts() {
		counts[v] = 0
	}
	for _, r := range checks {
		counts[r.Verdict]++
	}
	records := memory.FilterChecks(checks, memory.CheckFilter{Verdict: wantVerdict, Path: *path})

	out := map[string]any{
		"plan":    plan,
		"matched": res.Count,
		"checked": len(checks),
		"counts":  counts,
		"records": records,
	}
	if *all {
		out["logs"] = nonNil(pathsOf(locs))
	} else {
		out["path"], out["scope"] = loc.Path, loc.Scope
	}
	if abs != "" {
		out["repo_root"] = abs
	}
	if len(namedFlags(fs, "verdict", "path")) > 0 {
		out["returned"] = len(records)
	}
	if len(idList) > 0 {
		out["not_found"] = nonNil(idsNotReturned(idList, records))
	}
	emit(out)
	return nil
}

type repoLocator map[string][]string

func locateRepos() (repoLocator, error) {
	sources, err := memory.SourcePaths("")
	if err != nil {
		return nil, fail("read_failed", "%s", err.Error())
	}
	repos := repoLocator{}
	for _, src := range sources {
		if src.Scope != memory.ScopeRepo || src.RepoID == "" {
			continue
		}
		repos[src.RepoID] = append(repos[src.RepoID], logAt(src).Repo)
	}
	return repos, nil
}

func (r repoLocator) locate(e *memory.Entry) (string, string) {
	if e.Origin == nil || strings.TrimSpace(e.Origin.RepoID) == "" {
		return "", "the record carries no origin.repo_id, so nothing says which repository its anchors were read out of"
	}
	id := e.Origin.RepoID
	if filepath.IsAbs(id) && isDir(id) {
		return filepath.Clean(id), ""
	}
	roots := r[id]
	switch len(roots) {
	case 0:
		return "", fmt.Sprintf("origin.repo_id %s matches no repository log in %s and is not an existing directory",
			id, memory.CurrentStore())
	case 1:
		return roots[0], ""
	}
	var checkouts []string
	for _, root := range roots {
		if isDir(filepath.Join(root, ".git")) {
			checkouts = append(checkouts, root)
		}
	}
	if len(checkouts) == 1 {
		return checkouts[0], ""
	}
	return "", fmt.Sprintf("origin.repo_id %s is shared by %d repository logs and %d of their roots have a .git "+
		"directory rather than a .git file, where exactly one must: %s", id, len(roots), len(checkouts), strings.Join(roots, ", "))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func needsStore(f memory.Filter) bool { return f.AdmitsObjectives() || f.CurrentOnly }

func runQuery(c *common, f memory.Filter, proj ...memory.Projection) error {
	var loc memory.Location
	var lines, stored []memory.Line
	var err error
	if needsStore(f) {
		if loc, err = c.resolve(); err != nil {
			return err
		}
		var held [][]memory.Line
		if held, stored, err = memory.ReadStoreEach("", loc.Path); err != nil {
			return fail("read_failed", "%s", err.Error())
		}
		lines = held[0]
	} else if loc, lines, err = c.load(); err != nil {
		return err
	}
	out := queryAnswer(memory.QueryIn(lines, stored, f), f, proj...)
	out["path"], out["scope"] = loc.Path, loc.Scope
	emit(out)
	return nil
}

// runQueryBoth is `query --scope both`. The repository log is resolved first, so
// that outside a git repository the refusal `--scope repo` gives comes before the
// user location is created. Both are resolved the way `areas` loads them: two
// known locations, where a missing user log is an empty one rather than a refusal.
func runQueryBoth(c *common, f memory.Filter, proj memory.Projection) error {
	locs := make([]memory.Location, 0, 2)
	paths := make([]string, 0, 2)
	for _, s := range []memory.Scope{memory.ScopeRepo, memory.ScopeUser} {
		sc := common{scope: string(s), plan: c.plan, dir: c.dir}
		loc, err := sc.resolve()
		if err != nil {
			return err
		}
		locs = append(locs, loc)
		paths = append(paths, loc.Path)
	}
	var logs [][]memory.Line
	var stored []memory.Line
	if needsStore(f) {
		var err error
		if logs, stored, err = memory.ReadStoreEach("", paths...); err != nil {
			return fail("read_failed", "%s", err.Error())
		}
	} else {
		for _, loc := range locs {
			lines, err := readLog(loc)
			if err != nil {
				return err
			}
			logs = append(logs, lines)
		}
	}
	out := queryAnswer(memory.QueryEachIn(logs, stored, f), f, proj)
	out["scope"], out["paths"] = scopeBoth, paths
	emit(out)
	return nil
}

// queryAnswer is every key of a query's answer except the ones that say which log
// was read.
func queryAnswer(res memory.Result, f memory.Filter, proj ...memory.Projection) map[string]any {
	records := any(res.Records)
	if len(proj) > 0 {
		records = proj[0].Apply(res.Records)
	}
	out := map[string]any{
		"plan":  f.Plan,
		"count": res.Count, "returned": res.Returned, "truncated": res.Truncated,
		"records": records,
	}
	if res.Hint != "" {
		out["hint"] = res.Hint
	}
	if len(f.ID) > 0 {
		out["not_found"] = nonNil(res.NotFound)
	}
	// Only when the query admits objectives. A key that appeared on every read
	// would be answering a question `query --kind decision` did not ask.
	//
	// `search` always admits them: it has no --kind flag, so it has no way to
	// exclude an objective, and it therefore always carries the field. That is
	// the invariant holding rather than an exception to it — carving `search`
	// out would be a rule with nothing behind it.
	if res.CurrentObjective != nil {
		out["current_objective"] = res.CurrentObjective
	}
	return out
}

// cmdState is the derivation the README used to say did not exist: what state
// the edges put a record in, where it sits in the objective tree, what is
// holding it up and how far along it is.
//
// A read command over the whole log. `--plan` is accepted and ignored here, like
// everywhere but `query` and `search`, because narrowing the derivation would
// change the answers rather than the view: an objective's parent, its children
// and the current objective are properties of the log, not of the question.
func cmdState(args []string) error {
	fs := flag.NewFlagSet("state", flag.ExitOnError)
	var c common
	c.bind(fs)
	id := fs.String("id", "", "report one objective instead of every one")
	text := fs.Bool("text", false, "print what is waiting as text, over every log in the store instead of this repository")
	here := fs.Bool("here", false, "narrow --text to this repository")
	all := fs.Bool("all", false, "widen the JSON report to every log in the store")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *all && *here {
		return fail("conflicting_scope", "state takes --all or --here, not both")
	}
	if *all || (*text && !*here) {
		if err := c.checkScopeFlags(); err != nil {
			return err
		}
		subject, alternative := "what --all reports", "drop --all to report this repository's log"
		if !*all {
			subject, alternative = "what --text prints", "add --here to print this repository's"
		}
		if err := refuseNamedScope(fs, &c, subject, alternative); err != nil {
			return err
		}
		return stateAcrossLogs(*id, *text)
	}

	load := c.load
	if *here {
		load = c.loadExisting
	}
	loc, lines, err := load()
	if err != nil {
		return err
	}
	stored, err := memory.ReadStore("")
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}

	tree := memory.NewTreeIn(lines, stored)
	rep := tree.Report().WithRoot(loc.Repo)

	if *id != "" {
		filtered, n := filterByID(rep, *id)
		if n == 0 {
			return fail("unknown_objective", "no objective with id %s in log %s of %s",
				*id, loc.Path, memory.CurrentStore())
		}
		rep = filtered
	}

	if *text {
		fmt.Print(memory.RenderWaiting([]memory.LogState{{Name: loc.Name(), Tree: tree, Report: rep}}, nil))
		return nil
	}

	emit(map[string]any{
		"path": loc.Path, "scope": loc.Scope,
		"objectives_from": loc.Path,
		"derived_over":    memory.CurrentStore(),
		"count":           len(rep.Objectives),
		"objectives":      rep.Objectives,
		"excluded":        rep.Excluded,
		"current":         rep.Current,
		"anomalies":       rep.Anomalies,
	})
	return nil
}

func everyLog() ([]memory.Location, [][]memory.Line, []memory.Line, error) {
	sources, err := memory.SourcePaths("")
	if err != nil {
		return nil, nil, nil, fail("read_failed", "%s", err.Error())
	}
	locs := make([]memory.Location, 0, len(sources))
	held := make([][]memory.Line, 0, len(sources))
	var stored []memory.Line
	for _, src := range sources {
		loc := logAt(src)
		lines, err := readLog(loc)
		if err != nil {
			return nil, nil, nil, err
		}
		locs = append(locs, loc)
		held = append(held, lines)
		stored = append(stored, lines...)
	}
	return locs, held, stored, nil
}

func storeLogs() ([]string, error) {
	sources, err := memory.SourcePaths("")
	if err != nil {
		return nil, fail("read_failed", "%s", err.Error())
	}
	paths := make([]string, 0, len(sources))
	for _, src := range sources {
		paths = append(paths, src.Path)
	}
	return paths, nil
}

func pathsOf(locs []memory.Location) []string {
	var paths []string
	for _, loc := range locs {
		paths = append(paths, loc.Path)
	}
	return paths
}

func stateAcrossLogs(id string, text bool) error {
	locs, held, stored, err := everyLog()
	if err != nil {
		return err
	}
	paths := pathsOf(locs)

	logs := make([]memory.LogState, 0, len(locs))
	reports := make([]memory.RepoReport, 0, len(locs))
	seen := map[string]bool{}
	matched := 0

	for i, loc := range locs {
		tree := memory.NewTreeIn(held[i], stored)
		rep := dropSeen(tree.Report().WithRoot(loc.Repo), loc.Scope, seen)
		if id != "" {
			var n int
			rep, n = filterByID(rep, id)
			matched += n
		}
		logs = append(logs, memory.LogState{Name: loc.Name(), Tree: tree, Report: rep})
		reports = append(reports, memory.RepoReport{Repo: loc.Name(), Report: rep})
	}

	if id != "" && matched == 0 {
		return fail("unknown_objective", "no objective with id %s in any of the %d logs in %s",
			id, len(paths), memory.CurrentStore())
	}

	if text {
		fmt.Print(memory.RenderWaiting(logs, nil))
		return nil
	}

	merged := memory.MergeReports(reports)
	emit(map[string]any{
		"logs":       nonNil(paths),
		"count":      len(merged.Objectives),
		"objectives": merged.Objectives,
		"excluded":   merged.Excluded,
		"current":    merged.Current,
		"anomalies":  merged.Anomalies,
	})
	return nil
}

func dropSeen(rep memory.StateReport, scope memory.Scope, seen map[string]bool) memory.StateReport {
	first := func(list, id string) bool {
		key := list + "\x00" + id + "\x00" + string(scope)
		if seen[key] {
			return false
		}
		seen[key] = true
		return true
	}
	objectives := []memory.ObjectiveState{}
	for _, o := range rep.Objectives {
		if first("objective", o.ID) {
			objectives = append(objectives, o)
		}
	}
	excluded := []memory.ExcludedObjective{}
	for _, e := range rep.Excluded {
		if first("objective", e.ID) {
			excluded = append(excluded, e)
		}
	}
	anomalies := []memory.Anomaly{}
	for _, a := range rep.Anomalies {
		if first("anomaly", a.ID+"\x00"+a.Message) {
			anomalies = append(anomalies, a)
		}
	}
	if rep.Current.ID != "" && !first("current", rep.Current.ID) {
		rep.Current = memory.CurrentObjective{}
	}
	rep.Objectives, rep.Excluded, rep.Anomalies = objectives, excluded, anomalies
	return rep
}

func filterByID(rep memory.StateReport, id string) (memory.StateReport, int) {
	objectives := []memory.ObjectiveState{}
	for _, o := range rep.Objectives {
		if o.ID == id {
			objectives = append(objectives, o)
		}
	}
	excluded := []memory.ExcludedObjective{}
	for _, e := range rep.Excluded {
		if e.ID == id {
			excluded = append(excluded, e)
		}
	}
	// The current objective and the anomalies stay as they are. Both are
	// answers about the whole log, and narrowing to one record does not make
	// a different objective current.
	rep.Objectives, rep.Excluded = objectives, excluded
	return rep, len(objectives) + len(excluded)
}

var edgesHintAdvice = []string{
	"narrow with --type/--from/--to/--since/--until",
	"pass --limit N to take the most recent",
	"ask for --count alone",
}

func cmdEdges(args []string) error {
	fs := flag.NewFlagSet("edges", flag.ExitOnError)
	var c common
	c.bind(fs)
	types := fs.String("type", "", "comma-separated edge types (see `memory kinds`); a type outside the closed set is refused")
	from := fs.String("from", "", "exact id the edge points from; naming an endpoint lists every log of the store rather than this one")
	to := fs.String("to", "", "exact id the edge points to; naming an endpoint lists every log of the store rather than this one")
	since := fs.String("since", "", "edges stamped at this RFC3339 timestamp or later; the bound itself is included, and compared as written, so a bare date starts that day")
	until := fs.String("until", "", "edges stamped at this RFC3339 timestamp or earlier; the bound itself is included, and compared as written, so a bare date stops just before that day")
	count := fs.Bool("count", false, "report how many match and return no edges")
	limit := fs.Int("limit", 0, "return at most N, most recent first")
	threshold := fs.Int("threshold", 0, "override the count-before-dump threshold; -1 disables it")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if len(namedFlags(fs, "plan")) > 0 {
		return fail("plan_not_on_edges", "--plan %q bounds nothing here: an edge carries no plan, only the "+
			"records at its two ends do, so bounding edges by one would mean joining against them — which this "+
			"command does not do. Drop --plan, or name an endpoint with --from/--to.", c.plan)
	}
	wantTypes, err := toEdgeTypes(splitList(*types))
	if err != nil {
		return err
	}
	loc, err := c.resolve()
	if err != nil {
		return err
	}
	endpoint := *from != "" || *to != ""
	if endpoint {
		if err := refuseNamedScope(fs, &c, "the search for an edge at --from/--to",
			"drop --from/--to to list one log's edges"); err != nil {
			return err
		}
	}
	var lines []memory.Line
	var logs []string
	if endpoint {
		if lines, err = memory.ReadStore(""); err != nil {
			return fail("read_failed", "%s", err.Error())
		}
		if logs, err = storeLogs(); err != nil {
			return err
		}
	} else if lines, err = readLog(loc); err != nil {
		return err
	}

	res := memory.QueryEdges(lines, memory.EdgeFilter{
		Types:      wantTypes,
		From:       *from,
		To:         *to,
		Since:      strings.TrimSpace(*since),
		Until:      strings.TrimSpace(*until),
		CountOnly:  *count,
		HintAdvice: edgesHintAdvice,
		Limit:      *limit,
		Threshold:  *threshold,
	})

	out := map[string]any{
		"count": res.Count, "returned": res.Returned, "truncated": res.Truncated,
		"edges": res.Edges,
	}
	if endpoint {
		out["logs"] = logs
	} else {
		out["path"], out["scope"] = loc.Path, loc.Scope
	}
	if res.Hint != "" {
		out["hint"] = res.Hint
	}
	emit(out)
	return nil
}

func cmdPromote(args []string) error {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	var c common
	c.bind(fs)
	ids := fs.String("ids", "", "comma-separated entry ids to promote")
	kinds := fs.String("kind", "", "comma-separated kinds to promote as a class (see `memory kinds`)")
	to := fs.String("to", "", "target scope: user")
	dryRun := fs.Bool("dry-run", false, "print what would be promoted and write nothing")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	want := splitList(*ids)
	wantKinds := toKinds(splitList(*kinds))
	// A union, and either one alone is enough: naming a class and naming a record
	// are two ways of asking for the same operation. Only having asked for neither
	// is a refusal, and it names both ways of asking rather than the one it
	// happens to check first.
	if len(want) == 0 && len(wantKinds) == 0 {
		return fail("missing_selection", "promote needs --ids <id>[,<id>...] or --kind <kind>[,<kind>...], or both")
	}
	// toKinds validates nothing — on `query` an unknown kind simply matches no
	// record, which is a harmless empty answer. Here it would be a promotion that
	// silently moved nothing, and a caller reading `promoted: []` cannot tell that
	// from a class that happened to be empty. So the closed set is checked, and
	// the refusal carries it rather than leaving the caller to guess which four.
	// The structure is emitted rather than only spelled into the message for the
	// reason `add` emits its problems: a caller should not have to parse prose to
	// find out what it may say instead.
	for _, k := range wantKinds {
		if memory.IsPromotable(k) {
			continue
		}
		emit(map[string]any{
			"error": fmt.Sprintf("kind %q is not promotable as a class: --kind takes one of %s", k, joinKinds(memory.PromotableKinds())),
			"code":  "kind_not_promotable", "kind": string(k), "promotable": memory.PromotableKinds(),
		})
		os.Exit(1)
	}
	target := memory.Scope(*to)
	switch target {
	case memory.ScopeUser:
	case memory.ScopeRepo:
		// Promotion is a copy across files, and repo → repo is not a move at
		// all: since schema version 2 the repo log is where a plan's records are
		// written in the first place, so there is nothing below it to lift them
		// out of. Only repo → user still crosses a file boundary.
		return fail("target_removed", "promote --to repo was removed: a plan's records are already in "+
			"the repo log, so there is nothing to promote them from; only --to user crosses logs")
	default:
		return fail("bad_target", "--to must be user")
	}

	srcLoc, lines, err := c.load()
	if err != nil {
		return err
	}
	dstLoc, err := memory.Resolve(target, c.dir)
	if err != nil {
		return fail("bad_scope", "%s", err.Error())
	}
	if dstLoc.Path == srcLoc.Path {
		return fail("same_scope", "source and target resolve to the same log")
	}
	// The destination is created like every location this tool resolves, and after
	// the refusals above so that a promote that never happens creates nothing. It
	// is a write, so the failure is a refusal: resolveForWrite's reasoning, applied
	// to the one location that does not come from a --scope flag.
	//
	// Except under --dry-run, which is the one path here that is not a write at
	// all: a preview that left a directory behind would have been a preview you
	// cannot take back. A destination that does not exist reads as an empty log,
	// so the report is the same one the real run would print.
	if !*dryRun {
		if err := materialize(dstLoc); err != nil {
			return err
		}
	}

	dstLines, err := readLog(dstLoc)
	if err != nil {
		return err
	}

	plan := memory.PlanPromotion(
		memory.Entries(lines), memory.Edges(lines),
		memory.Entries(dstLines), memory.Edges(dstLines),
		want, target, wantKinds...,
	)

	if len(plan.Records) > 0 && !*dryRun {
		if err := writeLog(dstLoc).Append(plan.Records...); err != nil {
			return fail("write_failed", "%s", err.Error())
		}
	}

	// The same report either way, down to the keys, with one boolean saying which
	// run this was. A preview in a shape of its own would be a second thing to
	// read and a second thing to keep in step with the first, and its whole job is
	// to be believed about what the real run will do.
	emit(map[string]any{
		"from": srcLoc.Path, "to": dstLoc.Path,
		"promoted": nonNil(plan.Promoted), "already_present": nonNil(plan.AlreadyPresent),
		"unknown": nonNil(plan.Unknown), "edges_copied": plan.EdgesCopied,
		"border": nonNil(plan.Border), "border_count": len(plan.Border),
		"edges_skipped": plan.EdgesSkipped, "dry_run": *dryRun,
	})
	return nil
}

func joinKinds(ks []memory.Kind) string {
	out := make([]string, 0, len(ks))
	for _, k := range ks {
		out = append(out, string(k))
	}
	return strings.Join(out, ", ")
}

func joinEdgeTypes(ts []memory.EdgeType) string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, string(t))
	}
	return strings.Join(out, ", ")
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	var c common
	c.bind(fs)
	all := fs.Bool("all", false, "validate every log in the store")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	// A location that could not be created is reported and never dropped: an
	// .ai-kit/ nobody can write to still reads, records and all, and validating a
	// smaller set than the caller asked about while answering ok=true is the
	// failure --all already had once. It never touches `ok`, because whether a
	// directory could be created says nothing about whether the records inside it
	// are well-formed, and collapsing the two would leave a caller unable to tell
	// a corrupt log from a read-only one. It does move the exit code — see below.
	var locs []memory.Location
	notCreated := []string{}
	if *all {
		if err := refuseNamedScope(fs, &c, "what --all validates", "drop --all to validate one log"); err != nil {
			return err
		}
		sources, err := memory.SourcePaths("")
		if err != nil {
			return fail("read_failed", "%s", err.Error())
		}
		for _, src := range sources {
			locs = append(locs, logAt(src))
		}
	} else {
		l, err := c.locate()
		if err != nil {
			return err
		}
		locs = append(locs, l)
		if err := materialize(l); err != nil {
			notCreated = append(notCreated, err.Error())
		}
	}

	stored, err := memory.ReadStore("")
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}
	known := map[string]bool{}
	for _, e := range memory.Entries(stored) {
		known[e.ID] = true
	}

	reports := make([]memory.Report, 0, len(locs))
	ok := true
	unjudged := 0
	for _, l := range locs {
		lines, err := memory.Open(l.Path).Read()
		if err != nil {
			return fail("read_failed", "%s", err.Error())
		}
		r := memory.ValidateLogIn(l.Path, lines, known)
		reports = append(reports, r)
		ok = ok && r.OK
		unjudged += len(r.Unjudged)
	}

	// The unjudged count is surfaced at the top level next to ok, because it
	// is the one number a caller must not miss: ok=true with unjudged>0 means
	// "nothing is wrong with what I could read", which is a different claim
	// from "nothing is wrong". Records this binary is too old to judge are not
	// a failure, so they never flip ok or the exit code.
	out := map[string]any{"ok": ok, "reports": reports}
	if unjudged > 0 {
		out["unjudged"] = unjudged
	}
	if len(notCreated) > 0 {
		out["init_failed"] = notCreated
	}
	emit(out)

	// Two independent failures, one non-zero exit, and the JSON says which. A
	// caller that reads only the exit code — which is every caller in a shell
	// pipeline — must not be told that a sweep succeeded when it covered less than
	// it was asked to cover; a JSON key nobody parsed is not a report. `ok` stays
	// the narrower claim it always was, about the records, so the two remain
	// distinguishable for whoever does read the output.
	if !ok || len(notCreated) > 0 {
		os.Exit(1)
	}
	return nil
}

func cmdKinds(args []string) error {
	fs := flag.NewFlagSet("kinds", flag.ExitOnError)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	// The justification allowlist is published beside the closed sets, and for
	// the same reason they are: a rule the tool enforces but does not advertise
	// is a rule a caller only meets as a refusal. Only the types that carry one
	// appear here — an absent key means the type has no rule, which is not the
	// same claim as an empty list.
	justifications := map[string][]memory.Kind{}
	for _, t := range memory.JustifiedEdgeTypes() {
		justifications[string(t)] = memory.Justifications(t)
	}

	emit(map[string]any{
		"entry_kinds":    memory.EntryKinds(),
		"edge_types":     memory.EdgeTypes(),
		"scopes":         memory.Scopes(),
		"threshold":      memory.DefaultThreshold,
		"justifications": justifications,
		"anchored_kinds": memory.AnchoredKinds(),
		// What `promote --kind` will take as a class, which is neither every kind
		// nor the Knowledge group. A caller that had to discover it by being
		// refused would be a caller guessing.
		"promotable_kinds": memory.PromotableKinds(),
		"verdicts":         memory.Verdicts(),
	})
	return nil
}

// ----------------------------------------------------------------- helpers ---

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func deriveSlug(title string) string {
	s := nonSlug.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveRepoRoot turns whatever names the working tree into an absolute path,
// and refuses one that is not a directory.
//
// The refusal is the point. Every anchor resolves under this path, so a typo in
// it would otherwise come back as `could_not_check` on every record at once —
// and that verdict is read as "the claim could not be verified", which is a
// sentence about the code rather than about the flag. One refusal says which of
// the two it is.
func resolveRepoRoot(root string) (string, error) {
	if root == "" {
		return "", fail("no_repo_root", "no repository to work against: pass --repo-root <path>")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fail("no_repo_root", "%s", err.Error())
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fail("no_repo_root", "--repo-root %s: %s", root, err.Error())
	}
	if !info.IsDir() {
		return "", fail("no_repo_root", "--repo-root %s is not a directory", root)
	}
	return abs, nil
}

// anchorSpecs collects a repeatable --anchor. Repeatable rather than
// comma-separated like --about and --ids: those take single tokens, and a path
// is the one value here that could plausibly contain the separator.
type anchorSpecs []string

func (a *anchorSpecs) String() string { return strings.Join(*a, " ") }

func (a *anchorSpecs) Set(v string) error {
	*a = append(*a, v)
	return nil
}

func buildAnchors(repoRoot string, specs anchorSpecs) ([]memory.Anchor, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	if repoRoot == "" {
		return nil, fail("no_repo_root", "--anchor needs a repository to read the cited lines out of, "+
			"and --scope user resolves to a log outside any repo: pass --repo-root <path>")
	}
	root, err := resolveRepoRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	out := make([]memory.Anchor, 0, len(specs))
	for _, spec := range specs {
		path, line, height, err := memory.ParseAnchorSpec(spec)
		if err != nil {
			return nil, fail("bad_anchor", "%s", err.Error())
		}
		a, err := memory.NewAnchor(root, path, line, height)
		if err != nil {
			return nil, fail("bad_anchor", "%s", err.Error())
		}
		out = append(out, a)
	}
	return out, nil
}

func toEdgeTypes(ss []string) ([]memory.EdgeType, error) {
	known := memory.EdgeTypes()
	out := make([]memory.EdgeType, 0, len(ss))
	for _, s := range ss {
		t := memory.EdgeType(s)
		if !slices.Contains(known, t) {
			return nil, fail("unknown_edge_type", "%q is not an edge type: --type takes any of %s",
				s, joinEdgeTypes(known))
		}
		out = append(out, t)
	}
	return out, nil
}

func toVerdict(s string) (memory.Verdict, error) {
	known := memory.Verdicts()
	v := memory.Verdict(strings.TrimSpace(s))
	if !slices.Contains(known, v) {
		return "", fail("unknown_verdict", "%q is not a verdict: --verdict takes one of %s, and one value rather than a list",
			s, joinVerdicts(known))
	}
	return v, nil
}

func joinVerdicts(vs []memory.Verdict) string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, string(v))
	}
	return strings.Join(out, ", ")
}

func idsNotReturned(ids []string, checks []memory.RecordCheck) []string {
	var out []string
	for _, id := range ids {
		if !slices.ContainsFunc(checks, func(c memory.RecordCheck) bool { return c.ID == id }) {
			out = append(out, id)
		}
	}
	return out
}

func toKinds(ss []string) []memory.Kind {
	out := make([]memory.Kind, 0, len(ss))
	for _, s := range ss {
		out = append(out, memory.Kind(s))
	}
	return out
}

// repoLabel is the directory name a repo root is known by, and empty for a root
// that is empty — where filepath.Base would answer ".", a label that names no
// repository and reads like one.
func repoLabel(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Base(filepath.Clean(root))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
