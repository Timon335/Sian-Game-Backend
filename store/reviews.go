package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrDuplicateReview หมายถึงสมาชิกคนนี้รีวิวเกมนี้แล้ว
	ErrDuplicateReview = errors.New("duplicate review")
	// ErrNotOwner หมายถึงรีวิวนี้ไม่ใช่ของสมาชิกคนนี้
	ErrNotOwner = errors.New("not owner")
)

// Review คือรีวิวหนึ่งรายการ GameTitle และ Email ใช้เฉพาะหน้าหลังบ้าน
type Review struct {
	ID          int64     `json:"id"`
	MemberID    int64     `json:"memberId"`
	DisplayName string    `json:"displayName"`
	GameID      int64     `json:"gameId"`
	Rating      int       `json:"rating"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	GameTitle   string    `json:"gameTitle,omitempty"`
	Email       string    `json:"email,omitempty"`
}

const reviewSelect = `SELECT r.id, r.member_id, m.display_name, r.game_id, r.rating, r.text, r.created_at, r.updated_at
FROM reviews r JOIN members m ON m.id = r.member_id`

func scanReview(row pgx.Row) (Review, error) {
	var r Review
	err := row.Scan(&r.ID, &r.MemberID, &r.DisplayName, &r.GameID, &r.Rating, &r.Text, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// ListReviews คืนรีวิวของเกม ใหม่สุดก่อน
func (s *Store) ListReviews(ctx context.Context, gameID int64) ([]Review, error) {
	rows, err := s.Pool.Query(ctx, reviewSelect+` WHERE r.game_id = $1 ORDER BY r.created_at DESC, r.id DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) reviewByID(ctx context.Context, id int64) (Review, error) {
	return scanReview(s.Pool.QueryRow(ctx, reviewSelect+` WHERE r.id = $1`, id))
}

// CreateReview เพิ่มรีวิว ไม่ต้องซื้อเกมก่อน แต่รีวิวได้เกมละครั้ง
func (s *Store) CreateReview(ctx context.Context, memberID, gameID int64, rating int, text string) (Review, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO reviews (member_id, game_id, rating, text) VALUES ($1,$2,$3,$4) RETURNING id`,
		memberID, gameID, rating, text).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return Review{}, ErrDuplicateReview
		case "23503":
			return Review{}, ErrNotFound
		}
	}
	if err != nil {
		return Review{}, err
	}
	return s.reviewByID(ctx, id)
}

// checkOwner ตรวจว่ารีวิวเป็นของสมาชิกคนนี้
func (s *Store) checkOwner(ctx context.Context, id, memberID int64) error {
	var owner int64
	err := s.Pool.QueryRow(ctx, `SELECT member_id FROM reviews WHERE id = $1`, id).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != memberID {
		return ErrNotOwner
	}
	return nil
}

// UpdateReview แก้รีวิวของตัวเองเท่านั้น
func (s *Store) UpdateReview(ctx context.Context, id, memberID int64, rating int, text string) (Review, error) {
	if err := s.checkOwner(ctx, id, memberID); err != nil {
		return Review{}, err
	}
	_, err := s.Pool.Exec(ctx,
		`UPDATE reviews SET rating = $3, text = $4, updated_at = now() WHERE id = $1 AND member_id = $2`,
		id, memberID, rating, text)
	if err != nil {
		return Review{}, err
	}
	return s.reviewByID(ctx, id)
}

// DeleteReview ลบรีวิวของตัวเองเท่านั้น
func (s *Store) DeleteReview(ctx context.Context, id, memberID int64) error {
	if err := s.checkOwner(ctx, id, memberID); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM reviews WHERE id = $1 AND member_id = $2`, id, memberID)
	return err
}
