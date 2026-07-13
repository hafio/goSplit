package store

import (
	"context"

	"github.com/hafio/gosplit/internal/config"
)

// insertReturningID runs an INSERT with `?` placeholders and returns the newly
// assigned integer primary key. On Postgres it appends `RETURNING id` and scans
// it; on SQLite it uses LastInsertId. `table` is unused but documents intent.
func (s *Store) insertReturningID(ctx context.Context, query, table string, args ...any) (int64, error) {
	_ = table
	if s.Engine == config.EnginePostgres {
		var id int64
		err := s.DB.QueryRowContext(ctx, s.rebind(query+" RETURNING id"), args...).Scan(&id)
		return id, err
	}
	res, err := s.DB.ExecContext(ctx, s.rebind(query), args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
