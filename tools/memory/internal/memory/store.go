package memory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store is an append-only JSONL log.
//
// The one invariant that matters: no operation in this package ever rewrites,
// reorders or removes a line that is already on disk. Corrections are new
// lines; invalidation is a supersedes edge. That is what makes the log
// trustworthy as history, and it is why nothing here opens a file for
// truncation.
type Store struct {
	Path string
}

func Open(path string) *Store { return &Store{Path: path} }

// Init creates the log and its parent directory if they do not exist. It never
// touches an existing file.
func (s *Store) Init() (created bool, err error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, f.Close()
}

// Append writes one record as a single line at the end of the log.
//
// The write is guarded by an advisory lock because parallel subagents are a
// normal part of this workflow — two of them closing out at the same moment
// must not interleave halves of two records on one line.
func (s *Store) Append(records ...any) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}

	var buf strings.Builder
	for _, r := range records {
		b, err := MarshalLine(r)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}

	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	unlock, err := lockFile(f)
	if err != nil {
		return err
	}
	defer unlock()

	if _, err := f.WriteString(buf.String()); err != nil {
		return err
	}
	return f.Sync()
}

// Read parses the whole log. A line that fails to parse becomes a Line with
// Err set rather than aborting the read: one bad line must not make the lines
// around it unreadable.
func (s *Store) Read() ([]Line, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []Line
	sc := bufio.NewScanner(f)
	// Bodies can be long; the default 64 KiB token limit is not enough.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	num := 0
	for sc.Scan() {
		num++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		lines = append(lines, ParseLine(num, raw))
	}
	if err := sc.Err(); err != nil {
		return lines, fmt.Errorf("read %s: %w", s.Path, err)
	}
	return lines, nil
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
