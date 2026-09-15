package memory

import (
	"fmt"
	"os"
)

type Log struct {
	Path  string
	Scope Scope
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

	rows, err := db.Query(`SELECT source_path, scope FROM lines
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
		if err := rows.Scan(&l.Path, &scope); err != nil {
			return logs, fmt.Errorf("read %s: %w", path, err)
		}
		l.Scope = Scope(scope)
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return logs, fmt.Errorf("read %s: %w", path, err)
	}
	return logs, nil
}
