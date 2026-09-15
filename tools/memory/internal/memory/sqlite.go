package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const DefaultStorePath = "/srv/projects/.ai-kit/memory.db"

var errNoStore = errors.New("no memory store is configured: call memory.UseStore before reading or writing, " +
	"with memory.DefaultStorePath unless the caller was given a path of its own")

var currentStore string

func UseStore(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve store %s: %w", path, err)
	}
	if _, err := storeDSN(abs); err != nil {
		return err
	}
	currentStore = abs
	return nil
}

func CurrentStore() string { return currentStore }

const schema = `CREATE TABLE IF NOT EXISTS lines (
	source_path TEXT    NOT NULL,
	line_number INTEGER NOT NULL,
	scope       TEXT    NOT NULL,
	repo_id     TEXT,
	raw         TEXT    NOT NULL,
	PRIMARY KEY (source_path, line_number)
)`

func storeDSN(path string) (string, error) {
	if strings.Contains(path, "?") {
		return "", fmt.Errorf("the store path %s contains '?', which the SQLite driver would read as the start of its options", path)
	}
	return path + "?_txlock=immediate&_busy_timeout=10000", nil
}

func openStore(path string) (*sql.DB, error) {
	dsn, err := storeDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func createStore(path string) (*sql.DB, error) {
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

	db, err := openStore(path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

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
