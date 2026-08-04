package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UploadRecords accounts for stored images.
//
// The files themselves are content-addressed on disk, which stops the same
// picture being stored twice but does nothing about a thousand different ones.
// This is what makes a quota possible: who uploaded what, and how big it was.
type UploadRecords struct {
	pool *pgxpool.Pool
}

func NewUploadRecords(pool *pgxpool.Pool) *UploadRecords {
	return &UploadRecords{pool: pool}
}

// Usage reports what one account holds and what the deployment holds, in
// plain numbers so the HTTP layer needs no type from this package.
func (r *UploadRecords) Usage(ctx context.Context, ownerUserID string) (ownerBytes, ownerFiles, totalBytes int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(size_bytes) FILTER (WHERE owner_user_id = $1), 0),
			COUNT(*) FILTER (WHERE owner_user_id = $1),
			-- The global figure counts each stored file once, however many
			-- accounts reference it: that is what the disk actually holds.
			COALESCE((SELECT SUM(size) FROM (SELECT DISTINCT ON (name) size_bytes AS size FROM uploads ORDER BY name) AS distinct_files), 0)
		FROM uploads`, ownerUserID).Scan(&ownerBytes, &ownerFiles, &totalBytes)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read upload usage: %w", err)
	}
	return ownerBytes, ownerFiles, totalBytes, nil
}

// Record notes that an account holds an image. Re-uploading the same image is
// not an error and does not charge the account twice.
func (r *UploadRecords) Record(ctx context.Context, name, ownerUserID string, size int64, contentType string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO uploads (name, owner_user_id, size_bytes, content_type)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name, owner_user_id) DO NOTHING`, name, ownerUserID, size, contentType)
	if err != nil {
		return fmt.Errorf("record upload: %w", err)
	}
	return nil
}

// Owns reports whether this account uploaded the image.
func (r *UploadRecords) Owns(ctx context.Context, name, ownerUserID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM uploads WHERE name = $1 AND owner_user_id = $2)`, name, ownerUserID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check upload ownership: %w", err)
	}
	return exists, nil
}

// Release drops this account's claim on an image and reports whether the file
// is now unreferenced and can be deleted from disk.
func (r *UploadRecords) Release(ctx context.Context, name, ownerUserID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin release upload: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `DELETE FROM uploads WHERE name = $1 AND owner_user_id = $2`, name, ownerUserID)
	if err != nil {
		return false, fmt.Errorf("release upload: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	var remaining int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM uploads WHERE name = $1`, name).Scan(&remaining); err != nil {
		return false, fmt.Errorf("count remaining claims: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit release upload: %w", err)
	}
	return remaining == 0, nil
}

// KnownNames lists every image the database knows about, so files on disk that
// nothing references can be cleared away.
func (r *UploadRecords) KnownNames(ctx context.Context) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT name FROM uploads`)
	if err != nil {
		return nil, fmt.Errorf("list uploads: %w", err)
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan upload name: %w", err)
		}
		names[name] = true
	}
	return names, rows.Err()
}
