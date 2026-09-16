package memory

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store is one log: the rows of the memory store that carry its source path.
//
// The one invariant that matters: no operation in this package ever rewrites,
// reorders or removes a row that is already stored. Corrections are new rows;
// invalidation is a supersedes edge. That is what makes the log trustworthy as
// history, and it is why nothing here issues an UPDATE or a DELETE.
type Store struct {
	Path      string
	StorePath string
	Scope     Scope
	RepoID    string
}

func Open(path string) *Store { return &Store{Path: path} }

func OpenIn(store, path string) *Store { return &Store{Path: path, StorePath: store} }

// gitignoreSentinel is the whole content of the .gitignore written inside the
// .ai-kit directory.
const gitignoreSentinel = "*\n"

// ensureDir creates the .ai-kit directory and, with it, the .gitignore that
// keeps the directory out of git. It reports whether this call was the one that
// wrote the sentinel.
//
// The sentinel goes *inside* the directory rather than as a line appended to the
// repository's own .gitignore: that file belongs to the repository and is not
// this tool's to edit, and a `*` from within covers everything the loop later
// puts here — the drafts under plans/ — without a second line ever being needed.
// It is written with no check for whether there is a git repository above it,
// because outside one it is inert rather than wrong.
func (s *Store) ensureDir() (created bool, err error) {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	name := filepath.Join(dir, ".gitignore")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	if _, err := f.WriteString(gitignoreSentinel); err != nil {
		f.Close()
		// Leave nothing behind. An empty .gitignore ignores nothing, and O_EXCL
		// means no later run would ever get the chance to repair it.
		os.Remove(name)
		return false, err
	}
	return true, f.Close()
}

// Init creates the .ai-kit directory, its .gitignore and the store itself if
// they do not exist. It never touches a row that is already there, and created
// reports whether this call was this repository's first contact.
func (s *Store) Init() (created bool, err error) {
	created, err = s.ensureDir()
	if err != nil {
		return false, err
	}
	store, err := s.store()
	if err != nil {
		return created, err
	}
	db, err := createStore(store)
	if err != nil {
		return created, err
	}
	return created, db.Close()
}

// Append writes each record as one row at the end of the log.
func (s *Store) Append(records ...any) error {
	if len(records) == 0 {
		return nil
	}
	store, err := s.store()
	if err != nil {
		return err
	}
	source, err := s.source()
	if err != nil {
		return err
	}
	if _, err := s.ensureDir(); err != nil {
		return err
	}

	raw := make([]string, 0, len(records))
	for _, r := range records {
		b, err := MarshalLine(r)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		raw = append(raw, string(b))
	}

	db, err := createStore(store)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("append to %s: %w", s.Path, err)
	}
	defer tx.Rollback()

	var next int
	if err := tx.QueryRow(`SELECT coalesce(max(line_number), 0) + 1 FROM lines WHERE source_path = ?`, source).Scan(&next); err != nil {
		return fmt.Errorf("append to %s: %w", s.Path, err)
	}

	insert, err := tx.Prepare(`INSERT INTO lines (source_path, line_number, scope, repo_id, raw) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("append to %s: %w", s.Path, err)
	}
	defer insert.Close()

	scope := s.scope()
	repo := s.repoID(scope)
	for i, line := range raw {
		if _, err := insert.Exec(source, next+i, string(scope), repo, line); err != nil {
			return fmt.Errorf("append to %s:%d: %w", s.Path, next+i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("append to %s: %w", s.Path, err)
	}
	return nil
}

// Read parses the whole log. A line that fails to parse becomes a Line with
// Err set rather than aborting the read: one bad line must not make the lines
// around it unreadable.
func (s *Store) Read() ([]Line, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(store); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", store, err)
	}

	db, err := openStore(store)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT line_number, raw FROM lines WHERE source_path = ? ORDER BY line_number ASC`, source)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Path, err)
	}
	defer rows.Close()

	var lines []Line
	for rows.Next() {
		var num int
		var raw string
		if err := rows.Scan(&num, &raw); err != nil {
			return lines, fmt.Errorf("read %s: %w", s.Path, err)
		}
		lines = append(lines, ParseLine(num, raw))
	}
	if err := rows.Err(); err != nil {
		return lines, fmt.Errorf("read %s: %w", s.Path, err)
	}
	return lines, nil
}

func (s *Store) store() (string, error) {
	path := s.StorePath
	if path == "" {
		path = currentStore
	}
	if path == "" {
		return "", errNoStore
	}
	return filepath.Abs(path)
}

func (s *Store) source() (string, error) {
	abs, err := filepath.Abs(s.Path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", s.Path, err)
	}
	return abs, nil
}

func (s *Store) scope() Scope {
	if s.Scope != "" {
		return s.Scope
	}
	user, err := Resolve(ScopeUser, "")
	if err != nil {
		return ScopeRepo
	}
	abs, err := s.source()
	if err != nil || abs != user.Path {
		return ScopeRepo
	}
	return ScopeUser
}

func (s *Store) repoID(scope Scope) any {
	if scope == ScopeUser || s.RepoID == "" {
		return nil
	}
	return s.RepoID
}

// Entries returns only the well-formed entries, in write order.
func Entries(lines []Line) []*Entry {
	out := make([]*Entry, 0, len(lines))
	for _, l := range lines {
		if l.Entry != nil {
			out = append(out, l.Entry)
		}
	}
	return out
}

// Edges returns only the well-formed edges, in write order.
func Edges(lines []Line) []*Edge {
	out := make([]*Edge, 0, len(lines))
	for _, l := range lines {
		if l.Edge != nil {
			out = append(out, l.Edge)
		}
	}
	return out
}
