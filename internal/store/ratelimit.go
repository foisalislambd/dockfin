package store

import (
	"context"
	"time"
)

// AllowRateLimit records one hit in the shared sliding window.
// ok is false when the key is already at the limit. retry is how long until
// the oldest hit leaves the window.
func (s *Store) AllowRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	if limit <= 0 || window <= 0 {
		return true, 0, nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	// One lock per key so replicas cannot all admit the same last slot.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return false, 0, err
	}
	cut := time.Now().Add(-window)
	if _, err := tx.Exec(ctx, `DELETE FROM rate_limit_hits WHERE bucket_key=$1 AND seen_at <= $2`, key, cut); err != nil {
		return false, 0, err
	}
	var n int
	var oldest *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*), MIN(seen_at) FROM rate_limit_hits WHERE bucket_key=$1
	`, key).Scan(&n, &oldest); err != nil {
		return false, 0, err
	}
	if n >= limit {
		retry := window
		if oldest != nil {
			retry = window - time.Since(*oldest)
		}
		if retry < time.Second {
			retry = time.Second
		}
		if err := tx.Commit(ctx); err != nil {
			return false, 0, err
		}
		return false, retry, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO rate_limit_hits (bucket_key, seen_at) VALUES ($1, NOW())`, key); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return true, 0, nil
}

// PruneRateLimitHits drops hits older than the longest limiter window.
func (s *Store) PruneRateLimitHits(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM rate_limit_hits WHERE seen_at < NOW() - INTERVAL '1 hour'`)
	return err
}
