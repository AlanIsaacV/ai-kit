package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type Log struct {
	Path   string
	Scope  Scope
	RepoID string
}

func SourcePaths(store string) ([]Log, error) {
	path, err := (&Store{StorePath: store}).store()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	db, err := openStore(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT source_path, scope, repo_id FROM lines
		WHERE line_number = (SELECT min(line_number) FROM lines AS earliest
			WHERE earliest.source_path = lines.source_path)
		ORDER BY source_path ASC`)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer rows.Close()

	var logs []Log
	for rows.Next() {
		var l Log
		var scope string
		var repoID sql.NullString
		if err := rows.Scan(&l.Path, &scope, &repoID); err != nil {
			return logs, fmt.Errorf("read %s: %w", path, err)
		}
		l.Scope = Scope(scope)
		l.RepoID = repoID.String
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return logs, fmt.Errorf("read %s: %w", path, err)
	}
	return logs, nil
}

func ReadStore(store string) ([]Line, error) {
	_, lines, err := ReadStoreEach(store)
	return lines, err
}

func ReadStoreEach(store string, sources ...string) ([][]Line, []Line, error) {
	held := make([][]Line, len(sources))
	want := make(map[string][]int, len(sources))
	for i, src := range sources {
		abs, err := filepath.Abs(src)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve %s: %w", src, err)
		}
		want[abs] = append(want[abs], i)
	}

	path, err := (&Store{StorePath: store}).store()
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return held, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}

	db, err := openStore(path)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT source_path, line_number, raw FROM lines ORDER BY source_path ASC, line_number ASC`)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer rows.Close()

	var lines []Line
	for rows.Next() {
		var source, raw string
		var num int
		if err := rows.Scan(&source, &num, &raw); err != nil {
			return held, lines, fmt.Errorf("read %s: %w", path, err)
		}
		line := ParseLine(num, raw)
		lines = append(lines, line)
		for _, i := range want[source] {
			held[i] = append(held[i], line)
		}
	}
	if err := rows.Err(); err != nil {
		return held, lines, fmt.Errorf("read %s: %w", path, err)
	}
	return held, lines, nil
}
