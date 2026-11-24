package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const SentriiPrefix = "sentrii:"

const SentriiDatabaseEnv = "AI_KIT_SENTRII_DATABASE_URL"

const sentriiTimeout = 5 * time.Second

const sentriiStoredIDsSQL = `SELECT id FROM records WHERE id = ANY($1)`

var sentriiEdgeTypes = map[EdgeType]bool{EdgeRelatesTo: true, EdgeCausedBy: true}

func SentriiID(ref string) (string, bool) {
	return strings.CutPrefix(ref, SentriiPrefix)
}

func SentriiEdgeTypes() []EdgeType { return sortedEdgeTypes(sentriiEdgeTypes) }

type SentriiUnreachable struct {
	Reading string
	Cause   error
}

func (e *SentriiUnreachable) Error() string {
	return fmt.Sprintf("sentrii's memory store could not be read: %s: %v", e.Reading, e.Cause)
}

func (e *SentriiUnreachable) Unwrap() error { return e.Cause }

func SentriiHeld(ctx context.Context, ids []string) (map[string]bool, error) {
	reading := fmt.Sprintf("checking %d cited sentrii id(s)", len(ids))
	dsn := strings.TrimSpace(os.Getenv(SentriiDatabaseEnv))
	if dsn == "" {
		return nil, &SentriiUnreachable{Reading: reading, Cause: fmt.Errorf("%s is not set, so there is no sentrii store to check against", SentriiDatabaseEnv)}
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, &SentriiUnreachable{Reading: reading, Cause: fmt.Errorf("parsing %s: %w", SentriiDatabaseEnv, err)}
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = sentriiTimeout
	}

	queryCtx, cancel := context.WithTimeout(ctx, sentriiTimeout)
	defer cancel()

	conn, err := pgx.ConnectConfig(queryCtx, config)
	if err != nil {
		return nil, sentriiReadError(ctx, queryCtx, reading, err)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(queryCtx, sentriiStoredIDsSQL, ids)
	if err != nil {
		return nil, sentriiReadError(ctx, queryCtx, reading, err)
	}
	defer rows.Close()
	held := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, sentriiReadError(ctx, queryCtx, reading, err)
		}
		held[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, sentriiReadError(ctx, queryCtx, reading, err)
	}
	return held, nil
}

func sentriiReadError(ctx, queryCtx context.Context, reading string, err error) error {
	if ctx.Err() == nil && errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
		return &SentriiUnreachable{Reading: reading, Cause: fmt.Errorf("did not answer in time: took longer than %s: %w", sentriiTimeout, context.DeadlineExceeded)}
	}
	var connecting *pgconn.ConnectError
	if errors.As(err, &connecting) && connecting.Config != nil {
		var answered *pgconn.PgError
		if !errors.As(connecting, &answered) {
			return &SentriiUnreachable{Reading: reading, Cause: fmt.Errorf("did not answer at %s:%d/%s: %v",
				connecting.Config.Host, connecting.Config.Port, connecting.Config.Database, connecting.Unwrap())}
		}
	}
	return &SentriiUnreachable{Reading: reading, Cause: err}
}
