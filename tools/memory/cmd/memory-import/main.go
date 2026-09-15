// Command memory-import copies every memory log on this host into one SQLite
// store, a row per line, with the repository each line came from beside it.
//
// It runs before anything reads that store because it holds the one detail of
// the move to a single store that cannot be recovered later: a record written
// before origin.repo_id existed names its repository only through the path of
// the log it sits in. The line is stored exactly as written and the identity goes
// in a column, so no log is ever written to — `memory` keeps reading and writing
// the logs as before.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/alanv/ai-kit/tools/memory/cmd/internal/gitcmd"
	"github.com/alanv/ai-kit/tools/memory/internal/memory"
)

const usage = `memory-import — copy every memory log on this host into one store

Usage:
  memory-import [--store PATH]

Reads the logs 'memory state --all' reads: every <repo>/.ai-kit/memory.jsonl
under MEMORY_ROOTS (a PATH-style list, default /srv/projects), plus
$HOME/.ai-kit/memory.jsonl. Every non-blank line not yet in the store at PATH
(default /srv/projects/.ai-kit/memory.db) is inserted as written, keyed by the
log's absolute path and the line number, with the log's scope and, for a repo
log, the identity 'memory add' stamps for that repository. A line already stored
is skipped, so a second run over unchanged logs inserts nothing.

The run commits all of it or nothing. It commits nothing when a log cannot be
read, and nothing when a stored line differs from its log or the log no longer
has it: an append-only log was rewritten, and the store keeps what it had.

Prints one JSON object on success and on failure. Exit 0 means the store holds
every line; exit 1 means nothing was committed, and the JSON says why.
`

const defaultStore = "/srv/projects/.ai-kit/memory.db"

const schema = `CREATE TABLE IF NOT EXISTS lines (
	source_path TEXT    NOT NULL,
	line_number INTEGER NOT NULL,
	scope       TEXT    NOT NULL,
	repo_id     TEXT,
	raw         TEXT    NOT NULL,
	PRIMARY KEY (source_path, line_number)
)`

type report struct {
	Store       string      `json:"store"`
	Committed   bool        `json:"committed"`
	RowsInStore *int        `json:"rows_in_store,omitempty"`
	Logs        []logReport `json:"logs"`
	Totals      totals      `json:"totals"`
	Warnings    []string    `json:"warnings"`
	Error       string      `json:"error,omitempty"`
	Code        string      `json:"code,omitempty"`
}

// Every line read is counted exactly once: in RowsInserted, RowsSkipped or
// LinesNotCommitted. RowsSkipped is null while the log's stored rows have not
// been read, since which of its lines are already stored is then unknown.
//
// Identity and OriginRepoIDDiffers are null for the user log, which belongs to
// no repository and so has no identity for a line to differ from.
type logReport struct {
	Path                string     `json:"path"`
	Scope               string     `json:"scope"`
	Identity            *string    `json:"identity"`
	LinesRead           int        `json:"lines_read"`
	RowsInserted        int        `json:"rows_inserted"`
	RowsSkipped         *int       `json:"rows_skipped"`
	LinesNotCommitted   int        `json:"lines_not_committed"`
	OriginRepoIDDiffers *int       `json:"origin_repo_id_differs"`
	Conflicts           []conflict `json:"conflicts,omitempty"`
	Error               string     `json:"error,omitempty"`
}

type conflict struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// RowsSkipped is null when any log's is. OriginRepoIDDiffers sums repo logs only.
type totals struct {
	Logs                int  `json:"logs"`
	LinesRead           int  `json:"lines_read"`
	RowsInserted        int  `json:"rows_inserted"`
	RowsSkipped         *int `json:"rows_skipped"`
	LinesNotCommitted   int  `json:"lines_not_committed"`
	OriginRepoIDDiffers int  `json:"origin_repo_id_differs"`
}

type source struct {
	path     string
	scope    memory.Scope
	identity string
	lines    []memory.Line
}

