package memory

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReadKeepsTheLineNumbersTheJSONLLogGaveItsLines(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "repo", ".ai-kit", "memory.jsonl")
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		t.Fatal(err)
	}

	raw := []string{
		marshalLine(t, newTestEntry(t, func(e *Entry) { e.Title = "first" })),
		marshalLine(t, newTestEntry(t, func(e *Entry) { e.Title = "second" })),
		marshalLine(t, newTestEntry(t, func(e *Entry) { e.Title = "third" })),
	}
	content := raw[0] + "\n\n" + raw[1] + "\n\n\n" + raw[2] + "\n"
	if err := os.WriteFile(log, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	want := readJSONL(t, log)
	if len(want) != 3 || want[0].Num != 1 || want[1].Num != 3 || want[2].Num != 6 {
		t.Fatalf("the JSONL read this is measured against numbers physical lines: %v", nums(want))
	}

	store := filepath.Join(dir, "memory.db")
	seedStore(t, store, log, want)

	got, err := OpenIn(store, log).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d lines from the store, %d from the log: %v vs %v", len(got), len(want), nums(got), nums(want))
	}
	for i := range want {
		if got[i].Num != want[i].Num {
			t.Errorf("line %d is numbered %d in the store and %d in the log", i, got[i].Num, want[i].Num)
		}
		if got[i].Raw != want[i].Raw {
			t.Errorf("line %d differs:\n store %s\n log   %s", i, got[i].Raw, want[i].Raw)
		}
		if (got[i].Entry == nil) != (want[i].Entry == nil) {
			t.Fatalf("line %d parsed one way and not the other", i)
		}
		if got[i].Entry != nil && got[i].Entry.Title != want[i].Entry.Title {
			t.Errorf("line %d is %q in the store and %q in the log", i, got[i].Entry.Title, want[i].Entry.Title)
		}
	}
}

func TestConcurrentAppendsBothLandWithDistinctLineNumbers(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "memory.db")
	log := filepath.Join(dir, "repo", ".ai-kit", "memory.jsonl")

	first := newTestEntry(t, func(e *Entry) { e.Title = "first writer" })
	second := newTestEntry(t, func(e *Entry) { e.Title = "second writer" })

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, e := range []*Entry{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = OpenIn(store, log).Append(e)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d lost its append: %v", i, err)
		}
	}

	lines, err := OpenIn(store, log).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected both records in the log, got %d: %v", len(lines), nums(lines))
	}
	if lines[0].Num == lines[1].Num {
		t.Fatalf("both records were written as line %d", lines[0].Num)
	}
	ids := map[string]bool{}
	for _, l := range lines {
		if l.Entry == nil {
			t.Fatalf("line %d is not an entry: %v", l.Num, l.Err)
		}
		ids[l.Entry.ID] = true
	}
	if !ids[first.ID] || !ids[second.ID] {
		t.Fatalf("one of the two writes was lost: %v", ids)
	}
}

func marshalLine(t *testing.T, v any) string {
	t.Helper()
	b, err := MarshalLine(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readJSONL(t *testing.T, path string) []Line {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var lines []Line
	sc := bufio.NewScanner(f)
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
		t.Fatal(err)
	}
	return lines
}

func seedStore(t *testing.T, store, source string, lines []Line) {
	t.Helper()
	db, err := createStore(store)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, l := range lines {
		if _, err := db.Exec(
			`INSERT INTO lines (source_path, line_number, scope, repo_id, raw) VALUES (?, ?, ?, ?, ?)`,
			source, l.Num, string(ScopeRepo), nil, l.Raw); err != nil {
			t.Fatal(err)
		}
	}
}

func nums(lines []Line) []int {
	out := make([]int, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Num)
	}
	return out
}
