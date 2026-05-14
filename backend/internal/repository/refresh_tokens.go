package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
)

type RefreshTokenRepo struct {
	DB *sql.DB
}

type CreateTokenInput struct {
	ID        string
	UserID    string
	TokenHash []byte
	ParentID  *string
	UserAgent string
	IPAddress string
	ExpiresAt time.Time
}

func (r *RefreshTokenRepo) Create(ctx context.Context, in CreateTokenInput) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, parent_id, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.DB.ExecContext(ctx, q, in.ID, in.UserID, in.TokenHash, in.ParentID, in.UserAgent, in.IPAddress, in.ExpiresAt)
	return err
}

func (r *RefreshTokenRepo) GetByHash(ctx context.Context, hash []byte) (*models.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, parent_id, user_agent, ip_address, created_at, expires_at, revoked_at, replaced_by_id
		FROM refresh_tokens WHERE token_hash = $1
	`
	var t models.RefreshToken
	var parent, replaced sql.NullString
	var revoked sql.NullTime
	err := r.DB.QueryRowContext(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &parent, &t.UserAgent, &t.IPAddress,
		&t.CreatedAt, &t.ExpiresAt, &revoked, &replaced,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parent.Valid {
		s := parent.String
		t.ParentID = &s
	}
	if replaced.Valid {
		s := replaced.String
		t.ReplacedByID = &s
	}
	if revoked.Valid {
		v := revoked.Time
		t.RevokedAt = &v
	}
	return &t, nil
}

// Rotate marks the old token revoked and links it to the new one, then inserts the new row, atomically.
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID string, newToken CreateTokenInput) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, parent_id, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, newToken.ID, newToken.UserID, newToken.TokenHash, newToken.ParentID, newToken.UserAgent, newToken.IPAddress, newToken.ExpiresAt); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW(), replaced_by_id = $1
		WHERE id = $2 AND revoked_at IS NULL
	`, newToken.ID, oldID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("refresh token already rotated or revoked")
	}
	return tx.Commit()
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// RevokeFamily revokes every active descendant rooted at the supplied token id.
// Use on detected reuse: invalidate the whole chain to force re-login.
func (r *RefreshTokenRepo) RevokeFamily(ctx context.Context, userID string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}