func main() {
	rep := &report{Logs: []logReport{}, Warnings: []string{}}
	err := run(rep, os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(usage)
		return
	}
	if err != nil {
		rep.Code = "error"
		var ce *cliError
		if errors.As(err, &ce) {
			rep.Code = ce.Code
		}
		rep.Error = err.Error()
	}
	rep.total()
	emit(rep)
	if err != nil {
		os.Exit(1)
	}
}

func run(rep *report, args []string) error {
	fs := flag.NewFlagSet("memory-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	store := fs.String("store", defaultStore, "the SQLite store to build or bring up to date")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fail("bad_flag", "%s", err.Error())
	}
	if fs.NArg() > 0 {
		return fail("bad_flag", "unexpected argument %q: the only flag is --store", fs.Arg(0))
	}

	path, err := filepath.Abs(*store)
	if err != nil {
		return fail("bad_store", "%s", err.Error())
	}
	rep.Store = path
	// The driver splits its options off at the first '?' of a plain path.
	if strings.Contains(path, "?") {
		return fail("bad_store", "the store path %s contains '?', which the SQLite driver would read as the start of its options", path)
	}

	found, err := memory.DiscoverLogs(memory.Roots())
	if err != nil {
		if errors.Is(err, memory.ErrRootUnreadable) {
			return fail("bad_root", "%s", err.Error())
		}
		return fail("discover_failed", "%s", err.Error())
	}
	rep.Warnings = append(rep.Warnings, found.Warnings()...)

	sources, err := readAll(rep, found.Logs)
	if err != nil {
		return err
	}
	return load(rep, path, sources)
}

// readAll reads every log before the store is opened, so a log that cannot be
// read fails the run while the store is still untouched.
func readAll(rep *report, locs []memory.Location) ([]source, error) {
	sources := make([]source, 0, len(locs))
	var unreadable []string
	for _, loc := range locs {
		path, err := filepath.Abs(loc.Path)
		if err != nil {
			return nil, fail("read_failed", "%s: %v", loc.Path, err)
		}
		src := source{path: path, scope: loc.Scope}
		entry := logReport{Path: path, Scope: string(loc.Scope)}
		if loc.Scope == memory.ScopeRepo {
			src.identity = gitcmd.RepoIdentity(loc.Repo)
			identity := src.identity
			entry.Identity = &identity
		}

		src.lines, err = readLog(path)
		if err != nil {
			entry.Error = err.Error()
			unreadable = append(unreadable, path)
		}
		entry.LinesRead = len(src.lines)
		entry.LinesNotCommitted = entry.LinesRead
		if entry.Identity != nil {
			differs := otherRepoIDs(src.lines, src.identity)
			entry.OriginRepoIDDiffers = &differs
		}

		rep.Logs = append(rep.Logs, entry)
		sources = append(sources, src)
	}
	if len(unreadable) > 0 {
		return nil, fail("unreadable_log", "could not read %s; nothing was committed", strings.Join(unreadable, ", "))
	}
	return sources, nil
}

// readLog holds a shared lock for as long as Store.Read takes, because
// Store.Append writes under an exclusive one: a line read while it is still being
// written would be stored cut short, the store never updates a row, and every
// later run would refuse that log as rewritten.
func readLog(path string) ([]memory.Line, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := lockShared(f); err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return memory.Open(path).Read()
}

func otherRepoIDs(lines []memory.Line, identity string) int {
	n := 0
	for _, l := range lines {
		if l.Entry == nil || l.Entry.Origin == nil {
			continue
		}
		if id := l.Entry.Origin.RepoID; id != "" && id != identity {
			n++
		}
	}
	return n
}

