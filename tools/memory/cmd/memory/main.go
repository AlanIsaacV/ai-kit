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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alanv/ai-kit/tools/memory/internal/memory"
)

const usage = `memory — append-only knowledge log

Usage:
  memory init      [--scope repo|user]
  memory add       --plan SLUG --kind KIND --title TEXT --about TAG[,TAG...] [--body TEXT] [--slug SLUG] [--anchor PATH:LINE] [--repo-root PATH]
  memory link      --type TYPE --from ID --to ID
  memory query     [--plan SLUG] [--about TAGS] [--kind KINDS] [--slug SLUG] [--text S] [--current] [--limit N]
  memory search    TEXT [--plan SLUG] [--current] [--limit N]
  memory anchors   [--plan SLUG] [--about TAGS] [--kind KINDS] [--slug SLUG] [--current] [--repo-root PATH]
  memory graph     --from ID [--depth N]
  memory state     [--id ID]
  memory render    --as KIND
  memory promote   --ids ID[,ID...] --to user
  memory validate  [--all]
  memory kinds

Common flags:
  --scope repo|user        which log to act on (default: repo)
  --plan SLUG              which plan a record belongs to: stamped by 'add',
                           bounds 'query' and 'search'. Defaults to AI_KIT_PLAN.
                           Other commands accept it and read the whole log.
  --dir PATH               start directory for repo discovery (default: cwd)

There is one log per repo, at <repo>/.ai-kit/memory.jsonl. The plan a record
belongs to lives in the record, as origin.plan, so any two records can be linked
whatever plan produced them.

Every command prints JSON on stdout, on success and on failure alike, except
'render' which prints markdown. Exit 0 means the command ran; exit 1 means it
refused, and the JSON says why.
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
	case "anchors":
		err = cmdAnchors(os.Args[2:])
	case "graph":
		err = cmdGraph(os.Args[2:])
	case "state":
		err = cmdState(os.Args[2:])
	case "render":
		err = cmdRender(os.Args[2:])
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
// and ignored, because a caller that has a plan slug in hand will pass it, and
// refusing the flag would break every invocation that names one out of habit.
func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.scope, "scope", "repo", "repo|user")
	fs.StringVar(&c.plan, "plan", "", "plan slug: stamped by add, bounds query and search")
	fs.StringVar(&c.dir, "dir", "", "start directory for repo discovery")
}

func (c *common) resolve() (memory.Location, error) {
	loc, err := memory.Resolve(memory.Scope(c.scope), c.dir)
	if err != nil {
		return loc, fail("bad_scope", "%s", err.Error())
	}
	return loc, nil
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

// planStamp is the plan a write records itself under. Unlike planFilter it
// refuses to be empty: see memory.ErrNoPlan.
func (c *common) planStamp() (string, error) {
	slug, err := memory.RequirePlan(c.plan)
	if err != nil {
		if errors.Is(err, memory.ErrNoPlan) {
			return "", fail("no_plan", "%s", err.Error())
		}
		return "", fail("bad_plan", "%s", err.Error())
	}
	return slug, nil
}

func (c *common) load() (memory.Location, []memory.Line, error) {
	loc, err := c.resolve()
	if err != nil {
		return loc, nil, err
	}
	lines, err := memory.Open(loc.Path).Read()
	if err != nil {
		return loc, nil, fail("read_failed", "%s", err.Error())
	}
	return loc, lines, nil
}

// ------------------------------------------------------------- subcommands ---

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	var c common
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	loc, err := c.resolve()
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	loc, err := c.resolve()
	if err != nil {
		return err
	}
	// Before the id, before anything: a record whose plan cannot be named is a
	// record no bounded query will ever return, and the log is append-only, so
	// there is no taking it back afterwards.
	plan, err := c.planStamp()
	if err != nil {
		return err
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
	//
	// --repo-root is what makes an anchored record writable at user scope, where
	// the log resolves to a home directory and loc.Repo is empty. The requirement
	// itself never bends: a `gotcha` is refused without an anchor in every scope,
	// so what this flag changes is that the root can be supplied rather than only
	// inferred.
	anchorRepo := firstNonEmpty(*anchorRoot, loc.Repo)
	built, err := buildAnchors(anchorRepo, anchors)
	if err != nil {
		return err
	}

	origin := &memory.Origin{
		// The repository the anchors were read out of is the repository this
		// record came from, and at user scope it is the only one there is:
		// loc.Repo is empty there, and filepath.Base("") is ".", which names
		// nothing. An anchor's path says where in a repository and never which
		// one, so a user-scope record whose origin.repo read "." would carry a
		// citation nobody could resolve.
		Repo:   firstNonEmpty(*repo, repoLabel(loc.Repo), repoLabel(anchorRepo)),
		Plan:   plan,
		Author: firstNonEmpty(*author, os.Getenv("AI_KIT_AUTHOR"), gitUserName(loc.Repo)),
		Unit:   *unit,
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
		Origin:  origin,
	}

	// Validate before writing. The log is append-only, so a bad record cannot
	// be taken back — the only place to stop it is here.
	if ps := memory.ValidateEntry(e); len(ps) > 0 {
		emit(map[string]any{"error": "record rejected", "code": "invalid_record", "problems": ps})
		os.Exit(1)
	}

	if err := memory.Open(loc.Path).Append(e); err != nil {
		return fail("write_failed", "%s", err.Error())
	}
	emit(map[string]any{"path": loc.Path, "record": e})
	return nil
}

func cmdLink(args []string) error {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	var c common
	c.bind(fs)
	typ := fs.String("type", "", "edge type (see `memory kinds`)")
	from := fs.String("from", "", "source entry id")
	to := fs.String("to", "", "target entry id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	loc, lines, err := c.load()
	if err != nil {
		return err
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
		return fail("dangling_edge", "no entry in %s for %s", loc.Path, strings.Join(missing, ", "))
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

	if err := memory.Open(loc.Path).Append(edge); err != nil {
		return fail("write_failed", "%s", err.Error())
	}
	emit(map[string]any{"path": loc.Path, "edge": edge})
	return nil
}

func cmdQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	var c common
	c.bind(fs)
	about := fs.String("about", "", "comma-separated tags; a tag ending in / matches by path prefix")
	kinds := fs.String("kind", "", "comma-separated kinds")
	slug := fs.String("slug", "", "exact slug")
	text := fs.String("text", "", "case-insensitive substring over title and body")
	current := fs.Bool("current", false, "drop entries something else supersedes")
	limit := fs.Int("limit", 0, "return at most N, most recent first")
	threshold := fs.Int("threshold", 0, "override the count-before-dump threshold; -1 disables it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	return runQuery(&c, memory.Filter{
		Plan:        plan,
		About:       splitList(*about),
		Kinds:       toKinds(splitList(*kinds)),
		Slug:        *slug,
		Text:        *text,
		CurrentOnly: *current,
		Limit:       *limit,
		Threshold:   *threshold,
	})
}

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	var c common
	c.bind(fs)
	current := fs.Bool("current", false, "drop entries something else supersedes")
	limit := fs.Int("limit", 0, "return at most N, most recent first")
	threshold := fs.Int("threshold", 0, "override the count-before-dump threshold; -1 disables it")
	if err := fs.Parse(args); err != nil {
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
		Plan: plan, Text: text, CurrentOnly: *current, Limit: *limit, Threshold: *threshold,
	})
}

// cmdAnchors is `gotcha`'s reader, and the only mechanical check in this tool of
// whether a record's claim about code still stands.
//
// It takes the same filter the read commands take, because the question "is what
// we wrote about this area still true" is asked by area, by kind and by plan the
// same way every other question here is. What it returns is one verdict per
// anchored record and never the record's body: this answers whether to go and
// look, not what the record said.
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
	current := fs.Bool("current", false, "drop entries something else supersedes")
	limit := fs.Int("limit", 0, "check at most N, most recent first")
	repoRoot := fs.String("repo-root", "", "repository to check against (default: the repo the log lives in)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := c.planFilter()
	if err != nil {
		return err
	}
	loc, lines, err := c.load()
	if err != nil {
		return err
	}

	abs, err := resolveRepoRoot(firstNonEmpty(*repoRoot, loc.Repo))
	if err != nil {
		return err
	}

	// The count-before-dump threshold is off here and only here. It exists to stop
	// a broad query from spending the caller's context on record bodies, and this
	// command returns no bodies — withholding the verdicts would leave the check
	// unrunnable over a whole log, which is the one way anybody wants to run it.
	res := memory.Query(lines, memory.Filter{
		Plan:        plan,
		About:       splitList(*about),
		Kinds:       toKinds(splitList(*kinds)),
		Slug:        *slug,
		Text:        *text,
		CurrentOnly: *current,
		Limit:       *limit,
		Threshold:   -1,
	})

	checks := memory.CheckEntries(abs, res.Records)
	counts := map[memory.Verdict]int{}
	for _, v := range memory.Verdicts() {
		counts[v] = 0
	}
	for _, r := range checks {
		counts[r.Verdict]++
	}

	emit(map[string]any{
		"path": loc.Path, "scope": loc.Scope, "plan": plan,
		"repo_root": abs,
		"matched":   res.Count,
		"checked":   len(checks),
		"counts":    counts,
		"records":   checks,
	})
	return nil
}

func runQuery(c *common, f memory.Filter) error {
	loc, lines, err := c.load()
	if err != nil {
		return err
	}
	res := memory.Query(lines, f)
	out := map[string]any{
		"path": loc.Path, "scope": loc.Scope, "plan": f.Plan,
		"count": res.Count, "returned": res.Returned, "truncated": res.Truncated,
		"records": res.Records,
	}
	if res.Hint != "" {
		out["hint"] = res.Hint
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
	emit(out)
	return nil
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	loc, lines, err := c.load()
	if err != nil {
		return err
	}

	rep := memory.NewTree(lines).Report()

	if *id != "" {
		objectives := []memory.ObjectiveState{}
		for _, o := range rep.Objectives {
			if o.ID == *id {
				objectives = append(objectives, o)
			}
		}
		excluded := []memory.ExcludedObjective{}
		for _, e := range rep.Excluded {
			if e.ID == *id {
				excluded = append(excluded, e)
			}
		}
		if len(objectives)+len(excluded) == 0 {
			return fail("unknown_objective", "no objective with id %s in %s", *id, loc.Path)
		}
		// The current objective and the anomalies stay as they are. Both are
		// answers about the whole log, and narrowing to one record does not make
		// a different objective current.
		rep.Objectives, rep.Excluded = objectives, excluded
	}

	emit(map[string]any{
		"path": loc.Path, "scope": loc.Scope,
		"count":      len(rep.Objectives),
		"objectives": rep.Objectives,
		"excluded":   rep.Excluded,
		"current":    rep.Current,
		"anomalies":  rep.Anomalies,
	})
	return nil
}

func cmdGraph(args []string) error {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	var c common
	c.bind(fs)
	from := fs.String("from", "", "entry id to walk from")
	depth := fs.Int("depth", 1, "how many hops")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return fail("missing_from", "graph needs --from <id>")
	}
	loc, lines, err := c.load()
	if err != nil {
		return err
	}
	nodes := memory.Graph(memory.Entries(lines), memory.Edges(lines), *from, *depth)
	if nodes == nil {
		return fail("unknown_id", "no entry with id %s in %s", *from, loc.Path)
	}
	emit(map[string]any{"path": loc.Path, "from": *from, "depth": *depth, "count": len(nodes), "nodes": nodes})
	return nil
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	var c common
	c.bind(fs)
	as := fs.String("as", "", "kind to render, e.g. adr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *as == "" {
		return fail("missing_kind", "render needs --as <kind>")
	}
	loc, lines, err := c.load()
	if err != nil {
		return err
	}
	fmt.Print(memory.Render(memory.Entries(lines), memory.Edges(lines), memory.Kind(*as), loc.Scope))
	return nil
}

func cmdPromote(args []string) error {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	var c common
	c.bind(fs)
	ids := fs.String("ids", "", "comma-separated entry ids to promote")
	to := fs.String("to", "", "target scope: user")
	if err := fs.Parse(args); err != nil {
		return err
	}
	want := splitList(*ids)
	if len(want) == 0 {
		return fail("missing_ids", "promote needs --ids <id>[,<id>...]")
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

	dstLines, err := memory.Open(dstLoc.Path).Read()
	if err != nil {
		return fail("read_failed", "%s", err.Error())
	}

	plan := memory.PlanPromotion(
		memory.Entries(lines), memory.Edges(lines),
		memory.Entries(dstLines), memory.Edges(dstLines),
		want, target,
	)

	if len(plan.Records) > 0 {
		if err := memory.Open(dstLoc.Path).Append(plan.Records...); err != nil {
			return fail("write_failed", "%s", err.Error())
		}
	}

	emit(map[string]any{
		"from": srcLoc.Path, "to": dstLoc.Path,
		"promoted": nonNil(plan.Promoted), "already_present": nonNil(plan.AlreadyPresent),
		"unknown": nonNil(plan.Unknown), "edges_copied": plan.EdgesCopied,
	})
	return nil
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
	all := fs.Bool("all", false, "validate every log reachable from here")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var locs []memory.Location
	if *all {
		// Every scope there is. It used to include the plan scope, and that made
		// --all a sweep that swept nothing: resolving it needed a single plan
		// directory to pick, a repo with several had none, and the loop skipped
		// the failure and reported ok over an empty set. With one log per repo
		// there is nothing left to guess.
		for _, s := range []memory.Scope{memory.ScopeRepo, memory.ScopeUser} {
			l, err := memory.Resolve(s, c.dir)
			if err != nil {
				continue // a scope that cannot be resolved simply has no log
			}
			locs = append(locs, l)
		}
	} else {
		l, err := c.resolve()
		if err != nil {
			return err
		}
		locs = append(locs, l)
	}

	reports := make([]memory.Report, 0, len(locs))
	ok := true
	unjudged := 0
	for _, l := range locs {
		lines, err := memory.Open(l.Path).Read()
		if err != nil {
			return fail("read_failed", "%s", err.Error())
		}
		r := memory.ValidateLog(l.Path, lines)
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
	emit(out)
	if !ok {
		os.Exit(1)
	}
	return nil
}

func cmdKinds(args []string) error {
	fs := flag.NewFlagSet("kinds", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
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
		"verdicts":       memory.Verdicts(),
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

func gitUserName(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