func load(rep *report, path string, sources []source) error {
	db, err := openStore(path)
	if err != nil {
		return fail("store_failed", "%s", err.Error())
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fail("store_failed", "begin: %v", err)
	}
	defer tx.Rollback()

	pending := make([][]memory.Line, len(sources))
	conflicts := 0
	for i, src := range sources {
		stored, err := storedLines(tx, src.path)
		if err != nil {
			return fail("store_failed", "read rows of %s: %v", src.path, err)
		}
		entry := &rep.Logs[i]
		skipped, differs := 0, 0
		for _, l := range src.lines {
			raw, ok := stored[l.Num]
			if !ok {
				pending[i] = append(pending[i], l)
				continue
			}
			delete(stored, l.Num)
			if raw == l.Raw {
				skipped++
			} else {
				differs++
				entry.Conflicts = append(entry.Conflicts, conflict{l.Num, "the line differs from the row stored for it"})
			}
		}
		entry.RowsSkipped = &skipped
		entry.LinesNotCommitted = len(pending[i]) + differs
		for num := range stored {
			entry.Conflicts = append(entry.Conflicts, conflict{num, "a row is stored for this line and the log no longer has it"})
		}
		slices.SortFunc(entry.Conflicts, func(a, b conflict) int { return a.Line - b.Line })
		conflicts += len(entry.Conflicts)
	}
	if conflicts > 0 {
		return fail("log_rewritten", "%d stored lines no longer match their logs, which an append-only log never does; nothing was committed", conflicts)
	}

	insert, err := tx.Prepare(`INSERT INTO lines (source_path, line_number, scope, repo_id, raw) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fail("store_failed", "prepare insert: %v", err)
	}
	defer insert.Close()
	for i, src := range sources {
		var repoID any
		if src.identity != "" {
			repoID = src.identity
		}
		for _, l := range pending[i] {
			if _, err := insert.Exec(src.path, l.Num, string(src.scope), repoID, l.Raw); err != nil {
				return fail("store_failed", "insert %s:%d: %v", src.path, l.Num, err)
			}
		}
	}

	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM lines`).Scan(&count); err != nil {
		return fail("store_failed", "count rows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		return fail("store_failed", "commit: %v", err)
	}
	rep.Committed = true
	rep.RowsInStore = &count
	for i := range sources {
		rep.Logs[i].RowsInserted = len(pending[i])
		rep.Logs[i].LinesNotCommitted -= len(pending[i])
	}
	return nil
}

func storedLines(tx *sql.Tx, path string) (map[int]string, error) {
	rows, err := tx.Query(`SELECT line_number, raw FROM lines WHERE source_path = ?`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var num int
		var raw string
		if err := rows.Scan(&num, &raw); err != nil {
			return nil, err
		}
		out[num] = raw
	}
	return out, rows.Err()
}

// openStore creates the store so that every member of the directory's group can
// write to it, because the logs it is built from are written by more than one OS
// user.
//
// The file is created here, group-writable, before SQLite ever opens it. Left to
// SQLite it would be created 0644 under the umask, or under the default ACL's mask
// where there is one, and SQLite gives each -journal, -wal and -shm file the mode
// of the database file — so that one mode is the one that has to be right.
//
// The transaction takes the write lock when it begins rather than at its first
// insert, so two runs at once wait for each other instead of both reading the
// same rows and one failing on commit.
func openStore(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return nil, err
	}
	if err := addMode(dir, os.ModeSetgid|0o070); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o664)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err := addMode(path, 0o060); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path+"?_txlock=immediate&_busy_timeout=10000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// addMode changes nothing when the bits are already set, so a store another user
// created and shared correctly does not have to be owned by this one.
func addMode(path string, bits os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if mode&bits == bits {
		return nil
	}
	if err := os.Chmod(path, mode|bits); err != nil {
		return fmt.Errorf("%s must be %v to be shared, and could not be made so: %w", path, mode|bits, err)
	}
	return nil
}

func (r *report) total() {
	skipped := 0
	t := totals{Logs: len(r.Logs), RowsSkipped: &skipped}
	for _, l := range r.Logs {
		t.LinesRead += l.LinesRead
		t.RowsInserted += l.RowsInserted
		t.LinesNotCommitted += l.LinesNotCommitted
		if l.RowsSkipped == nil {
			t.RowsSkipped = nil
		} else {
			skipped += *l.RowsSkipped
		}
		if l.OriginRepoIDDiffers != nil {
			t.OriginRepoIDDiffers += *l.OriginRepoIDDiffers
		}
	}
	r.Totals = t
}

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
